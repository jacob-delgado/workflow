// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleIssuesKey answers the Issues pane's own keys.
func (m Model) handleIssuesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.down):
		return m.moveIssue(1)
	case key.Matches(msg, m.keys.up):
		return m.moveIssue(-1)
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(m.issueURL())
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(m.issueURL())
	case key.Matches(msg, m.keys.filter) && m.issues.filterable():
		m.issues = m.issues.beginFilter()

		return m, nil
	}

	if next, cmd, handled := m.handleIssueVerbKey(msg); handled {
		return next, cmd
	}

	return m.handleIssueListKey(msg)
}

// handleIssueVerbKey answers the keys that act on the selected issue — change
// its status, comment, assign, log work, or branch for it — reporting whether
// it claimed the key, so the caller can fall through to the list keys.
func (m Model) handleIssueVerbKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	var act func() (Model, tea.Cmd)

	switch {
	case key.Matches(msg, m.keys.changeStatus):
		act = m.openStatusPicker
	case key.Matches(msg, m.keys.comment):
		act = m.startComment
	case key.Matches(msg, m.keys.assign):
		act = m.openAssign
	case key.Matches(msg, m.keys.logWork):
		act = m.openLogWork
	case key.Matches(msg, m.keys.branchForIssue):
		act = m.openBranchCreator
	default:
		return m, nil, false
	}

	next, cmd := act()

	return next, cmd, true
}

// handleIssueListKey answers the keys that manage the list itself — switching
// view, loading the next page, refreshing — before falling through to the keys
// that read an issue in full.
func (m Model) handleIssueListKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.nextView):
		return m.nextIssueView()
	case key.Matches(msg, m.keys.loadMore):
		return m.loadMoreIssues()
	case key.Matches(msg, m.keys.refresh):
		return m.refreshIssues()
	default:
		return m.handleIssueViewingKey(msg)
	}
}

// handleIssueViewingKey answers the keys that, in the collapsed layout, read the
// selected issue in full or return to scanning the list.
func (m Model) handleIssueViewingKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		m.issues.viewing = true

		return m.loadDetail()
	case key.Matches(msg, m.keys.closeOverlay):
		m.issues.viewing = false

		return m, nil
	default:
		return m, nil
	}
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
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep filter")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
	}
}

// extendFilterWith adds a key's text to the filter when it types something,
// treating the space key as a space, and does nothing for a key that does not.
func (m Model) extendFilterWith(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	text := msg.Text
	if text == "" && msg.Code == tea.KeySpace {
		text = " "
	}

	if text == "" {
		return m, nil
	}

	m.issues = m.issues.extendFilter(text)

	return m.loadDetail()
}

// refreshIssues reads the list again, and the issue shown in full whether or not
// it changed, so r retries a detail load that failed. An issue the selection has
// only just reached is left to the read its rest will start.
func (m Model) refreshIssues() (Model, tea.Cmd) {
	m.issues.loading = true

	selected, ok := m.issues.current()
	if !ok {
		return m, m.searchIssues()
	}

	m, detail := m.reloadDetail(selected.Key)

	return m, tea.Batch(m.searchIssues(), detail)
}
