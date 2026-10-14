// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// cornerOf is the row and column a dialog titled title starts at on a plain
// screen, and whether it is there at all.
func cornerOf(view, title string) (int, int, bool) {
	for row, line := range strings.Split(plain(view), "\n") {
		if before, _, found := strings.Cut(line, "─ "+title+" "); found {
			return row, len([]rune(before)) - 1, true
		}

		if before, _, found := strings.Cut(line, "━ "+title+" "); found {
			return row, len([]rune(before)) - 1, true
		}
	}

	return 0, 0, false
}

// detailColumn is where the detail pane starts at 120 columns, right of the
// rail; an overlay drawn in the pane starts there, and a dialog left of it.
const detailColumn = 36

// discardLook is the world on the Commits pane with the last look before a
// discard open, at 120 by 40.
func discardLook(t *testing.T) (*world, string) {
	t.Helper()

	discarding := newWorld()
	discarding.changes = workTree()
	discarding.diff = []string{"-\tkeep := true", "+\tkeep := false"}

	return discarding, typing(t, discarding.live(t, 120, 40), "3", "j", "x").View().Content
}

func TestTheKeysFloatOverTheScreenWithTheRailBehind(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "?").View().Content

	// Assert
	row, column, found := cornerOf(view, "Keys")
	if !found {
		t.Fatalf("no dialog titled Keys:\n%s", plain(view))
	}

	behind := []rune(strings.Split(plain(view), "\n")[row])[:column]
	if column >= detailColumn || strings.TrimSpace(string(behind)) == "" {
		t.Errorf("the keys are not a dialog with the rail showing left of it (column %d):\n%s", column, plain(view))
	}
}

func TestALastLookFloatsAndEscGoesBack(t *testing.T) {
	t.Parallel()

	// Arrange
	discarding, looking := discardLook(t)

	// Act
	model := typing(t, discarding.live(t, 120, 40), "3", "j", "x", keyEsc)

	// Assert
	if _, column, found := cornerOf(looking, "Discard changes"); !found || column >= detailColumn {
		t.Errorf("the last look is not a dialog over the middle of the screen (column %d):\n%s", column, plain(looking))
	}

	refuseScreen(t, model.View().Content, "Discard changes")
}

func TestTheBackdropBehindADialogIsFaint(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "?").View().Content

	// Assert
	faint := opening(lipgloss.NewStyle().Faint(true))
	lines := strings.Split(view, "\n")

	if last := lines[len(lines)-2]; !strings.HasPrefix(last, faint) {
		t.Errorf("the body's last row behind the dialog is not faint: %q", last)
	}

	if strings.HasPrefix(lines[0], faint) || strings.HasPrefix(lines[len(lines)-1], faint) {
		t.Errorf("the spine or the footer went faint behind the dialog")
	}
}

func TestAClickOutsideADialogDoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	listing := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), placeKey)
	before := listing.View().Content

	// Act
	// In the detail pane behind the dialog, above and right of it, on the row
	// that would be the checklist's second were it drawn in the pane.
	after := click(t, listing, placesViewWidth-10, 3).View().Content

	// Assert
	if after != before {
		t.Errorf("a click outside the dialog changed the screen:\n%s", plain(after))
	}
}

func TestAClickInsideADialogPicksTheRow(t *testing.T) {
	t.Parallel()

	// Arrange
	listing := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), placeKey)

	row, column := -1, -1

	for index, line := range strings.Split(plain(listing.View().Content), "\n") {
		if before, _, found := strings.Cut(line, "[ ] "+statusInDev); found {
			row, column = index, ansi.StringWidth(before)
		}
	}

	if row < 0 {
		t.Fatalf("no %q row in the checklist:\n%s", statusInDev, plain(listing.View().Content))
	}

	// Act
	picked := click(t, listing, column+1, row).View().Content

	// Assert
	requireScreen(t, picked, "▸ [ ] "+statusInDev)
}

// dialogWords is everything a dialog titled title says, its lines read inside
// its own frame and joined by spaces, so a sentence it wraps reads whole.
func dialogWords(t *testing.T, view, title string) string {
	t.Helper()

	lines := strings.Split(plain(view), "\n")

	row, left, found := cornerOf(view, title)
	if !found {
		t.Fatalf("no dialog titled %q:\n%s", title, plain(view))
	}

	var words []string

	for _, line := range lines[row+1:] {
		runes := []rune(line)
		if len(runes) <= left || (runes[left] != '┃' && runes[left] != '│') {
			break
		}

		inside, _, _ := strings.Cut(string(runes[left+1:]), string(runes[left]))
		if trimmed := strings.TrimSpace(inside); trimmed != "" {
			words = append(words, trimmed)
		}
	}

	return strings.Join(words, " ")
}

func TestALastLookIsMeasuredAtTheWidthItIsDrawn(t *testing.T) {
	t.Parallel()

	// A diff line as wide as the screen less the dialog's margins, which the
	// look wraps where it is drawn, inside its frame, and must measure there.
	long := "+" + strings.Repeat("abcde", 15)

	cases := map[string]struct {
		width, height int
		diff          []string
	}{
		"at 70 columns":           {width: 70, height: 30, diff: []string{"-\tretries := 1", "+\tretries := 3"}},
		"at 80, with a long line": {width: 80, height: 40, diff: []string{"-\tretries := 1", long}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			discarding := newWorld()
			discarding.changes = workTree()
			discarding.diff = tt.diff

			// Act
			view := typing(t, discarding.live(t, tt.width, tt.height), "3", "j", "x").View().Content

			// Assert
			if said := dialogWords(t, view, "Discard changes"); !strings.Contains(said, "This cannot be undone.") {
				t.Errorf("the look lost its last line; it says %q:\n%s", said, plain(view))
			}
		})
	}
}

func TestADialogNarrowerThanSixtyColumnsIsFramedOnAWideScreen(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, placesWorld().live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, filterTitleBar)
}

func TestTheKeysSayThereIsMoreBelowOnAMiddlingScreen(t *testing.T) {
	t.Parallel()

	// Between sixty and eighty columns the key list is a dialog narrower than
	// sixty, framed because the screen is not, and taller than the body.
	for _, cols := range []int{60, 70, 78} {
		t.Run(strconv.Itoa(cols), func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newWorld().live(t, cols, 24), "?").View().Content

			// Assert
			requireScreen(t, view, "┌─ Keys", "more below")
		})
	}
}
