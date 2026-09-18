// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestGitHubCIListsEachCheckWithItsPage(t *testing.T) {
	t.Parallel()

	// Arrange
	statuses := `{"total_count":1,"statuses":[{"state":"failure","context":"lint","target_url":"https://ci/lint"}]}`
	runs := `{"total_count":1,"check_runs":[{"status":"completed","conclusion":"success",` +
		`"name":"build","html_url":"https://ci/build"}]}`

	client := githubChecks(t, statuses, runs)

	// Act
	got, err := client.CheckStatus(t.Context(), githubRepo(), forge.PullRequest{}, headCommit)

	// Assert
	want := []forge.Check{
		{Name: "lint", State: forge.CIFailed, URL: "https://ci/lint"},
		{Name: "build", State: forge.CIPassed, URL: "https://ci/build"},
	}
	if err != nil || !slices.Equal(got.Checks, want) {
		t.Errorf("checks = %+v, %v; want %+v", got.Checks, err, want)
	}
}

func TestGitLabCIListsThePipelineAsOneCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := forgeAnswering(t, http.StatusOK,
		`{"iid":8,"head_pipeline":{"status":"failed","web_url":"https://gl/pipelines/9"}}`)

	// Act
	got, err := client.CheckStatus(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, headCommit)

	// Assert
	want := []forge.Check{{Name: "pipeline", State: forge.CIFailed, URL: "https://gl/pipelines/9"}}
	if err != nil || !slices.Equal(got.Checks, want) {
		t.Errorf("checks = %+v, %v; want %+v", got.Checks, err, want)
	}
}
