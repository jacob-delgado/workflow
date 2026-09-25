// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestRerunChecksReRunsGitHubsFailedRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		runsPath  = "/repos/example/repo/actions/runs"
		failed    = runsPath + "/11/rerun-failed-jobs"
		succeeded = runsPath + "/22/rerun-failed-jobs"
	)

	client, seen := forgeRouting(t, map[string]string{
		runsPath: `{"workflow_runs":[{"id":11,"conclusion":"failure"},{"id":22,"conclusion":"success"}]}`,
		failed:   `{}`,
	})

	// Act
	reran, err := client.RerunChecks(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, "abc123")

	// Assert
	// The failed run is re-run and the passed one is left alone.
	if err != nil || !reran {
		t.Fatalf("RerunChecks = %v, %v, want a re-run", reran, err)
	}

	if got := requestTo(*seen, runsPath); got.query != "head_sha=abc123&page=1&per_page=100" {
		t.Errorf("runs asked with query %q, want the head sha and a full first page", got.query)
	}

	if got := requestTo(*seen, failed); got.method != http.MethodPost {
		t.Errorf("re-ran the failed run with %q, want a POST", got.method)
	}

	if got := requestTo(*seen, succeeded); got.method != "" {
		t.Errorf("re-ran the passed run with %q, want it left alone", got.method)
	}
}

func TestRerunChecksReRunsAFailedRunPastTheFirstPage(t *testing.T) {
	t.Parallel()

	// Arrange
	// A hundred runs passed; the one that failed is the hundred-and-first, on
	// the second page. Its re-run is the only one served, so any other would
	// fail the test.
	const (
		runsPath = "/repos/example/repo/actions/runs"
		lastRun  = runsPath + "/101/rerun-failed-jobs"
	)

	passed := func(number int) string { return `{"id":` + strconv.Itoa(number) + `,"conclusion":"success"}` }

	client := forgePaging(t, map[string][]string{
		runsPath: {
			`{"total_count":101,"workflow_runs":` + listingOf(1, 100, passed) + `}`,
			`{"total_count":101,"workflow_runs":[{"id":101,"conclusion":"failure"}]}`,
		},
		lastRun: {`{}`},
	})

	// Act
	reran, err := client.RerunChecks(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, "abc123")

	// Assert
	if err != nil || !reran {
		t.Errorf("RerunChecks = %v, %v; want the failed run on the second page re-run", reran, err)
	}
}

func TestRerunChecksRetriesGitLabsPipeline(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		mergePath = "/projects/group%2Fsub%2Frepo/merge_requests/8"
		retryPath = "/projects/group%2Fsub%2Frepo/pipelines/99/retry"
	)

	client, seen := forgeRouting(t, map[string]string{
		mergePath: `{"iid":8,"head_pipeline":{"id":99,"status":"failed"}}`,
		retryPath: `{"id":99,"status":"running"}`,
	})

	// Act
	reran, err := client.RerunChecks(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, "")

	// Assert
	if err != nil || !reran {
		t.Fatalf("RerunChecks = %v, %v, want a retry", reran, err)
	}

	if got := requestTo(*seen, retryPath); got.method != http.MethodPost {
		t.Errorf("retried the pipeline with %q, want a POST", got.method)
	}
}

func TestRerunChecksReportsARefusedRerun(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token can read the runs but not re-run them, the way an under-scoped
	// credential answers.
	client := serveForge(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if request.Method == http.MethodPost {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`{"message":"Resource not accessible by personal access token"}`))

			return
		}

		_, _ = writer.Write([]byte(`{"workflow_runs":[{"id":11,"conclusion":"failure"}]}`))
	})

	// Act
	_, err := client.RerunChecks(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, "abc123")

	// Assert
	if !errors.Is(err, forge.ErrRefused) || !strings.Contains(err.Error(), "Resource not accessible") {
		t.Errorf("RerunChecks returned %v, want ErrRefused with the forge's reason", err)
	}
}

func TestRerunChecksReRunsATimedOutRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// A timed-out run is a failure the Review pane shows, so it must be re-run
	// like a plain "failure" one.
	const (
		runsPath = "/repos/example/repo/actions/runs"
		timedOut = runsPath + "/33/rerun-failed-jobs"
	)

	client, seen := forgeRouting(t, map[string]string{
		runsPath: `{"workflow_runs":[{"id":33,"conclusion":"timed_out"}]}`,
		timedOut: `{}`,
	})

	// Act
	reran, err := client.RerunChecks(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, "abc123")

	// Assert
	if err != nil || !reran {
		t.Fatalf("RerunChecks = %v, %v, want the timed-out run re-run", reran, err)
	}

	if got := requestTo(*seen, timedOut); got.method != http.MethodPost {
		t.Errorf("re-ran the timed-out run with %q, want a POST", got.method)
	}
}

func TestRerunChecksReRunsNothingWhenNoRunFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failure is a status reported outside Actions, so there is no run to
	// re-run.
	const runsPath = "/repos/example/repo/actions/runs"

	client, _ := forgeRouting(t, map[string]string{
		runsPath: `{"workflow_runs":[{"id":44,"conclusion":"success"}]}`,
	})

	// Act
	reran, err := client.RerunChecks(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, "abc123")

	// Assert
	if err != nil || reran {
		t.Errorf("RerunChecks = %v, %v, want nothing re-run", reran, err)
	}
}
