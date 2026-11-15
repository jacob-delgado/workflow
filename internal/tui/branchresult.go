// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// creating puts the creator back with its create out, and is the command
// that creates the branch.
func (c branchCreator) creating(m Model, name string) (Model, tea.Cmd) {
	m = m.withAnswered(c)

	return m, c.createCommand(m, name)
}

// createCommand is the command that creates the branch: in a worktree beside the
// repository, or in place, switching to it. Its answer carries the round the
// creator opened in.
func (c branchCreator) createCommand(m Model, name string) tea.Cmd {
	base, round := c.base, c.opened
	issue, forIssue := c.issue, c.forIssue

	if c.worktree {
		ctx, add := m.session, m.deps.Git.CreateWorktree

		return func() tea.Msg {
			path, err := add(ctx, name, base)

			return worktreeCreated{name: name, path: path, issue: issue, forIssue: forIssue, round: round, err: err}
		}
	}

	ctx, createBranch := m.session, m.deps.Git.CreateBranch

	return func() tea.Msg {
		return branchCreated{
			name: name, issue: issue, forIssue: forIssue, round: round, err: createBranch(ctx, name, base),
		}
	}
}

// fetched reports how the fetch before branching went, to the creator that
// sent it in round.
type fetched struct {
	name  string
	round int
	err   error
}

var _ applier = fetched{}

// apply creates the branch once the fetch succeeds, or keeps the creator open,
// refused at its fetch, offering to branch from what is already there when
// the fetch fails.
// A late answer, its creator no longer the one open, changes nothing, success
// or not; none arrives, since the creator takes no key but ctrl+c, which
// quits, while its write is out.
func (msg fetched) apply(m Model) (Model, tea.Cmd) {
	m, creator, done := answered[branchCreator](m, msg.round, msg.err)
	if !done {
		return m, nil
	}

	return creator.sentAt(stepCreate).creating(m, msg.name)
}

// branchCreated reports how creating a branch went, to the creator that sent
// it in round, and the issue it was for so its status can be offered once it
// exists.
type branchCreated struct {
	name     string
	issue    jira.Issue
	forIssue bool
	round    int
	err      error
}

var _ applier = branchCreated{}

// apply switches the panes to the new branch, or keeps the creator open with
// git's reason. A branch for an issue not yet started then offers to move it
// along; a branch for any issue offers to start its task once nothing else is
// being asked.
// A late answer, its creator no longer the one open, changes nothing, success
// or not; none arrives, since the creator takes no key but ctrl+c, which
// quits, while its write is out.
func (msg branchCreated) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[branchCreator](m, msg.round, msg.err)
	if !done {
		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " created and switched to " + msg.name)
	m, status := m.readingChanges()
	m, read := m.readingBranch()
	m, branches := m.readingIssueBranches()
	reload := tea.Batch(read, status, branches)

	// Set before the status picker opens, so the offer waits behind it.
	if msg.forIssue {
		issue := msg.issue
		m.followUp = offeredOnOpening(func(m Model) waitingOffer { return m.offerStart(issue) })
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

	picker, offer := m.pickStatusFor(issue, statusOffer{kind: offerInProgress})

	return picker, tea.Batch(reload, offer)
}

// worktreeCreated reports how creating a worktree went, and the issue it was
// for so its status can be offered once it exists, as branchCreated does.
type worktreeCreated struct {
	name, path string
	issue      jira.Issue
	forIssue   bool
	round      int
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
// A late answer, its creator no longer the one open, changes nothing, success
// or not; none arrives, since the creator takes no key but ctrl+c, which
// quits, while its write is out.
func (msg worktreeCreated) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[branchCreator](m, msg.round, msg.err)
	if !done {
		return m, nil
	}

	path := msg.path
	m = m.closeOverlay().noticed(m.marks.done + " created worktree for " + sanitize.Line(msg.name) + " at " +
		shownDir(m.deps, path))
	m.followUp = func(m Model) (Model, tea.Cmd) {
		offer := m.switchLook(path)
		offer.title, offer.leave = "Switch to the new worktree", escSkip

		m.overlay = offer

		return m, nil
	}

	m, reread := m.readingRepositoriesAgain()
	m, branches := m.readingIssueBranches()

	return m.offeringInProgress(msg.issue, msg.forIssue, tea.Batch(branches, reread))
}
