// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The GitLab paths a failed merge request's CI is read from.
const (
	failedMergePath = "/projects/group%2Fsub%2Frepo/merge_requests/8"
	failedJobsPath  = "/projects/group%2Fsub%2Frepo/pipelines/77/jobs"
)

// canceled is what GitLab calls a job stopped before it finished, as a scope
// to ask for and as the reason it did not pass.
const canceled = "canceled"

func TestGitLabCIListsAFailedPipelinesFailedJobsWithWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{
		failedMergePath: `{"iid":8,"head_pipeline":{"id":77,"status":"failed","web_url":"https://gl/pipelines/77"}}`,
		failedJobsPath: `[{"id":501,"name":"unit-race","stage":"test","failure_reason":"script_failure",` +
			`"web_url":"https://gl/jobs/501"}]`,
	}))

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	want := []forge.Check{{
		ID: "501", Name: "unit-race", Stage: "test", Reason: "script failure", State: forge.CIFailed,
		URL: "https://gl/jobs/501", LogAvailable: true,
	}}
	if err != nil || got.State != forge.CIFailed || !slices.Equal(got.Checks, want) {
		t.Errorf("CheckStatus = %+v, %v; want the failed job, its stage and why", got, err)
	}

	query, _ := url.ParseQuery(requestTo(*seen, failedJobsPath).query)
	if !slices.Equal(query["scope[]"], []string{"failed", canceled}) {
		t.Errorf("jobs asked with %q, want only the failed and canceled ones", requestTo(*seen, failedJobsPath).query)
	}
}

func TestGitLabCIKeepsThePipelineWhenItsJobsCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		failedMergePath: `{"iid":8,"head_pipeline":{"id":77,"status":"failed","web_url":"https://gl/pipelines/77"}}`,
		failedJobsPath:  notJSON,
	}))

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	want := []forge.Check{{Name: "pipeline", State: forge.CIFailed, URL: "https://gl/pipelines/77"}}
	if err != nil || !slices.Equal(got.Checks, want) {
		t.Errorf("CheckStatus = %+v, %v; want the pipeline as the one check", got, err)
	}
}

func TestGitHubCIKeepsEachCheckRunsIDAndWhyItFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	runs := `{"total_count":1,"check_runs":[{"id":901,"status":"completed","conclusion":"failure",` +
		`"name":"unit-race","html_url":"https://ci/901","output":{"title":"3 tests failed","summary":"long…"},` +
		`"app":{"slug":"github-actions"}}]}`
	client := githubChecks(t, `{"total_count":0,"statuses":[]}`, runs)

	// Act
	got, err := client.CheckStatus(t.Context(), githubRepo(), forge.PullRequest{}, headCommit)

	// Assert
	want := []forge.Check{{
		ID: "901", Name: "unit-race", Reason: "3 tests failed", State: forge.CIFailed, URL: "https://ci/901",
		LogAvailable: true,
	}}
	if err != nil || !slices.Equal(got.Checks, want) {
		t.Errorf("checks = %+v, %v; want %+v", got.Checks, err, want)
	}
}

func TestGitLabCIListsACanceledPipelinesCanceledJobs(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, func(asked recorded) (int, string) {
		query, _ := url.ParseQuery(asked.query)

		switch {
		case asked.path == failedMergePath:
			return http.StatusOK, `{"iid":8,"head_pipeline":{"id":77,"status":"canceled","web_url":"https://gl/pipelines/77"}}`
		case asked.path == failedJobsPath && slices.Contains(query["scope[]"], canceled):
			return http.StatusOK, `[{"id":502,"name":"e2e","stage":"test","status":"canceled","web_url":"https://gl/jobs/502"}]`
		default:
			return http.StatusOK, "[]"
		}
	})

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	want := []forge.Check{{
		ID: "502", Name: "e2e", Stage: "test", Reason: canceled, State: forge.CIFailed,
		URL: "https://gl/jobs/502", LogAvailable: true,
	}}
	if err != nil || got.State != forge.CIFailed || !slices.Equal(got.Checks, want) {
		t.Errorf("CheckStatus = %+v, %v; want the canceled job, its stage, and that it was canceled", got, err)
	}
}
