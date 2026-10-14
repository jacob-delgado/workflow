// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestADrainRunsASequenceOfCommandsInOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)
	toReview := func() tea.Msg { return keyMsg("4") }
	toHelp := func() tea.Msg { return keyMsg("?") }

	// Act
	drained := drain(t, model, tea.Sequence(toReview, toHelp))

	// Assert
	// The Review pane holds the focus behind the keys, its rule drawn heavy:
	// run the other way round, the open keys would have taken the 4.
	requireScreen(t, drained.View().Content, "Keys", "━ 4 Review ━")
}
