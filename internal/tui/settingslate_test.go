// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// commitAnswers answer the commit section's four questions: the scope kept,
// the types and the limit typed, the trailer kept.
const commitAnswers = "\nfeat,fix\n50\n\n"

// promptedSettings is the world's interface asking Settings' forms as plain
// prompts, answered with commitAnswers; the count grows as each command
// asking them runs, so withheldKeys can keep their answer back.
func promptedSettings(t *testing.T) (tui.Model, func() int) {
	t.Helper()

	repo, prompts := newWorld(), &answering{typed: commitAnswers}

	var ran atomic.Int32

	deps := repo.deps()
	deps.Forms.Ask = func(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
		ask := prompts.ask(form, done)

		return func() tea.Msg {
			ran.Add(1)

			return ask()
		}
	}
	model := sized(t, tui.New(repo.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	return drain(t, model, model.Init()), func() int { return int(ran.Load()) }
}

func TestASectionAnsweredOnceItsFormClosedKeepsNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		after []string
		want  string
	}{
		"its form closed, Settings still open": {after: []string{keyEsc}, want: "Subject limit"},
		"Settings closed":                      {after: []string{keyEsc, keyEsc}, want: "9 Repositories"},
		"another form opened over the rows":    {after: []string{keyEsc, keyEnter}, want: "Saves to"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model, ran := promptedSettings(t)
			asked, late := withheldKeys(t, model, ran, append(toRow(subjectLimitRow), keyEnter)...)
			moved, _ := withheldKeys(t, asked, ran, tt.after...)

			// Act
			view := handedOver(t, moved, late).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "feat, fix", "(edited)")
		})
	}
}

func TestASectionsOwnAnswerIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	model, ran := promptedSettings(t)
	asked, answer := withheldKeys(t, model, ran, append(toRow(subjectLimitRow), keyEnter)...)

	// Act
	view := handedOver(t, asked, answer).View().Content

	// Assert
	requireScreen(t, view, "feat, fix", "(edited)")
}

func TestAPasteWhileSettingsSavesChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	saving, _ := pressed(t, typing(t, newWorld().live(t, 120, 40), edited(projectRow, editedProject)...), saveKey)

	// Act
	after := pasting(t, saving, "x")

	// Assert
	if got, want := after.View().Content, saving.View().Content; got != want {
		t.Errorf("a paste while saving changed Settings:\n%s", plain(got))
	}
}

func TestSettingsRowsFollowTheTerminalWhenItIsResized(t *testing.T) {
	t.Parallel()

	// Arrange
	wide := typing(t, newWorld().live(t, 120, 40), toRow(projectRow)...)

	// Act
	narrowed, cmd := wide.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	narrow := drain(t, concrete(t, narrowed), cmd)

	// Assert
	rows := strings.Split(narrow.View().Content, "\n")
	requireScreen(t, narrow.View().Content, "Project")

	if len(rows) > 24 {
		t.Errorf("Settings drew %d rows on a 24-row terminal", len(rows))
	}

	for index, row := range rows {
		if width := lipgloss.Width(row); width > 80 {
			t.Errorf("row %d is %d cells wide on an 80-column terminal", index, width)
		}
	}

	if cmd != nil {
		t.Error("a resize over Settings' rows ran a command, want none: no form is open to fit")
	}
}
