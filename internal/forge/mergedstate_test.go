// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestFindPullRequestReportsAMergedBranch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo     forge.Repo
		findPath string
		body     string
	}{
		"github": {
			repo: githubRepo(), findPath: githubPullsPath,
			body: `[{"number":42,"html_url":"u","title":"t","state":"closed","merged_at":"2026-01-01T00:00:00Z"}]`,
		},
		"gitlab": {
			repo: gitlabRepo(), findPath: gitlabMergesPath,
			body: `[{"iid":8,"web_url":"u","title":"t","state":"merged"}]`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeRouting(t, map[string]string{tt.findPath: tt.body})

			// Act
			pull, found, err := client.FindPullRequest(t.Context(), tt.repo, featureBranch)

			// Assert
			if err != nil || !found || pull.State != forge.StateMerged {
				t.Errorf("FindPullRequest = %+v, %v, %v, want a merged pull", pull, found, err)
			}
		})
	}
}

func TestOpenedReportsOnlyTheOpenState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		state forge.PullState
		want  bool
	}{
		"open":   {state: forge.StateOpen, want: true},
		"merged": {state: forge.StateMerged, want: false},
		"closed": {state: forge.StateClosed, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			pull := forge.PullRequest{State: tt.state}

			// Act & Assert
			if got := pull.Opened(); got != tt.want {
				t.Errorf("PullRequest{%v}.Opened() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestFindPullRequestPrefersAnOpenPullOverAMerged(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch was reused: an old merged pull and a new open one.
	client, _ := forgeRouting(t, map[string]string{
		githubPullsPath: `[{"number":9,"state":"closed","merged_at":"2026-01-01T00:00:00Z"},` +
			`{"number":42,"state":"open"}]`,
	})

	// Act
	pull, found, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || !found || pull.Number != 42 || pull.State != forge.StateOpen {
		t.Errorf("FindPullRequest = %+v, %v, %v, want the open #42", pull, found, err)
	}
}

func TestFindPullRequestIgnoresAClosedButUnmergedPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// A pull closed without merging is not the branch's business — it may open a
	// new one.
	client, _ := forgeRouting(t, map[string]string{
		githubPullsPath: `[{"number":42,"state":"closed"}]`,
	})

	// Act
	_, found, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || found {
		t.Errorf("FindPullRequest found = %v (err %v), want a closed-unmerged pull ignored", found, err)
	}
}
