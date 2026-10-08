// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
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

	if query, _ := url.ParseQuery(requestTo(*seen, failedJobsPath).query); query.Get("scope[]") != "failed" {
		t.Errorf("jobs asked with %q, want only the failed ones", requestTo(*seen, failedJobsPath).query)
	}
}

func TestGitLabCIKeepsThePipelineWhenItsJobsCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		failedMergePath: `{"iid":8,"head_pipeline":{"id":77,"status":"failed","web_url":"https://gl/pipelines/77"}}`,
		failedJobsPath:  `not json`,
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
