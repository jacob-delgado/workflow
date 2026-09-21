// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// switchTitle titles the detail pane while the task switcher is open.
const switchTitle = "Switch task"

// errDirtyTree refuses a switch that would carry uncommitted work onto another
// branch. Stashing is left to the person, so the reason says what to do rather
// than doing it.
var errDirtyTree = errors.New("uncommitted changes — commit or stash them before switching tasks")

// taskBranch is a local branch that names an issue, offered to switch to.
type taskBranch struct {
	name     string
	issueKey jira.Key
	summary  string
}

// branchesListed carries the local branches back into the update loop.
type branchesListed struct {
	found []string
	err   error
}

// apply records the branches on the open switcher, or does nothing when it has
// since closed.
func (msg branchesListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(branchPicker)
	if !open {
		return m, nil
	}

	picker.branches, picker.listErr, picker.settled = m.taskBranches(msg.found), msg.err, true
	m.overlay = picker

	return m, nil
}

// taskBranches keeps the branches that name an issue, other than the one
// checked out, and names each by its issue.
func (m Model) taskBranches(names []string) []taskBranch {
	current := m.branch.branch.Name

	var branches []taskBranch

	for _, name := range names {
		key, named := convention.IssueKey(name, m.cfg.Jira.Project)
		if !named || name == current {
			continue
		}

		issue, _ := m.issues.find(jira.Key(key))
		branches = append(branches, taskBranch{name: name, issueKey: jira.Key(key), summary: issue.Summary})
	}

	return branches
}

// branchPicker lists the issue branches to switch to, and how a switch is going.
type branchPicker struct {
	marks     glyphs
	styles    styles
	branches  []taskBranch
	listErr   error
	switchErr error
	settled   bool
	sending   bool
	selected  int
}

var _ overlay = branchPicker{}

// openBranchPicker opens the task switcher and starts listing the local
// branches.
func (m Model) openBranchPicker() (Model, tea.Cmd) {
	m.overlay = branchPicker{marks: m.marks, styles: m.styles}
	list := m.deps.Git.Branches

	return m, func() tea.Msg {
		found, err := list()

		return branchesListed{found: found, err: err}
	}
}

// view draws the switcher in as many rows as fit.
func (p branchPicker) view(_, rows int) (string, string) {
	lines := []string{"Switch to another task's branch.", ""}

	switch {
	case !p.settled:
		lines = append(lines, "loading branches"+p.marks.ellipsis)
	case p.listErr != nil:
		lines = append(lines, failedGlyph(p.styles, p.marks)+" "+p.listErr.Error())
	case len(p.branches) == 0:
		lines = append(lines, "No other task branch to switch to.")
	default:
		lines = append(lines, p.rows(rows-len(lines)-outcomeRows)...)
		lines = append(lines, p.outcome()...)
	}

	return switchTitle, strings.Join(lines, "\n")
}

// rows draws as many branches as fit, scrolled so the selection stays on screen.
func (p branchPicker) rows(space int) []string {
	first, last := window(p.selected, len(p.branches), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		lines = append(lines, p.marks.marker(index == p.selected)+p.label(p.branches[index]))
	}

	return lines
}

// label names a branch by its issue, with the summary when the issue is one of
// yours, and the branch name so there is no doubt which will be checked out.
func (p branchPicker) label(branch taskBranch) string {
	named := string(branch.issueKey)
	if branch.summary != "" {
		named += " " + branch.summary
	}

	return named + p.marks.separator + branch.name
}

// outcome says how switching is going, if it was tried.
func (p branchPicker) outcome() []string {
	switch {
	case p.sending:
		return []string{"", "switching" + p.marks.ellipsis}
	case p.switchErr != nil:
		return []string{"", failedGlyph(p.styles, p.marks) + " " + p.switchErr.Error()}
	default:
		return nil
	}
}

// footer offers what works in the switcher now. A switch in flight only offers
// quitting, so nothing interrupts it.
func (p branchPicker) footer(keys keyMap) []key.Binding {
	if p.sending {
		return []key.Binding{keys.interrupt}
	}

	return keys.listKeys()
}

// handleKey answers a key while the switcher has the keyboard.
func (p branchPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		p.selected = max(0, min(p.selected+1, len(p.branches)-1))
	case key.Matches(msg, m.keys.up):
		p.selected = max(0, p.selected-1)
	case key.Matches(msg, m.keys.confirm):
		return p.choose(m)
	}

	m.overlay = p

	return m, nil
}

// chosen is the selected branch, if there is one to switch to.
func (p branchPicker) chosen() (taskBranch, bool) {
	if len(p.branches) == 0 {
		return taskBranch{}, false
	}

	return p.branches[p.selected], true
}

// choose switches to the selected branch, refusing a dirty tree with the reason
// rather than carrying uncommitted work across.
func (p branchPicker) choose(m Model) (Model, tea.Cmd) {
	branch, ok := p.chosen()
	if !ok {
		return m, nil
	}

	if m.changes.dirty() {
		p.switchErr = errDirtyTree
		m.overlay = p

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would switch to " + branch.name), nil
	}

	p.sending, p.switchErr = true, nil
	m.overlay = p
	checkout := m.deps.Git.Checkout

	return m, func() tea.Msg { return taskSwitched{name: branch.name, err: checkout(branch.name)} }
}

// taskSwitched reports how switching to a branch went.
type taskSwitched struct {
	name string
	err  error
}

// apply reloads the panes for the branch switched to, or keeps the switcher open
// with git's reason.
func (msg taskSwitched) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		picker, open := m.overlay.(branchPicker)
		if open {
			picker.sending, picker.switchErr = false, msg.err
			m.overlay = picker
		}

		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " switched to " + msg.name)

	return m, tea.Batch(m.loadBranch(), m.loadChanges())
}
