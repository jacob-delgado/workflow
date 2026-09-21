// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The picker footer and narrow-terminal fit, split from picker_test.go.

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestThePickerFooterSaysHowToApplyOrLeave(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	screen := jiraScreen(t, fake.deps(twoIssues()))

	// Act
	footer := footerLine(openPicker(t, screen).View().Content)

	// Assert
	requireScreen(t, footer, "enter apply", keyEsc)
}

func TestThePickerFitsANarrowTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	model := started(t, sized(t, tui.New(completeConfig(), nil, fake.deps(twoIssues())), 79, 30))

	// Act
	view := openPicker(t, model).View().Content

	// Assert
	requireScreen(t, view, pickerTitle, "▸ ◐ Start Review → In Review")

	for index, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 80 {
			t.Errorf("line %d is %d cells, wider than the terminal: %q", index, width, line)
		}
	}
}
