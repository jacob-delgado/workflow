// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
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
// along.
func (msg branchCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return failedCreation(m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " created and switched to " + msg.name)
	reload := tea.Batch(m.loadBranch(), m.loadChanges())

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
// is a separate directory to move to.
func (msg worktreeCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return failedCreation(m, msg.err), nil
	}

	return m.closeOverlay().noticed(m.marks.done + " worktree for " + msg.name + " at " + msg.path), nil
}

// failedCreation keeps the branch creator open, carrying the reason it could not
// create what was asked, so it can be corrected and tried again.
func failedCreation(m Model, err error) Model {
	creator, open := m.overlay.(branchCreator)
	if open {
		creator.send = creator.send.failed(err)
		m.overlay = creator
	}

	return m
}
