// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
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

// issuesOffers are the Issues pane's keys: in the collapsed layout, reading
// the selected issue or going back to the list; the verbs its seams can carry
// out on the selected issue and its links; then the list's own keys. With no
// issue selected, or the first run's setup offered, a branch can still be
// started from nothing.
func (m Model) issuesOffers() []offer {
	selected, ok := m.issues.current()

	switch {
	case m.setupShown():
		setup := offer{binding: relabel(m.keys.confirm, "set up"), can: true, act: m.openSetup}

		return slices.Concat([]offer{setup, m.newBranchOffer()}, m.issueListOffers())
	case !ok:
		return slices.Concat([]offer{m.newBranchOffer()}, m.issueListOffers())
	default:
		return slices.Concat(m.readingOffers(), m.issueVerbOffers(selected), m.linkOffers(m.issueURL()),
			m.issueListOffers())
	}
}

// issuesKeys is the Issues pane's footer: the offers that act right now.
func (m Model) issuesKeys() []key.Binding {
	return liveKeys(m.issuesOffers())
}

// issueVerbOffers are the verbs for the selected issue whose seams are wired:
// status, comment, then the branch, which moves the loop on, ahead of assign,
// log work and tracking it in Taskwarrior — or going to its task — since a
// narrow footer drops the last verbs first.
func (m Model) issueVerbOffers(selected jira.Issue) []offer {
	return []offer{
		{binding: m.keys.changeStatus, can: m.deps.Jira.Transitions != nil, act: m.openStatusPicker},
		{binding: m.keys.comment, can: m.deps.Jira.Comment != nil, act: m.startComment},
		{binding: m.keys.startWork, can: m.canCreateBranch(), act: m.openBranchCreator},
		{binding: m.keys.assign, can: m.deps.Jira.Assign != nil, act: m.openAssign},
		{
			binding: m.keys.logWork, can: m.deps.Jira.AddWorklog != nil && !isForgeKey(selected.Key),
			act: m.openLogWork,
		},
		{binding: m.trackKey(selected.Key), can: m.canTrack(selected.Key), act: m.trackSelectedIssue},
	}
}

// newBranchOffer is starting a branch named for no issue, which the branch key
// does on the Issues pane when none is selected.
func (m Model) newBranchOffer() offer {
	return offer{binding: relabel(m.keys.startWork, "new branch"), can: m.canCreateBranch(), act: m.openBranchCreator}
}

// readingOffers are, in the collapsed layout where the list and the issue take
// turns, reading the selected issue in full or going back to the list.
func (m Model) readingOffers() []offer {
	switch {
	case !m.shape().Collapsed():
		return nil
	case m.issues.viewing:
		return []offer{{binding: relabel(m.keys.closeOverlay, escBack+" to list"), can: true, act: m.backToIssueList}}
	default:
		return []offer{{binding: relabel(m.keys.confirm, "read issue"), can: true, act: m.readSelectedIssue}}
	}
}

// issueListOffers are the keys that manage the list itself: filter it, narrow
// it to places, switch view, read the next page, search again.
func (m Model) issueListOffers() []offer {
	filterable := m.issues.filterable()

	return []offer{
		{binding: m.keys.searchIssues, can: filterable, act: m.beginIssueFilter},
		{binding: m.keys.filterIssues, can: filterable, act: m.openPlacePicker},
		{binding: m.keys.nextView, can: len(m.views) > 1, act: m.nextIssueView},
		{binding: m.keys.loadMore, can: m.issues.hasMore(), act: m.loadMoreIssues},
		{binding: m.keys.refresh, can: true, act: func() (Model, tea.Cmd) { return m.refreshPane(paneIssues) }},
	}
}
