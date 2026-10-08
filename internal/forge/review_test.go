// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestFindPullRequestReadsReviewState(t *testing.T) {
	t.Parallel()

	// Arrange
	// ana approved then had it dismissed, so it no longer counts; ben requests
	// changes; cass approves; a comment is not a stance.
	routes := map[string]string{
		githubPullsPath:        `[{"number":9,"html_url":"https://x/9","title":"fix: token","draft":false}]`,
		githubPullsPath + "/9": `{"mergeable":false}`,
		githubPullsPath + "/9/reviews": `[` +
			`{"state":"APPROVED","user":{"login":"ana"}},` +
			`{"state":"DISMISSED","user":{"login":"ana"}},` +
			`{"state":"CHANGES_REQUESTED","user":{"login":"ben"}},` +
			`{"state":"COMMENTED","user":{"login":"dan"}},` +
			`{"state":"APPROVED","user":{"login":"cass"}}]`,
	}

	client, _ := recordingForge(t, routing(routes))

	// Act
	found, _, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || found.Approvals != 1 || !found.ChangesRequested || found.Mergeable != forge.MergeConflicts {
		t.Errorf("review state = %+v, %v; want 1 approval, changes requested, conflicts", found, err)
	}
}

func TestFindPullRequestToleratesUnreadableReviewState(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only the list answers usefully; the detail and reviews reads get nothing
	// that parses. A pull request found is better than none, so the review fields
	// stay at their zero rather than failing the whole find.
	routes := map[string]string{
		githubPullsPath: `[{"number":3,"html_url":"https://x/3","title":"fix: token","draft":false}]`,
	}

	client, _ := recordingForge(t, routing(routes))

	// Act
	found, ok, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || !ok || found.Approvals != 0 || found.ChangesRequested || found.Mergeable != forge.MergeUnknown {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the pull found with its review state unknown", found, ok, err)
	}
}
