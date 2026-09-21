// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// checksTitle titles the detail pane while the checks are listed.
const checksTitle = "Checks"

// errNoCheckPage reports a check the forge gave no page to open.
var errNoCheckPage = errors.New("this check reports no page to open")

// checkList lists the pull request's checks, so which one failed is plain, and
// opens the page of whichever is selected.
type checkList struct {
	marks    glyphs
	styles   styles
	checks   []forge.Check
	selected int
	outcome  string
	err      error
}

var _ overlay = checkList{}

// openChecks lists the checks reported on the pull request. Its caller offers it
// only when canOpenChecks reports there are checks and an opener for their pages.
func (m Model) openChecks() (Model, tea.Cmd) {
	m.overlay = checkList{marks: m.marks, styles: m.styles, checks: m.review.ci.Checks}

	return m, nil
}

// canOpenChecks reports that there are checks to list and an opener to reach
// their pages.
func (m Model) canOpenChecks() bool {
	return len(m.review.ci.Checks) > 0 && m.deps.OpenURL != nil
}

// view lists the checks, each by its state and name.
func (c checkList) view(_, rows int) (string, string) {
	lines := make([]string, 0, len(c.checks)+headerAndOutcomeRows)
	lines = append(lines, "Open a check's page with enter.", "")
	lines = append(lines, c.rows(rows-len(lines)-outcomeRows)...)
	lines = append(lines, c.outcomeLines()...)

	return checksTitle, strings.Join(lines, "\n")
}

// headerAndOutcomeRows is the two intro lines above the checks and the blank
// line plus outcome below them, so the slice is sized without a regrow.
const headerAndOutcomeRows = 4

// rows draws as many checks as fit, scrolled so the selection stays on screen.
func (c checkList) rows(space int) []string {
	first, last := window(c.selected, len(c.checks), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		check := c.checks[index]
		lines = append(lines, c.marks.marker(index == c.selected)+c.stateGlyph(check.State)+" "+check.Name)
	}

	return lines
}

// stateGlyph is how a check stands, by shape.
func (c checkList) stateGlyph(state forge.CIState) string {
	switch state {
	case forge.CIPassed:
		return c.marks.done
	case forge.CIFailed:
		return failedGlyph(c.styles, c.marks)
	case forge.CIRunning:
		return c.marks.inFlight
	case forge.CINone:
		return c.marks.unknown
	}

	return c.marks.unknown
}

// outcomeLines say how the last open went, if one was tried.
func (c checkList) outcomeLines() []string {
	switch {
	case c.err != nil:
		return []string{"", failedGlyph(c.styles, c.marks) + " " + c.err.Error()}
	case c.outcome != "":
		return []string{"", c.outcome}
	default:
		return nil
	}
}

// footer offers moving through the checks and opening one.
func (c checkList) footer(keys keyMap) []key.Binding {
	return keys.listKeys()
}

// handleKey answers a key while the checks are listed.
func (c checkList) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		c.selected = max(0, min(c.selected+1, len(c.checks)-1))
	case key.Matches(msg, m.keys.up):
		c.selected = max(0, c.selected-1)
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	}

	m.overlay = c

	return m, nil
}

// open opens the selected check's page, leaving the list up so another can be
// opened after it. The list is never empty: it opens only over reported checks.
func (c checkList) open(m Model) (Model, tea.Cmd) {
	check := c.checks[c.selected]
	if check.URL == "" {
		c.err, c.outcome = errNoCheckPage, ""
		m.overlay = c

		return m, nil
	}

	open := m.deps.OpenURL
	m.overlay = c

	return m, func() tea.Msg { return checkOpened{name: check.Name, err: open(check.URL)} }
}

// checkOpened reports how opening a check's page went.
type checkOpened struct {
	name string
	err  error
}

// apply records the outcome on the open list, or does nothing when it has since
// closed.
func (msg checkOpened) apply(m Model) (Model, tea.Cmd) {
	list, open := m.overlay.(checkList)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		list.err, list.outcome = msg.err, ""
	} else {
		list.err, list.outcome = nil, m.marks.done+" opened "+msg.name
	}

	m.overlay = list

	return m, nil
}
