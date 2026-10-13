// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// A merge request into the base from a fork's branch of the same name is
// someone else's: GitLab marks it by a source project other than its target.
func TestGitLabFindPassesOverAForksMergeRequestOfTheSameName(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		gitlabMergesPath: `[{"iid":3,"state":"opened","source_project_id":42,"target_project_id":7}]`,
	}))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), gitlabRepo(), forge.Head{Branch: featureBranch})

	// Assert
	if err != nil || found {
		t.Errorf("FindPullRequest = %+v, %v, %v; want none found", pull, found, err)
	}
}

func TestGitLabFindTakesTheBasesOwnMergeRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := recordingForge(t, routing(map[string]string{
		gitlabMergesPath: `[{"iid":3,"state":"opened","source_project_id":42,"target_project_id":7},` +
			`{"iid":5,"state":"opened","source_project_id":7,"target_project_id":7}]`,
	}))

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), gitlabRepo(), forge.Head{Branch: featureBranch})

	// Assert
	if err != nil || !found || pull.Number != 5 {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the base's own !5", pull, found, err)
	}
}
