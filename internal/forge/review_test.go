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

	// A pull request found is better than none, so a read of its detail or its
	// reviews that gets nothing that parses leaves what it would have filled in
	// at its zero rather than failing the whole find.
	list := `[{"number":3,"html_url":"https://x/3","title":"fix: token","draft":false}]`
	unreadable := `{"message":"not a list"}`

	cases := map[string]struct {
		routes    map[string]string
		mergeable forge.Mergeability
	}{
		"neither the detail nor the reviews": {
			routes:    map[string]string{githubPullsPath: list, githubPullsPath + "/3/reviews": unreadable},
			mergeable: forge.MergeUnknown,
		},
		"the reviews alone": {
			routes: map[string]string{
				githubPullsPath: list, githubPullsPath + "/3": `{"mergeable":false}`,
				githubPullsPath + "/3/reviews": unreadable,
			},
			mergeable: forge.MergeConflicts,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, routing(tt.routes))

			// Act
			found, ok, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

			// Assert
			if err != nil || !ok || found.Approvals != 0 || found.ChangesRequested || found.Mergeable != tt.mergeable {
				t.Errorf("FindPullRequest = %+v, %v, %v; want the pull found, mergeable %v, its reviews unknown",
					found, ok, err, tt.mergeable)
			}
		})
	}
}
