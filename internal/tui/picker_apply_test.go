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

	var searches atomic.Int32

	search := func() (jira.SearchResult, error) {
		searches.Add(1)

		return twoIssues()()
	}

	fake := &fakeJira{moves: workflowMoves()}
	screen := press(t, openPicker(t, jiraScreen(t, fake.deps(search))), "j")

	sending, cmd := pressed(t, screen, keyEnter)

	if view := sending.View(); !strings.Contains(view, "moving OPS-1 to Done…") {
		t.Errorf("the picker does not say the move is under way:\n%s", view)
	}

	moved, refresh := finish(t, sending, cmd)

	if got := fake.applied.Load(); got != "OPS-1 31" || fake.applies.Load() != 1 {
		t.Errorf("applied %v %d times, want OPS-1 through transition 31 once", got, fake.applies.Load())
	}

	view := moved.View()
	if strings.Contains(view, pickerTitle) || !strings.Contains(view, "● OPS-1 moved to Done") {
		t.Errorf("a move that worked did not close the picker and say so:\n%s", view)
	}

	finish(t, moved, refresh)

	if searches.Load() != 2 {
		t.Errorf("searched %d times, want the list refreshed after the move", searches.Load())
	}
}

func TestNothingInterruptsAMoveBeingSent(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	sending, _ := pressed(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), keyEnter)

	// Nothing may send twice, and nothing may hide an answer still to come.
	if _, again := pressed(t, sending, keyEnter); again != nil {
		t.Error("a second enter sent the transition again")
	}

	if view := press(t, sending, "esc").View(); !strings.Contains(view, pickerTitle) {
		t.Errorf("esc closed the picker with the answer still to come:\n%s", view)
	}

	// So the footer offers neither: only quitting still does anything.
	lines := strings.Split(sending.View(), "\n")
	if footer := lines[len(lines)-1]; strings.Contains(footer, "apply") || strings.Contains(footer, "esc") ||
		!strings.Contains(footer, "quit") {
		t.Errorf("the footer offers keys that do nothing while the move is sent: %q", footer)
	}
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

			screen, cmd := pressed(t, screen, keyEnter)
			screen, refresh := finish(t, screen, cmd)
			screen, _ = finish(t, screen, refresh)

			if view := screen.View(); !strings.Contains(view, tt.want) {
				t.Errorf("after the refresh, want %q selected:\n%s", tt.want, view)
			}
		})
	}
}

func TestTheNoticeGivesWayToTheKeysOnTheNextPress(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	screen, cmd := pressed(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), keyEnter)
	screen, _ = finish(t, screen, cmd)

	view := press(t, screen, "j").View()
	if strings.Contains(view, "moved to") || !strings.Contains(view, "quit") {
		t.Errorf("the notice outlived the next key press:\n%s", view)
	}
}

func TestARefusedMoveKeepsThePickerOpenToTryAgain(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{
		moves:    workflowMoves(),
		applyErr: fmt.Errorf("%w: %s", jira.ErrRejected, "Resolution is required."),
	}

	screen, cmd := pressed(t, press(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), "j"), keyEnter)
	screen, refresh := finish(t, screen, cmd)

	view := screen.View()

	reason := "✗ jira rejected the request: Resolution is required."
	if !strings.Contains(view, pickerTitle) || !strings.Contains(view, reason) {
		t.Errorf("a refused move did not stay open with Jira's reason:\n%s", view)
	}

	if refresh != nil {
		t.Error("a refused move refreshed the list, want nothing to have changed")
	}

	if !strings.Contains(view, "▸ ● Done") {
		t.Errorf("a refused move lost the selection:\n%s", view)
	}

	if _, retry := pressed(t, screen, keyEnter); retry == nil {
		t.Error("enter after a refusal did not try again")
	}
}

func TestEnterBeforeTheListingArrivesSendsNothing(t *testing.T) {
	t.Parallel()

	fake := &fakeJira{moves: workflowMoves()}
	loading, _ := pressed(t, jiraScreen(t, fake.deps(twoIssues())), "t")

	if _, cmd := pressed(t, loading, keyEnter); cmd != nil {
		t.Error("enter sent a transition before any were listed")
	}
}
