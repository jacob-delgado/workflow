// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

// mergedWithACommitSince is a merged branch that has since gained a commit not
// yet pushed.
func mergedWithACommitSince() *world {
	w := mergedBranch()
	w.branch.Ahead = 1

	return w
}

// TestNIsOfferedWhileNoPullRequestIsOpen holds the Review pane to the rule
// workflow pr and the web compose by: a new pull request is refused only while
// one is open.
func TestNIsOfferedWhileNoPullRequestIsOpen(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world  func() *world
		detail string
	}{
		"merged, nothing since":  {world: mergedBranch, detail: "n opens a new pull request"},
		"merged, a commit since": {world: mergedWithACommitSince, detail: "n opens a new pull request"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			offering := tt.world()
			model := offering.live(t, 120, 40)

			// Act: look at the Review pane
			review := typing(t, model, "4")

			// Assert: n is offered, and the pane says what it does
			view := review.View().Content
			requireScreen(t, view, tt.detail)
			requireScreen(t, footerLine(view), "n open pull request")

			// Act: open one
			typing(t, review, "n", keyEnter)

			// Assert: the forge was asked to open it
			if calls := offering.asked("open "); len(calls) != 1 {
				t.Errorf("open calls = %q, want one new pull request", calls)
			}
		})
	}
}
