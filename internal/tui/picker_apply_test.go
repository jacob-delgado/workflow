// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestEnterMovesTheIssueAndRefreshesTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	var searches atomic.Int32

	search := func() (jira.SearchResult, error) {
		searches.Add(1)

		return twoIssues()()
	}

	fake := &fakeJira{moves: workflowMoves()}
	screen := press(t, openPicker(t, jiraScreen(t, fake.deps(search))), "j")

	// Act: choose Done
	sending, cmd := pressed(t, screen, keyEnter)

	// Assert: the picker says the move is under way
	requireScreen(t, sending.View(), "moving OPS-1 to Done…")

	// Act: Jira accepts the move
	moved, refresh := finish(t, sending, cmd)

	// Assert: it was sent once, and the picker closes saying so
	if got := fake.applied.Load(); got != "OPS-1 31" || fake.applies.Load() != 1 {
		t.Errorf("applied %v %d times, want OPS-1 through transition 31 once", got, fake.applies.Load())
	}

	refuseScreen(t, moved.View(), pickerTitle)
	requireScreen(t, moved.View(), "● OPS-1 moved to Done")

	// Act: the refresh arrives
	finish(t, moved, refresh)

	// Assert: the list was searched again
	if searches.Load() != 2 {
		t.Errorf("searched %d times, want the list refreshed after the move", searches.Load())
	}
}

func TestNothingInterruptsAMoveBeingSent(t *testing.T) {
	t.Parallel()

	// Nothing may send twice, and nothing may hide an answer still to come.
	for _, key := range []string{keyEnter, keyEsc} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeJira{moves: workflowMoves()}
			sending, _ := pressed(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), keyEnter)

			// Act
			after, cmd := pressed(t, sending, key)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command while the move is sent", key)
			}

			if after.View() != sending.View() {
				t.Errorf("%s changed the screen while the move is sent:\n%s", key, after.View())
			}
		})
	}
}

func TestTheFooterOffersOnlyQuittingWhileAMoveIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))

	// Act
	sending, _ := pressed(t, screen, keyEnter)

	// Assert
	footer := footerLine(sending.View())
	refuseScreen(t, footer, "apply", keyEsc)
	requireScreen(t, footer, "quit")
}

func TestTheSelectionFollowsTheMovedIssueThroughARefresh(t *testing.T) {
	t.Parallel()

	// OPS-2, the middle row, is selected and moved. Each refresh is one where
	// keeping the row, or going back to the top, would select something else.
	cases := map[string]struct {
		refreshed []jira.Issue
		want      string
	}{
		"re-sorted to the top": {
			refreshed: []jira.Issue{
				issue("OPS-2", "Rotate keys", "indeterminate"), issue("OPS-1", "Fix login", "new"),
				issue("OPS-3", "Ship it", "new"),
			},
			want: "▸ ◐ OPS-2",
		},
		"left where it was": {
			refreshed: []jira.Issue{
				issue("OPS-1", "Fix login", "new"), issue("OPS-3", "Ship it", "new"),
				issue("OPS-2", "Rotate keys", "indeterminate"),
			},
			want: "▸ ◐ OPS-2",
		},
		// Done, it drops out of the list: the row stays put and so lands on its
		// neighbor, as deleting from any list does.
		"gone from the list": {
			refreshed: []jira.Issue{issue("OPS-1", "Fix login", "new"), issue("OPS-3", "Ship it", "new")},
			want:      "▸ ○ OPS-3",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var calls atomic.Int32

			search := func() (jira.SearchResult, error) {
				if calls.Add(1) == 1 {
					return assigned(
						issue("OPS-1", "Fix login", "new"), issue("OPS-2", "Rotate keys", "new"),
						issue("OPS-3", "Ship it", "new"),
					)()
				}

				return jira.SearchResult{Issues: tt.refreshed, Total: len(tt.refreshed)}, nil
			}

			fake := &fakeJira{moves: workflowMoves()}
			screen := openPicker(t, press(t, jiraScreen(t, fake.deps(search)), "j"))

			// Act
			screen, cmd := pressed(t, screen, keyEnter)
			screen, refresh := finish(t, screen, cmd)
			screen, _ = finish(t, screen, refresh)

			// Assert
			if view := screen.View(); !strings.Contains(view, tt.want) {
				t.Errorf("after the refresh, want %q selected:\n%s", tt.want, view)
			}
		})
	}
}

func TestTheNoticeGivesWayToTheKeysOnTheNextPress(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen, cmd := pressed(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), keyEnter)
	screen, _ = finish(t, screen, cmd)

	// Act
	footer := footerLine(press(t, screen, "j").View())

	// Assert
	refuseScreen(t, footer, "moved to")
	requireScreen(t, footer, "quit")
}

func TestARefusedMoveKeepsThePickerOpenToTryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{
		moves:    workflowMoves(),
		applyErr: fmt.Errorf("%w: %s", jira.ErrRejected, "Resolution is required."),
	}

	screen, cmd := pressed(t, press(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), "j"), keyEnter)

	// Act: Jira refuses the move
	refused, refresh := finish(t, screen, cmd)

	// Assert: the picker stays open, with Jira's reason and the selection
	requireScreen(t, refused.View(), pickerTitle, "✗ jira rejected the request: Resolution is required.", "▸ ● Done")

	if refresh != nil {
		t.Error("a refused move refreshed the list, want nothing to have changed")
	}

	// Act: try again
	retrying, retry := pressed(t, refused, keyEnter)
	finish(t, retrying, retry)

	// Assert: the move was sent a second time
	if got := fake.applies.Load(); got != 2 {
		t.Errorf("applied %d times, want the retry sent", got)
	}
}

func TestEnterBeforeTheListingArrivesSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	loading, _ := pressed(t, jiraScreen(t, fake.deps(twoIssues())), "t")

	// Act
	_, cmd := pressed(t, loading, keyEnter)

	// Assert
	if cmd != nil {
		t.Error("enter sent a transition before any were listed")
	}
}
