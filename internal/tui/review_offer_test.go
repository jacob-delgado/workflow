// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

// The Review pane's sentence for n: after a merged pull request, and where
// none was found.
const (
	nOpensANewOne = "n opens a new pull request"
	nOpensOne     = "n opens one"
)

// mergedWithACommitSince is a merged branch that has since gained a commit not
// yet pushed.
func mergedWithACommitSince() *world {
	w := mergedBranch()
	w.branch.Ahead = 1

	return w
}

// failedFind is the world before a pull request is opened, with the find
// failing for cause, as a forge does that cannot answer it.
func failedFind(cause error) func() *world {
	return func() *world {
		w := withoutPull()
		w.pullErr = fmt.Errorf("finding the pull request: %w", cause)

		return w
	}
}

// TestNIsOfferedWhileNoPullRequestIsOpen holds the Review pane to the rule
// workflow pr and the web compose by: a new pull request is refused only while
// one is open, and a find that fails for any reason but a missing token lets
// the open itself answer.
func TestNIsOfferedWhileNoPullRequestIsOpen(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world  func() *world
		detail string
	}{
		"merged, nothing since":      {world: mergedBranch, detail: nOpensANewOne},
		"merged, a commit since":     {world: mergedWithACommitSince, detail: nOpensANewOne},
		"the forge did not answer":   {world: failedFind(forge.ErrUnreachable), detail: nOpensOne},
		"the forge refused the find": {world: failedFind(forge.ErrRefused), detail: nOpensOne},
		"the forge asked to wait":    {world: failedFind(httpx.ErrRateLimited), detail: nOpensOne},
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

func TestAFailedFindOffersNoNewPullRequestBesideAnOpenOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request found open is kept through a later find that fails, and
	// it is still the open one n would duplicate.
	open := newWorld()
	model := open.live(t, 120, 40)
	open.pullFound, open.pullErr = false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)

	// Act
	view := typing(t, model, "4", "r").View().Content

	// Assert
	requireScreen(t, view, "#42 "+pullTitle, "could not reach the forge")
	refuseScreen(t, footerLine(view), "open pull request")
}
