// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

// setBack opens row's form, keeps changed for it, then opens it again and
// keeps read, the value Settings read for it.
func setBack(row int, changed, read string) []string {
	keys := append(edited(row, changed), keyEnter, keyClearLine)
	if read != "" {
		keys = append(keys, pasted(read))
	}

	return append(keys, saveKey)
}

func TestACountSetBackToItsReadValueIsNotEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := setBack(subjectLimitRow, "50", "0")

	// Act
	view := typing(t, newWorld().live(t, 120, 40), keys...).View().Content

	// Assert
	refuseScreen(t, view, "(edited)")
	requireScreen(t, footerLine(view), "esc close")
}

func TestASettingSetBackToItsReadValueIsNotEdited(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		row           func(t *testing.T) int
		changed, read string
	}{
		"a list":            {row: func(*testing.T) int { return commitTypesRow }, changed: "feat, fix"},
		"a key put back":    {row: commentKeyRow, changed: "C"},
		"a text typed back": {row: func(*testing.T) int { return projectRow }, changed: editedProject, read: readProject},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			keys := setBack(test.row(t), test.changed, test.read)

			// Act
			view := typing(t, newWorld().live(t, 120, 40), keys...).View().Content

			// Assert
			refuseScreen(t, view, "(edited)")
			requireScreen(t, footerLine(view), "esc close")
		})
	}
}
