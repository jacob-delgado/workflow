// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Scope validation and the ctrl+e boundary, split from composer_test.go.

import (
	"strings"
	"testing"
)

func TestTheComposerFlagsAnInvalidScopeAsItIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	opened := typing(t, composing.live(t, 120, 40), "3", "c", keyShiftTab)

	// Act
	view := typing(t, opened, letters("BAD")...).View()

	// Assert
	lines := strings.Split(view, "\n")
	scopeRow := -1

	for index, line := range lines {
		if strings.Contains(line, "scope") {
			scopeRow = index

			break
		}
	}

	if scopeRow < 0 || scopeRow+1 >= len(lines) || !strings.Contains(lines[scopeRow+1], "a scope is lowercase letters") {
		t.Errorf("the scope error is not shown under the scope field:\n%s", view)
	}
}

func TestCtrlEIsLeftToTheSubjectField(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	composer := typing(t, editing.live(t, 120, 40), "3", "c")

	// Act
	typing(t, composer, append(letters("redact tokens"), "ctrl+e")...)

	// Assert
	if calls := editing.asked("edit"); len(calls) != 0 {
		t.Errorf("ctrl+e opened the editor: %q", calls)
	}
}
