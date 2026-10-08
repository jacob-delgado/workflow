// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleIssuesKey answers the Issues pane's own keys: moving through the
// list, and what its footer offers.
func (m Model) handleIssuesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.keys.up, m.keys.down) {
		return m.moveIssue(m.keys.stepOf(msg))
	}

	return m.answer(m.issuesOffers(), msg)
}

// beginIssueFilter starts typing a filter over the list.
func (m Model) beginIssueFilter() (Model, tea.Cmd) {
	m.issues = m.issues.beginFilter()

	return m, nil
}

// readSelectedIssue reads the selected issue in full, in the collapsed layout
// where it takes the list's place.
func (m Model) readSelectedIssue() (Model, tea.Cmd) {
	m.issues.viewing = true

	return m.loadDetail()
}

// backToIssueList returns the collapsed layout from the issue read in full to
// the list.
func (m Model) backToIssueList() (Model, tea.Cmd) {
	m.issues.viewing = false

	return m, nil
}

// handleIssueFilterKey builds the filter from keystrokes: printable runes extend
// it, enter keeps it applied so the narrowed list can be navigated, and esc
// cancels it and restores the whole list. The arrow keys still move the
// selection, so the list can be filtered and scanned at once.
func (m Model) handleIssueFilterKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.issues = m.issues.clearFilter()

		return m.loadDetail()
	case tea.KeyEnter:
		m.issues = m.issues.confirmFilter()

		return m, nil
	case tea.KeyDown:
		return m.moveIssue(1)
	case tea.KeyUp:
		return m.moveIssue(-1)
	case tea.KeyBackspace:
		m.issues = m.issues.trimFilter()

		return m.loadDetail()
	default:
		return m.extendFilterWith(msg)
	}
}

// filterKeys is the footer while the filter is being typed: the two keys that
// close it. Every other key types into it or moves the selection. They are
// enter and esc themselves, which handleIssueFilterKey reads whatever ui.keys
// binds apply and close to, so a printable key moved onto either still types.
func (Model) filterKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep search")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
	}
}

// extendFilterWith adds a key's text to the filter when it types something,
// and does nothing for a key that does not.
func (m Model) extendFilterWith(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	return m.extendFilterBy(typedText(msg))
}

// extendFilterBy adds text to the filter, typed or pasted, and does nothing
// when there is none.
func (m Model) extendFilterBy(text string) (Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}

	m.issues = m.issues.extendFilter(text)

	return m.loadDetail()
}

// typedText is the text a key types, treating the space key as a space: ""
// for a key that types nothing.
func typedText(msg tea.KeyPressMsg) string {
	if msg.Text == "" && msg.Code == tea.KeySpace {
		return " "
	}

	return msg.Text
}

// refreshIssues reads the list again, and the issue shown in full whether or not
// it changed, so r retries a detail load that failed. An issue the selection has
// only just reached is left to the read its rest will start.
func (m Model) refreshIssues() (Model, tea.Cmd) {
	m = m.searching()

	selected, ok := m.issues.current()
	if !ok {
		return m, m.relistIssues()
	}

	m, detail := m.reloadDetail(selected.Key)

	return m, tea.Batch(m.relistIssues(), detail)
}
