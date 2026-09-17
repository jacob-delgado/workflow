// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// headCommit is the commit whose checks the GitHub cases read.
const headCommit = "abc123"

// githubChecks serves a commit's combined status and its check runs, and fails
// the test on any other request.
func githubChecks(t *testing.T, statuses, runs string) forge.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch request.URL.EscapedPath() {
		case "/repos/example/repo/commits/abc123/status":
			_, _ = writer.Write([]byte(statuses))
		case "/repos/example/repo/commits/abc123/check-runs":
			if !strings.Contains(request.URL.RawQuery, "per_page=100") {
				t.Errorf("check runs asked for %q, want a full page", request.URL.RawQuery)
			}

			_, _ = writer.Write([]byte(runs))
		default:
			t.Errorf("unexpected request for %s", request.URL.EscapedPath())
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret)
}

func TestGitHubCIReadsStatusesAndCheckRunsTogether(t *testing.T) {
	t.Parallel()

	const (
		noStatuses = `{"state":"pending","total_count":0,"statuses":[]}`
		noRuns     = `{"total_count":0,"check_runs":[]}`
	)

	cases := map[string]struct {
		statuses, runs string
		want           forge.CI
	}{
		// Checked on this repository: a commit with only check runs, or none
		// at all, has a combined status of "pending" with nothing in it.
		// Believing it would wait forever for CI that is never coming.
		"nothing configured": {
			statuses: noStatuses, runs: noRuns,
			want: forge.CI{State: forge.CINone, Total: 0, Done: 0, Failed: 0},
		},
		"all passed": {
			statuses: `{"state":"success","total_count":1,"statuses":[{"state":"success"}]}`,
			runs:     `{"total_count":1,"check_runs":[{"status":"completed","conclusion":"success"}]}`,
			want:     forge.CI{State: forge.CIPassed, Total: 2, Done: 2, Failed: 0},
		},
		"skipped and neutral count as passed": {
			statuses: noStatuses,
			runs: `{"total_count":2,"check_runs":[{"status":"completed","conclusion":"skipped"},` +
				`{"status":"completed","conclusion":"neutral"}]}`,
			want: forge.CI{State: forge.CIPassed, Total: 2, Done: 2, Failed: 0},
		},
		"a run still going": {
			statuses: noStatuses,
			runs: `{"total_count":2,"check_runs":[{"status":"in_progress","conclusion":null},` +
				`{"status":"completed","conclusion":"success"}]}`,
			want: forge.CI{State: forge.CIRunning, Total: 2, Done: 1, Failed: 0},
		},
		"a status still pending": {
			statuses: `{"state":"pending","total_count":1,"statuses":[{"state":"pending"}]}`,
			runs:     noRuns,
			want:     forge.CI{State: forge.CIRunning, Total: 1, Done: 0, Failed: 0},
		},
		"a failure wins over what is still running": {
			statuses: noStatuses,
			runs: `{"total_count":2,"check_runs":[{"status":"completed","conclusion":"failure"},` +
				`{"status":"queued","conclusion":null}]}`,
			want: forge.CI{State: forge.CIFailed, Total: 2, Done: 1, Failed: 1},
		},
		"canceled and timed out fail": {
			statuses: noStatuses,
			runs: `{"total_count":2,"check_runs":[{"status":"completed","conclusion":"canceled"},` +
				`{"status":"completed","conclusion":"timed_out"}]}`,
			want: forge.CI{State: forge.CIFailed, Total: 2, Done: 2, Failed: 2},
		},
		"a failed status fails": {
			statuses: `{"state":"failure","total_count":1,"statuses":[{"state":"failure"}]}`,
			runs:     noRuns,
			want:     forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1},
		},
		"an errored status fails": {
			statuses: `{"state":"error","total_count":1,"statuses":[{"state":"error"}]}`,
			runs:     noRuns,
			want:     forge.CI{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := githubChecks(t, tt.statuses, tt.runs)

			// Act
			got, err := client.CheckStatus(t.Context(), githubRepo(), forge.PullRequest{}, headCommit)

			// Assert
			if err != nil || got != tt.want {
				t.Errorf("CheckStatus = %+v, %v, want %+v", got, err, tt.want)
			}
		})
	}
}

func TestCIReportsARefusalToSay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		status int
		body   string
		want   error
	}{
		"github refusing": {repo: githubRepo(), status: http.StatusForbidden, body: `{}`, want: forge.ErrRefused},
		"gitlab not taking the token": {
			repo: gitlabRepo(), status: http.StatusUnauthorized, body: `{"message":"401 Unauthorized"}`,
			want: forge.ErrUnauthorized,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, tt.status, tt.body)

			// Act
			_, err := client.CheckStatus(t.Context(), tt.repo, forge.PullRequest{Number: 8}, headCommit)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("CheckStatus returned %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGitLabCIReadsTheMergeRequestsPipeline(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pipeline string
		want     forge.CIState
	}{
		"no pipeline":  {pipeline: `null`, want: forge.CINone},
		"running":      {pipeline: `{"status":"running"}`, want: forge.CIRunning},
		"pending":      {pipeline: `{"status":"pending"}`, want: forge.CIRunning},
		"created":      {pipeline: `{"status":"created"}`, want: forge.CIRunning},
		"waiting":      {pipeline: `{"status":"waiting_for_resource"}`, want: forge.CIRunning},
		"manual":       {pipeline: `{"status":"manual"}`, want: forge.CIRunning},
		"success":      {pipeline: `{"status":"success"}`, want: forge.CIPassed},
		"skipped":      {pipeline: `{"status":"skipped"}`, want: forge.CIPassed},
		"failed":       {pipeline: `{"status":"failed"}`, want: forge.CIFailed},
		"canceled":     {pipeline: `{"status":"canceled"}`, want: forge.CIFailed},
		"unrecognized": {pipeline: `{"status":"something-new"}`, want: forge.CIRunning},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeAnswering(t, http.StatusOK, `{"iid":8,"head_pipeline":`+tt.pipeline+`}`)

			// Act
			got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

			// Assert
			if err != nil || got.State != tt.want {
				t.Errorf("CheckStatus = %+v, %v, want state %d", got, err, tt.want)
			}

			// The merge request's own pipeline, not one found by commit: a
			// commit can have pipelines that have nothing to do with the review.
			if asked := lastRequest(t, seen); asked.path != "/projects/group%2Fsub%2Frepo/merge_requests/8" {
				t.Errorf("asked %s, want the merge request itself", asked.path)
			}
		})
	}
}

func TestGitHubCIReportsCheckRunsThatCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/check-runs") {
			writer.WriteHeader(http.StatusInternalServerError)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"state":"pending","total_count":0,"statuses":[]}`))
	}))
	t.Cleanup(server.Close)

	client := forge.New(server.Client().Do, server.URL, secret)

	// Act
	_, err := client.CheckStatus(t.Context(), githubRepo(), forge.PullRequest{}, headCommit)

	// Assert
	if !errors.Is(err, forge.ErrUnexpectedStatus) {
		t.Errorf("CheckStatus returned %v, want the check runs' unexpected status", err)
	}
}
