// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestEnterOpensTheRowsWholeSectionAsOneForm(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), append(toRow(projectRow), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "> https://jira.example.com", "> PROJ")
}

func TestAddingAnEntryAsksItsNameAndValueTogether(t *testing.T) {
	t.Parallel()

	// Arrange
	adding := typing(t, newWorld().live(t, 120, 40), append(toRow(addViewRow), keyEnter)...)

	// Act
	view := typing(t, adding, pasted("Board"), keyEnter).View().Content

	// Assert
	requireScreen(t, view, "> Board", "Add a view")
}

func TestATypedSecretIsMaskedAsItIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, newWorld().live(t, 120, 40), append(toRow(tokenRow), keyEnter)...)

	// Act
	view := typing(t, opened, pasted(typedSecret)).View().Content

	// Assert
	requireScreen(t, view, strings.Repeat("*", len(typedSecret)))
	refuseScreen(t, view, typedSecret)
}

func TestSettingsKeepsEscInANarrowFooter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"on its rows":     {keys: toRow(projectRow), want: escCloses},
		"in a section's":  {keys: append(toRow(projectRow), keyEnter), want: escBacks},
		"adding an entry": {keys: append(toRow(addViewRow), keyEnter), want: escBacks},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newWorld().live(t, 80, 24), each.keys...).View().Content

			// Assert
			if footer := footerLine(view); !strings.Contains(footer, each.want) {
				t.Errorf("the footer = %q, want %q", footer, each.want)
			}
		})
	}
}

func TestASettingThatIsOnOrOffTurnsInItsSectionsForm(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(storeDisabledRow), keyEnter, keyRight, saveKey, saveKey)...)

	// Assert
	if !repo.settings.Store.Disabled {
		t.Errorf("store.disabled saved as false, want it turned on")
	}
}

func TestKeepingASectionRefusesACountThatIsNotOne(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40),
		append(toRow(subjectLimitRow), keyEnter, keyClearLine, pasted("x"), saveKey)...).View().Content

	// Assert
	requireScreen(t, view, "a count is a whole number", "> x")
}

func TestSettingsHandsASectionToThePromptsWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit section's four questions: the scope kept, the types and the
	// limit typed, the trailer kept.
	prompts := &answering{typed: "\nfeat,fix\n50\n\n"}
	repo := newWorld()
	deps := repo.deps()
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(repo.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	// Act
	asked := typing(t, drain(t, model, model.Init()), append(toRow(subjectLimitRow), keyEnter)...)

	// Assert
	if prompts.asked.Load() != 1 {
		t.Errorf("handed the prompts %d forms, want the commit section", prompts.asked.Load())
	}

	requireScreen(t, asked.View().Content, "feat, fix", "50")
}

func TestSettingsSaysWhyItsPromptsStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	deps := repo.deps()
	deps.Forms.Ask = func(_ *huh.Form, done func(error) tea.Msg) tea.Cmd {
		return func() tea.Msg { return done(errPromptsStopped) }
	}
	model := sized(t, tui.New(repo.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	// Act
	stopped := typing(t, drain(t, model, model.Init()), append(toRow(projectRow), keyEnter)...)

	// Assert
	requireScreen(t, stopped.View().Content, "the prompts stopped", "Base URL")
}

func TestASectionsFormFollowsTheTerminalWhenItIsResized(t *testing.T) {
	t.Parallel()

	// Arrange
	wide := typing(t, newWorld().live(t, 160, 40), append(toRow(projectRow), keyEnter)...)

	// Act
	narrowed, cmd := wide.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	narrow := drain(t, concrete(t, narrowed), cmd)

	// Assert
	// The form is still open after the resize, and fits the narrower pane.
	requireScreen(t, narrow.View().Content, "> PROJ")

	for line := range strings.SplitSeq(plain(narrow.View().Content), "\n") {
		if strings.Contains(line, "Review status") && strings.Contains(line, "…") {
			t.Errorf("the form is still as wide as the terminal was, cut at the pane's edge:\n%s", line)
		}
	}
}
