// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
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

// commentsShownRow is the Terminal section's last row, the comments an
// issue's detail shows.
const commentsShownRow = asciiRow + 6

// notACount is why a count that is not one is refused, behind the mark of a
// refusal.
const notACount = "✗ a count is a whole number, 0 or more; 0 keeps the default"

func TestKeepingASectionRefusesACountThatIsNotOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		row, width, height int
		keep               string
	}{
		"kept":                              {row: subjectLimitRow, width: 120, height: 40, keep: saveKey},
		"kept at the smallest size":         {row: subjectLimitRow, width: 80, height: 24, keep: saveKey},
		"gone on from at the smallest size": {row: subjectLimitRow, width: 80, height: 24, keep: keyEnter},
		"kept from the terminal's last":     {row: commentsShownRow, width: 80, height: 24, keep: saveKey},
		"kept from the terminal's at 100":   {row: commentsShownRow, width: 100, height: 30, keep: saveKey},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newWorld().live(t, each.width, each.height),
				append(toRow(each.row), keyEnter, "ctrl+u", pasted("x"), each.keep)...).View().Content

			// Assert
			requireScreen(t, view, "> x")
			requireWhole(t, view, notACount)
		})
	}
}

func TestARefusalTakenAwayGivesTheFormBackItsRows(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := append(toRow(subjectLimitRow), keyEnter, keyClearLine, pasted("x"))
	unrefused := typing(t, newWorld().live(t, 80, 24), append(editing, keyBackspace)...).View().Content

	// Act
	view := typing(t, newWorld().live(t, 80, 24), append(editing, saveKey, keyBackspace)...).View().Content

	// Assert
	if plain(view) != plain(unrefused) {
		t.Errorf("once the refusal is gone the form is not drawn as it was before it:\n%s\nwant:\n%s",
			plain(view), plain(unrefused))
	}
}

// requireWhole fails the test unless the screen shows text whole, though the
// pane wraps it: read from the column its first words begin at, down every
// line from theirs.
func requireWhole(t *testing.T, view, text string) {
	t.Helper()

	const openingWords = 3

	lines := strings.Split(plain(view), "\n")
	opening := strings.Join(strings.Fields(text)[:openingWords], " ")

	for index, line := range lines {
		before, _, found := strings.Cut(line, opening)
		if !found {
			continue
		}

		column := ansi.StringWidth(before)
		read := make([]string, 0, len(lines)-index)

		for _, below := range lines[index:] {
			read = append(read, strings.TrimRight(ansi.Cut(below, column, ansi.StringWidth(below)), "┃│ "))
		}

		if strings.Contains(strings.Join(strings.Fields(strings.Join(read, " ")), " "), text) {
			return
		}
	}

	t.Errorf("the screen does not show %q whole:\n%s", text, plain(view))
}

func TestKeepingASectionUntouchedLeavesAChoiceItsOptionsSpellOtherwise(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		row    int
		stored func(*config.Config)
	}{
		"the service named slack": {
			row:    channelRow,
			stored: func(cfg *config.Config) { cfg.Messaging.Kind = config.KindSlack },
		},
		"a title from the commit": {
			row:    titleSourceRow,
			stored: func(cfg *config.Config) { cfg.PullRequest.TitleSource = string(convention.TitleFromCommit) },
		},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			each.stored(&repo.settings)

			// Act
			view := typing(t, repo.live(t, 120, 40), append(toRow(each.row), keyEnter, saveKey)...).View().Content

			// Assert
			refuseScreen(t, view, "(edited)")
		})
	}
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
	// The form is still open after the resize, and fits the narrower pane:
	// the review status's hint, on one line in the wide pane, wraps onto two.
	requireScreen(t, narrow.View().Content, "> PROJ")

	if line := lineShowing(t, narrow.View().Content, "Empty makes no offer."); strings.Contains(line, "The status") {
		t.Errorf("the review status's hint is as wide as before the resize: %s", line)
	}
}
