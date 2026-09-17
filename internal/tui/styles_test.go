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
	footer := footerLine(model.View())

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
			view := staged.live(t, 120, 40).View()

			// Assert
			requireScreen(t, view, tt.sentence)

			if strings.Contains(view, "\x1b[2m"+tt.sentence) {
				t.Errorf("%s is drawn faint:\n%q", name, view)
			}
		})
	}
}
