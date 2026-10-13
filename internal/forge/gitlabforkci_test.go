// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// A fork's merge request runs its pipeline in the fork, project 42, while it
// merges into project 7.
const (
	forkMergePath    = "/projects/group%2Fsub%2Frepo/merge_requests/8"
	forkPipelineJobs = "/projects/42/pipelines/77/jobs"
	forkMerge        = `{"iid":8,"target_project_id":7,` +
		`"head_pipeline":{"id":77,"status":"failed","project_id":42,"web_url":"https://gl/pipelines/77"}}`
)

func TestAForkPipelinesFailedJobsAreReadInTheFork(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		forkMergePath:    forkMerge,
		forkPipelineJobs: `[{"id":501,"name":"unit","stage":"test","web_url":"https://gl/jobs/501"}]`,
	}))

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	if err != nil || len(got.Checks) != 1 || got.Checks[0].ID != "501" || got.Checks[0].Project != "42" {
		t.Errorf("CheckStatus = %+v, %v; want job 501 read in project 42", got, err)
	}
}

func TestAForkJobsLogIsReadInTheFork(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{"/projects/42/jobs/501/trace": "boom\n"}))

	// Act
	log, err := client.JobLog(t.Context(), gitlabRepo(), forge.Check{ID: "501", LogAvailable: true, Project: "42"})

	// Assert
	if err != nil || log.Text != "boom" || len(*seen) != 1 {
		t.Errorf("JobLog = %+v, %v after %v; want the fork's trace", log, err, *seen)
	}
}

func TestAForkPipelineIsRetriedInTheFork(t *testing.T) {
	t.Parallel()

	// Arrange
	const retryPath = "/projects/42/pipelines/77/retry"

	client, seen := recordingForge(t, routing(map[string]string{forkMergePath: forkMerge, retryPath: `{"id":77}`}))

	// Act
	reran, err := client.RerunChecks(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, "")

	// Assert
	if err != nil || !reran || requestTo(*seen, retryPath).method != http.MethodPost {
		t.Errorf("RerunChecks = %v, %v after %v; want the fork's pipeline retried", reran, err, *seen)
	}
}

// A GitLab that names no pipeline project is read as the merge request's own.
func TestAPipelineNamingNoProjectIsReadInTheMergeRequestsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		forkMergePath: `{"iid":8,"target_project_id":7,"head_pipeline":{"id":77,"status":"failed"}}`,
		"/projects/group%2Fsub%2Frepo/pipelines/77/jobs": `[{"id":501,"name":"unit","web_url":"https://gl/jobs/501"}]`,
	}))

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	if err != nil || len(got.Checks) != 1 || got.Checks[0].ID != "501" || got.Checks[0].Project != "" {
		t.Errorf("CheckStatus = %+v, %v; want job 501 read in the merge request's project", got, err)
	}
}
