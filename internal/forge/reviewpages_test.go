// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"strconv"
	"testing"
)

// reviewBy is one review on a GitHub pull request: a stance, or a comment,
// left by login.
func reviewBy(state, login string) string {
	return `{"state":"` + state + `","user":{"login":"` + login + `"}}`
}

// commentBy is a comment by the numbered reviewer, which takes no stance.
func commentBy(number int) string {
	return reviewBy("COMMENTED", "u"+strconv.Itoa(number))
}

// commentsThenApproval is a full page whose last review is ana's approval.
func commentsThenApproval(number int) string {
	if number == 100 {
		return reviewBy("APPROVED", "ana")
	}

	return commentBy(number)
}

func TestFindPullRequestReadsEveryPageOfReviews(t *testing.T) {
	t.Parallel()

	// GitHub lists reviews oldest first, so a reviewer's latest stance is the
	// one most likely to be past the first page.
	cases := map[string]struct {
		pages     []string
		approvals int
	}{
		"an approval past the first page": {
			pages:     []string{listingOf(1, 100, commentBy), "[" + reviewBy("APPROVED", "ana") + "]"},
			approvals: 1,
		},
		"a dismissal past the first page": {
			pages:     []string{listingOf(1, 100, commentsThenApproval), "[" + reviewBy("DISMISSED", "ana") + "]"},
			approvals: 0,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forgePaging(t, map[string][]string{
				githubPullsPath:                {`[{"number":9,"html_url":"https://x/9","title":"fix: token","draft":false}]`},
				githubPullsPath + "/9":         {`{"mergeable":true}`},
				githubPullsPath + "/9/reviews": tt.pages,
			})

			// Act
			found, _, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

			// Assert
			if err != nil || found.Approvals != tt.approvals {
				t.Errorf("FindPullRequest = %+v, %v; want %d approvals", found, err, tt.approvals)
			}
		})
	}
}
