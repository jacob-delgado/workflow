// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strings"
	"testing"
)

func TestARefusalStaysInSightWhileAFormIsOpenOverTheRows(t *testing.T) {
	t.Parallel()

	// The write refused the project, and a form then opens over the rows,
	// which hides the project's row and the refusal under it: on the
	// project's own section, or on another's.
	cases := map[string]struct {
		keys []string
	}{
		"the refused setting's own section": {keys: []string{keyEnter}},
		"another section": {
			keys: append(slices.Repeat([]string{"j"}, branchTemplateRow-projectRow), keyEnter),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			repo.saveSettingsErr = fileRefusing("jira.project")
			refused := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...)

			// Act
			view := typing(t, refused, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, "✗ Jira › Project: "+errRefusedWhereWritten.Error())
		})
	}
}

func TestEachRefusalAtTheFootNamesItsSettingOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// The write refuses the project, the keys, a collection of a row per
	// action, and the views, a collection with no entry, whose one row adds
	// one; a form then opens over the rows.
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project", "ui.keys", "jira.views")
	refused := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...)

	// Act
	view := typing(t, refused, keyEnter).View().Content

	// Assert
	written := errRefusedWhereWritten.Error()
	requireScreen(t, view, "✗ Jira › Project: "+written, "✗ Jira › Views: "+written)

	if said := strings.Count(plain(view), "✗ Keyboard › Keys: "+written); said != 1 {
		t.Errorf("the keys' refusal is said %d times at the foot, want once:\n%s", said, plain(view))
	}
}

func TestWithNoFormOpenARefusalIsSaidUnderItsRowAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project")

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, editedProject)...).View().Content

	// Assert
	written := errRefusedWhereWritten.Error()
	if under := lineUnder(t, view, projectLabel); !strings.Contains(under, "✗ "+written) {
		t.Errorf("under the project's row is %q, want why it was refused:\n%s", under, plain(view))
	}

	refuseScreen(t, view, "✗ Jira › Project: "+written)
}

func TestARefusalAtTheFootIsNamedInASCII(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := typing(t, asciiInterface(t, newWorld(), 120, 40), editing(branchTemplateRow, "no-key")...)

	// Act
	view := typing(t, refused, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "x Branch > Name template: "+templateRefused)
}

func TestAFormMakesRoomAtEightyByTwentyFourForEveryRefusalStanding(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = fileRefusing("jira.project", "jira.views", "branch.template", "ui.keys")
	refused := typing(t, repo.live(t, 80, 24), editing(projectRow, editedProject)...)

	// Act
	view := typing(t, refused, keyEnter).View().Content

	// Assert
	// Each is named at the start of its line; its reason wraps at this width.
	requireScreen(t, view, "✗ Jira › Project:", "✗ Jira › Views:", "✗ Branch › Name template:",
		"✗ Keyboard › Keys:")
}
