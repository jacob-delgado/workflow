// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

// 80 by 24 is the size a terminal opens at: the rail keeps its 24 columns and
// the detail, 56 wide, loses its border.
const (
	openingWidth  = 80
	openingHeight = 24
)

func TestAtEightyByTwentyFourTheFocusedPaneIsMarkedWithTheDetailBorderless(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"a pane whose list is in the detail": {"3"},
		"with an overlay open":               {"3", "c"},
	}

	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := plain(typing(t, newWorld().live(t, openingWidth, openingHeight), keys...).View().Content)

			// Assert
			if rule := lineHolding(view, "3 Commits"); !strings.Contains(rule, "━") {
				t.Errorf("the Commits rule is %q, want it drawn heavy for focus:\n%s", rule, view)
			}
		})
	}
}

func TestAtEightyByTwentyFourANoticeLeavesTheFocusedPaneItsFourRows(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := newWorld().live(t, openingWidth, openingHeight)

	// Act
	view := plain(typing(t, opened, "m").View().Content)

	// Assert
	requireScreen(t, view, "mouse o")

	if rows := railRowsUnder(view, "1 Issues"); rows != 4 {
		t.Errorf("the focused Issues pane has %d rows with a notice showing, want 4:\n%s", rows, view)
	}
}

// railRowsUnder counts the rail's content rows under the rule titled title, up
// to the next rule.
func railRowsUnder(view, title string) int {
	rows, counting := 0, false

	for line := range strings.Lines(view) {
		switch {
		case strings.Contains(line, title):
			counting = true
		case counting && (strings.Contains(line, "─") || strings.Contains(line, "━")):
			return rows
		case counting:
			rows++
		}
	}

	return rows
}

func TestAtEightyByTwentyFourAnOverlaysFooterKeepsEsc(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo func() *world
		keys []string
	}{
		"the calendar":              {repo: summaryWorld, keys: calendarOpen()},
		"the commit, being written": {repo: newWorld, keys: []string{"3", "c"}},
		"the pull request composer": {repo: withoutPull, keys: []string{"4", "n"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, tt.repo().live(t, openingWidth, openingHeight), tt.keys...).View().Content

			// Assert
			requireScreen(t, footerLine(view), "esc close")
		})
	}
}

func TestAtEightyByTwentyFourAPanesFooterKeepsTheWayOut(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, reposWorld().live(t, openingWidth, openingHeight), reposKey).View().Content

	// Assert
	requireScreen(t, footerLine(view), "? keys", quitHint)
}

func TestAtEightyByTwentyFourTheCommitComposerKeepsEnterOverItsOtherVerbs(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, openingWidth, openingHeight), "3", "c").View().Content

	// Assert
	requireScreen(t, footerLine(view), "enter commit", escCloses)
}

func TestAtEightyByTwentyFourASearchBeingTypedSharesTheFooterRow(t *testing.T) {
	t.Parallel()

	// Act
	view := plain(typing(t, newWorld().live(t, openingWidth, openingHeight), "/", "F", "i").View().Content)

	// Assert
	requireScreen(t, footerLine(view), "search: Fi", "esc clear search")

	if rows := railRowsUnder(view, "1 Issues"); rows != 4 {
		t.Errorf("the Issues pane has %d rows while a search is typed, want 4:\n%s", rows, view)
	}
}
