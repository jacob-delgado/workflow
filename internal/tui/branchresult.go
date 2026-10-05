// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// createCommand is the command that creates the branch: in a worktree beside the
// repository, or in place, switching to it.
func (c branchCreator) createCommand(m Model, name string) tea.Cmd {
	base := c.base

	if c.worktree {
		add := m.deps.Git.CreateWorktree

		return func() tea.Msg {
			path, err := add(name, base)

			return worktreeCreated{name: name, path: path, err: err}
		}
	}

	createBranch := m.deps.Git.CreateBranch
	issue, forIssue := c.issue, c.forIssue

	return func() tea.Msg {
		return branchCreated{name: name, issue: issue, forIssue: forIssue, err: createBranch(name, base)}
	}
}

// fetched reports how the fetch before branching went.
type fetched struct {
	name string
	err  error
}

// apply creates the branch once the fetch succeeds, or keeps the creator open
// offering to branch from what is already there when the fetch fails.
func (msg fetched) apply(m Model) (Model, tea.Cmd) {
	creator, open := m.overlay.(branchCreator)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		creator.send.sending, creator.fetchProblem, creator.skipFetch = false, msg.err, true
		m.overlay = creator

		return m, nil
	}

	return m, creator.createCommand(m, msg.name)
}

// branchCreated reports how creating a branch went, and the issue it was for so
// its status can be offered once it exists.
type branchCreated struct {
	name     string
	issue    jira.Issue
	forIssue bool
	err      error
}

// apply switches the panes to the new branch, or keeps the creator open with
// git's reason. A branch for an issue not yet started then offers to move it
// along; a branch for any issue offers to start its task once nothing else is
// being asked.
func (msg branchCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchCreator](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " created and switched to " + msg.name)
	reload := tea.Batch(m.loadBranch(), m.loadChanges(), m.listIssueBranches())

	// Set before the status picker opens, so the offer waits behind it.
	if msg.forIssue {
		m.followUp = m.offerStart(msg.issue)
	}

	if !msg.forIssue || msg.issue.StatusCategory != jira.CategoryNew {
		return m, reload
	}

	// Offer the status change, pre-selected on the first in-progress transition;
	// the developer confirms it or backs out. Never applied for them.
	picker, offer := m.pickStatusFor(msg.issue, statusOffer{inProgress: true})

	return picker, tea.Batch(reload, offer)
}

// worktreeCreated reports how creating a worktree went.
type worktreeCreated struct {
	name, path string
	err        error
}

// apply says where the worktree is, or keeps the creator open with git's reason.
// The panes do not reload: the current checkout is untouched, and the worktree
// is a separate directory to move to, which is offered once nothing else is
// being asked. Its branch is new, though, so the issues are marked in flight
// again, and a Repositories pane already read lists the worktree.
func (msg worktreeCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchCreator](m, msg.err), nil
	}

	offer := worktreeOffer{dir: msg.path, shown: m.shownDir(msg.path)}
	m = m.closeOverlay().noticed(m.marks.done + " worktree for " + sanitize.Line(msg.name) + " at " + offer.shown)
	m.followUp = func(m Model) (Model, tea.Cmd) {
		m.overlay = offer

		return m, nil
	}

	reload := m.listIssueBranches()
	if m.repositories.read {
		reload = tea.Batch(reload, m.loadRepositories())
	}

	return m, reload
}

// worktreeOffer offers to switch to a worktree just made.
type worktreeOffer struct {
	dir, shown string
}

var _ overlay = worktreeOffer{}

// view says where the worktree is and what switching does.
func (o worktreeOffer) view(width, _ int) (string, string) {
	return "Switch to the new worktree",
		wrap("Switch to "+o.shown+"? workflow opens again there, on the worktree's branch.", width)
}

// footer offers switching or staying.
func (worktreeOffer) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "switch"), relabel(keys.closeOverlay, "stay")}
}

// handleKey switches, asking first as any switch does when something would be
// lost, or stays.
func (o worktreeOffer) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return m.closeOverlay().leaveFor(o.dir)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	default:
		return m, nil
	}
}

// failed is the creator kept open with the reason it could not create what was
// asked, so it can be corrected and tried again.
func (c branchCreator) failed(err error) branchCreator {
	c.send = c.send.failed(err)

	return c
}
