// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
)

// pickerField is what the picker's field shows, plain, after its prompt.
func pickerField(view string) string {
	return strings.TrimRight(pickerFieldRow(view), " ")
}

// pickerFieldRow is the picker's field from its prompt to the dialog's edge,
// blanks and all.
func pickerFieldRow(view string) string {
	for line := range strings.SplitSeq(plain(view), "\n") {
		if _, field, found := strings.Cut(line, "┃ › "); found {
			shown, _, _ := strings.Cut(field, "┃")

			return shown
		}
	}

	return ""
}

func TestRightDescendsIntoTheHighlightedDirectory(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), "~/src/w", "right").View().Content

	// Assert
	requireScreen(t, view, "› ~/src/web/", "in ~/src/web")
}

func TestEnterGoesIntoTheFirstMatchOfANamePastedWithItsPath(t *testing.T) {
	t.Parallel()

	// Arrange
	// The path and the name arrive together, before ~/src has been listed, so
	// the first match only appears once the listing answers.
	pasted := pasting(t, picking(t, pickingWorld(), ""), "~/src/w")

	// Act
	view := typing(t, pasted, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "› ~/src/web/", "in ~/src/web")
}

func TestHomeAndEndMoveTheFieldsCursor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		typed string
		keys  []string
		want  string
	}{
		"end, then right into web": {typed: "~/src/wb", keys: []string{keyLeft, keyLeft, keyEnd, keyRight}, want: webTyped},
		"home, then ~/ before it":  {typed: "src/web/", keys: []string{keyHome, "~", "/"}, want: webTyped},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := picking(t, pickingWorld(), tt.typed, tt.keys...).View().Content

			// Assert
			if got := pickerField(view); got != tt.want {
				t.Errorf("after %q and %v the field shows %q, want %q:\n%s", tt.typed, tt.keys, got, tt.want, plain(view))
			}
		})
	}
}

func TestALongPathScrollsTheFieldToItsCursor(t *testing.T) {
	t.Parallel()

	long := "~/" + strings.Repeat("deep/", 30) + "last"
	cases := map[string]struct {
		keys       []string
		start, end string
	}{
		"typed to its end":  {end: "deep/last"},
		"home to its start": {keys: []string{keyHome}, start: "~/deep/"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := picking(t, pickingWorld(), long, tt.keys...).View().Content

			// Assert
			row := pickerFieldRow(view)

			field := strings.TrimRight(row, " ")
			if !strings.HasPrefix(field, tt.start) || !strings.HasSuffix(field, tt.end) || strings.HasSuffix(field, "…") {
				t.Errorf("after typing a long path and %v the field shows %q, want it from %q to %q, not cut:\n%s",
					tt.keys, field, tt.start, tt.end, plain(view))
			}
			// No more is left blank than the cursor's cell and the dialog's padding.
			if blank := len(row) - len(field); blank > 2 {
				t.Errorf("after typing a long path and %v the field leaves %d columns blank, want at most 2:\n%s",
					tt.keys, blank, plain(view))
			}
		})
	}
}

func TestEndInTheListMovesTheHighlightToTheLastRow(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), srcTyped, keyTab, keyEnd).View().Content

	// Assert
	requireScreen(t, view, "▸ cli")
}

func TestBackspaceGoesUpOnlyFromAnEmptyNameAtTheEnd(t *testing.T) {
	t.Parallel()

	// An empty field shows where you work in its place.
	const shownHere = "~/src/api/cmd"

	cases := map[string]struct {
		typed string
		keys  []string
		want  string
	}{
		"an empty name":   {typed: srcTyped, want: "~/"},
		"inside the path": {typed: webTyped, keys: []string{keyLeft, keyLeft, keyLeft, keyLeft}, want: "~/srcweb/"},
		"a lone ~":        {typed: "~", want: shownHere},
		"the root":        {typed: "/", want: shownHere},
		"an empty field":  {typed: "", want: shownHere},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			typed := picking(t, pickingWorld(), tt.typed, tt.keys...)

			// Act
			view := typing(t, typed, keyBackspace).View().Content

			// Assert
			if got := pickerField(view); got != tt.want {
				t.Errorf("backspace after %q left the field %q, want %q:\n%s", tt.typed, got, tt.want, plain(view))
			}
		})
	}
}

func TestJTypesInTheFieldButMovesInTheListAfterTab(t *testing.T) {
	t.Parallel()

	// Arrange
	browsing := picking(t, pickingWorld(), srcTyped)

	// Act: j in the field
	typed := typing(t, browsing, "j")

	// Assert: it is typed
	requireScreen(t, typed.View().Content, "› ~/src/j")

	// Act: tab, then j in the list
	moved := typing(t, browsing, keyTab, "j")

	// Assert: it moves the highlight to the first directory
	requireScreen(t, moved.View().Content, "› ~/src/ ", "▸ api")
}

func TestLeftInTheListGoesUp(t *testing.T) {
	t.Parallel()

	// Act
	view := picking(t, pickingWorld(), srcTyped, keyTab, "left").View().Content

	// Assert
	requireScreen(t, view, "› ~/", "in ~")
}

func TestSwitchHereOpensALastLookAndEscGoesBackToThePicker(t *testing.T) {
	t.Parallel()

	// Arrange
	browsing := picking(t, pickingWorld(), webTyped)

	// Act: enter on switch here
	looking := typing(t, browsing, keyEnter)

	// Assert: the last look asks, over the picker it goes back to
	requireScreen(t, looking.View().Content, "Switch directory", "Switch to ~/src/web?", dirPickerTitle)

	// Act: esc
	back := typing(t, looking, keyEsc)

	// Assert: the picker is where it was
	requireScreen(t, back.View().Content, dirPickerTitle, "› ~/src/web/")
	refuseScreen(t, back.View().Content, "Switch to ~/src/web?")
}

func TestEnterAndSOnSwitchHereGoToTheSameDirectory(t *testing.T) {
	t.Parallel()

	// Below directories matching the name typed, switching here is switching
	// to where the field points; with none matching, to the path typed.
	cases := map[string]struct {
		listed []string
		typed  string
		keys   []string
		want   string
	}{
		"enter below matches": {typed: "~/src/w", keys: []string{keyDown, keyDown, keyEnter}, want: "Switch to ~/src?"},
		"s below matches":     {typed: "~/src/w", keys: []string{keyDown, keyDown, keyTab, "s"}, want: "Switch to ~/src?"},
		"enter with none listed": {
			listed: []string{apiDir}, typed: "~/src/web", keys: []string{keyEnter}, want: "Switch to ~/src/web?",
		},
		"s with none listed": {
			listed: []string{apiDir}, typed: "~/src/web", keys: []string{keyTab, "s"}, want: "Switch to ~/src/web?",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			working := pickingWorld()
			working.dirs.places[srcDir] = seams.Place{Dir: srcDir}

			if tt.listed != nil {
				working.dirs.listings[srcDir] = tt.listed
			}

			// Act
			view := picking(t, working, tt.typed, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}
