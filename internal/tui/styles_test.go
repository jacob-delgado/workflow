// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestTheFooterUsesTheThemeNotFixedGrays(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	model := newWorld().live(t, 120, 40)

	// Act
	// This asserts on the color escapes, so it needs the raw last line rather than
	// the stripped footerLine the plain-text tests use.
	lines := strings.Split(model.View().Content, "\n")
	footer := lines[len(lines)-1]

	// Assert
	if !strings.Contains(footer, "\x1b[1m") {
		t.Errorf("the footer draws no bold key:\n%q", footer)
	}

	if !strings.Contains(footer, "\x1b[2m") {
		t.Errorf("the footer draws no faint description:\n%q", footer)
	}
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestEmptyStateSentencesAreNotDrawnFaint(t *testing.T) {
	defer forceANSI(t)()

	cases := map[string]struct {
		prepare  func(*world)
		sentence string
	}{
		"the review empty state": {
			prepare:  func(w *world) { w.branch = gitrepo.Branch{Name: baseName, Base: baseRef}; w.pullFound = false },
			sentence: "on no feature branch",
		},
		"the slack empty state": {
			prepare:  func(w *world) { w.branch = gitrepo.Branch{Name: baseName, Base: baseRef}; w.pullFound = false },
			sentence: "○ nothing posted",
		},
		"the branch upstream state": {
			prepare:  func(w *world) { w.branch.Upstream = "" },
			sentence: "not pushed yet",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			staged := newWorld()
			tt.prepare(staged)

			// Act
			view := staged.live(t, 120, 40).View().Content

			// Assert
			requireScreen(t, view, tt.sentence)

			if strings.Contains(view, "\x1b[2m"+tt.sentence) {
				t.Errorf("%s is drawn faint:\n%q", name, view)
			}
		})
	}
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestNoColorKeepsTheCursorButDropsTheHue(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	// Color removed, with a text field open so a cursor is on screen.
	model := newWorld().live(t, 120, 40).WithoutColor()
	field := typing(t, model, "2", "b")

	// Act
	view := field.View().Content

	// Assert
	for _, hue := range []string{"\x1b[31m", "\x1b[32m", "\x1b[33m", "\x1b[34m", "\x1b[35m"} {
		if strings.Contains(view, hue) {
			t.Errorf("a hue %q is drawn with color off:\n%q", hue, view)
		}
	}

	// The cursor is reverse video, which v2 may combine with other attributes
	// (\x1b[7;37m), so match the reverse SGR by its start rather than alone.
	if !strings.Contains(view, "\x1b[7") {
		t.Errorf("the text cursor is gone with color off:\n%q", view)
	}
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestTheFocusedPaneWearsABoldTitle(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	// Focus starts on Issues.
	model := newWorld().live(t, 120, 40)

	// Act
	view := model.View().Content

	// Assert
	if !strings.Contains(view, "\x1b[1m1 Issues") {
		t.Errorf("the focused pane's title is not bold:\n%q", view)
	}

	if strings.Contains(view, "\x1b[1m2 Branch") {
		t.Errorf("an unfocused pane's title is bold:\n%q", view)
	}
}
