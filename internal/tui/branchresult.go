// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// createCommand is the command that creates the branch: in a worktree beside the
// repository, or in place, switching to it.
func (c branchCreator) createCommand(m Model, name string) tea.Cmd {
	base := c.base
	issue, forIssue := c.issue, c.forIssue

	if c.worktree {
		add := m.deps.Git.CreateWorktree

		return func() tea.Msg {
			path, err := add(name, base)

			return worktreeCreated{name: name, path: path, issue: issue, forIssue: forIssue, err: err}
		}
	}

	createBranch := m.deps.Git.CreateBranch

	return func() tea.Msg {
		return branchCreated{name: name, issue: issue, forIssue: forIssue, err: createBranch(name, base)}
	}
}

// fetched reports how the fetch before branching went.
type fetched struct {
	name string
	err  error
}

var _ applier = fetched{}

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

var _ applier = branchCreated{}

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

	return m.offeringInProgress(msg.issue, msg.forIssue, reload)
}

// offeringInProgress follows reload with the offer to move an issue the new
// branch is for along, when it is not yet started: the status picker,
// pre-selected on the first in-progress transition, which the developer
// confirms or backs out of. It is never applied for them.
func (m Model) offeringInProgress(issue jira.Issue, forIssue bool, reload tea.Cmd) (Model, tea.Cmd) {
	if !forIssue || issue.StatusCategory != jira.CategoryNew {
		return m, reload
	}

	picker, offer := m.pickStatusFor(issue, statusOffer{inProgress: true})

	return picker, tea.Batch(reload, offer)
}

// worktreeCreated reports how creating a worktree went, and the issue it was
// for so its status can be offered once it exists, as branchCreated does.
type worktreeCreated struct {
	name, path string
	issue      jira.Issue
	forIssue   bool
	err        error
}

var _ applier = worktreeCreated{}

// apply says where the worktree is, or keeps the creator open with git's reason.
// The panes do not reload: the current checkout is untouched, and the worktree
// is a separate directory to move to, which is offered once nothing else is
// being asked — after the offer to move an issue not yet started along, since
// its work has as surely begun. Its branch is new, though, so the issues are
// marked in flight again, and a Repositories pane already read lists the
// worktree.
func (msg worktreeCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchCreator](m, msg.err), nil
	}

	path := msg.path
	m = m.closeOverlay().noticed(m.marks.done + " created worktree for " + sanitize.Line(msg.name) + " at " +
		m.shownDir(path))
	m.followUp = func(m Model) (Model, tea.Cmd) {
		offer := m.switchLook(path)
		offer.title, offer.leave = "Switch to the new worktree", escSkip

		m.overlay = offer

		return m, nil
	}

	reload := m.listIssueBranches()
	if m.repositories.read {
		reload = tea.Batch(reload, m.loadRepositories())
	}

	return m.offeringInProgress(msg.issue, msg.forIssue, reload)
}

// failed is the creator kept open with the reason it could not create what was
// asked, so it can be corrected and tried again.
func (c branchCreator) failed(err error) branchCreator {
	c.send = c.send.failed(err)

	return c
}
