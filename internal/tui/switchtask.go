// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// switchTitle titles the detail pane while the task switcher is open.
const switchTitle = "Switch branch"

// errBranchHeldByGone is a branch a worktree whose directory is gone still holds:
// git will not check it out anywhere until that worktree is pruned.
var errBranchHeldByGone = errors.New("its worktree is gone; git worktree prune frees the branch to switch to")

// taskBranch is a branch that names an issue, offered to switch to: a local
// one, or one only the remote has, which switching to creates here. worktree
// is the directory of another worktree that has it checked out, where
// switching to it goes instead, since git will not check it out twice.
type taskBranch struct {
	name     string
	issueKey jira.Key
	summary  string
	remote   bool
	worktree string
	// worktreeShown is worktree written from your home, neutralized.
	worktreeShown string
	// worktreeGone reports the worktree's directory gone.
	worktreeGone bool
}

// branchesListed carries the local and remote branches back into the update
// loop, with which of the issues they name are yours: nil, with no tracker to
// ask or when asking it failed, counts every one. notAsked is why the tracker
// could not be asked, kept apart from err, a listing git could not make.
type branchesListed struct {
	local, remote []string
	mine          map[jira.Key]bool
	// links are the branches linked to an issue by hand, by name.
	links map[string]string
	// worktrees are the other worktrees, by the branch each has checked out.
	worktrees map[string]gitrepo.Worktree
	notAsked  error
	err       error
	// opened is the round the switcher that asked opened in.
	opened int
}

var _ applier = branchesListed{}

// yours reports whether key names one of your issues.
func (msg branchesListed) yours(key jira.Key) bool {
	return msg.mine == nil || msg.mine[key]
}

// candidates is every branch listed, the local ones first and then those only
// the remote has, marked so. A name on both is the local branch.
func (msg branchesListed) candidates() []taskBranch {
	branches := make([]taskBranch, 0, len(msg.local)+len(msg.remote))

	for _, name := range msg.local {
		branches = append(branches, taskBranch{name: name})
	}

	for _, name := range msg.remote {
		if !slices.Contains(msg.local, name) {
			branches = append(branches, taskBranch{name: name, remote: true})
		}
	}

	return branches
}

// apply records the branches on the switcher that asked, or does nothing when
// it has since closed, even when another has opened: that one asks for its
// own, and a choice made from these would be listed over.
func (msg branchesListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := answerFor[branchPicker](m, msg.opened)
	if !open {
		return m, nil
	}

	// The switcher lists its branches once, as it opens, before it could
	// begin a round: the listing is round 0's.
	found := switchable{branches: m.taskBranches(msg), notAsked: msg.notAsked}
	picker.listing, _ = picker.listing.Answered(0, found, msg.err)

	return m.withAnswered(picker), nil
}

// switchable is the branches the switcher offers, with why the tracker could
// not be asked which issues are yours, when it could not, which leaves every
// issue's branch among them.
type switchable struct {
	branches []taskBranch
	notAsked error
}

// taskBranches keeps the branches that name one of your issues, other than the
// one checked out, and names each by its issue.
func (m Model) taskBranches(listed branchesListed) []taskBranch {
	current := m.branch.checkedOut().Name

	var branches []taskBranch

	for _, branch := range listed.candidates() {
		key, named := loop.NamedIssue(branch.name, listed.links, m.cfg.Jira.Project)
		if !named || branch.name == current || !listed.yours(jira.Key(key.Key)) {
			continue
		}

		issue, _ := m.issues.find(jira.Key(key.Key))
		branch.issueKey, branch.summary = jira.Key(key.Key), issue.Summary

		if worktree, elsewhere := listed.worktrees[branch.name]; elsewhere {
			branch.worktree, branch.worktreeShown = worktree.Dir, shownDir(m.deps, worktree.Dir)
			branch.worktreeGone = worktree.Missing
		}

		branches = append(branches, branch)
	}

	return branches
}

// branchPicker lists the issue branches to switch to, and how a switch is going.
type branchPicker struct {
	// listing is the branches offered, read once as the switcher opens;
	// selected is which of them is chosen.
	listing  latest.Read[switchable]
	selected int
	// send is the switch: the working tree read, then the checkout, one
	// after the other as one write.
	send send.State
	// problem is why the branch chosen cannot be switched to, found before
	// anything is sent.
	problem error
	// opened is the round it opened in, which its listing and its switch
	// carry back.
	opened int
}

var (
	_ sender[branchPicker] = branchPicker{}
	_ steppable            = branchPicker{}
)

// openBranchPicker opens the task switcher and starts listing the branches.
func (m Model) openBranchPicker() (Model, tea.Cmd) {
	m, opened := m.nextRound()
	m.overlay = branchPicker{opened: opened}
	lister := branchLister{
		local: m.deps.Git.Branches, remote: m.deps.Git.RemoteBranches, links: m.deps.Git.IssueLinks,
		search: m.deps.Jira.SearchLenient, project: m.cfg.Jira.Project,
		worktrees: m.deps.Repositories.Worktrees, here: m.deps.Repositories.Here.Root,
	}
	ctx := m.session

	return m, func() tea.Msg {
		listed := lister.list(ctx)
		listed.worktrees, listed.opened = lister.otherWorktrees(ctx), opened

		return listed
	}
}

// view draws the switcher in as many rows as fit: reading the branches, why
// they could not be listed, or them.
func (p branchPicker) view(kit renderKit, width, rows int) (string, string) {
	header := []string{"Switch to another task's branch.", ""}
	space := rows - len(header)
	failed := func() []string { return []string{kit.failureLine(p.listing.Err())} }

	body := drawRead(p.listing, drawnRead[[]string]{
		reading:    func() []string { return []string{"reading the branches" + kit.marks.ellipsis} },
		shown:      func() []string { return p.listed(kit, width, space) },
		failed:     failed,
		failedOver: failed,
	})

	return switchTitle, strings.Join(slices.Concat(header, body), "\n")
}

// listed draws the branches listed in space rows, width columns wide, under
// the note saying the tracker could not be asked which are yours when it
// could not: that there is none to switch to, or them, with how switching to
// the chosen one is going.
func (p branchPicker) listed(kit renderKit, width, space int) []string {
	found, _ := p.listing.Value()
	lines := notAskedNote(kit, found.notAsked)
	branches := p.branches()

	if len(branches.items) == 0 {
		return append(lines, "No other task branch to switch to.")
	}

	outcome := p.outcome(kit, width)
	lines = append(lines, branches.rows(kit.marks, space-len(lines)-max(outcomeRows, len(outcome)),
		func(branch taskBranch) string { return branchLabel(kit.marks, branch) })...)

	return append(lines, outcome...)
}

// branches is the branches listed, none until they are, with the chosen one.
func (p branchPicker) branches() pickList[taskBranch] {
	found, _ := p.listing.Value()

	return pickList[taskBranch]{items: found.branches, selected: p.selected}
}

// notAskedNote is the one line saying the tracker could not be asked which
// issues are yours, and why, so the list under it holds every issue's branch,
// set apart from the list by a blank line; nothing when it was asked, or had no
// need to be.
func notAskedNote(kit renderKit, notAsked error) []string {
	if notAsked == nil {
		return nil
	}

	// A failure with no sentence of its own already says what was being asked,
	// and a second lead would push its reason off the row.
	if _, known := errorSentence(notAsked); !known {
		return []string{kit.failureLine(notAsked), ""}
	}

	return []string{
		kit.failedGlyph() + " could not ask which issues are yours: " + inFull(notAsked), "",
	}
}

// branchLabel names a branch by its issue, with the summary when the Issues pane has
// loaded the issue, and the branch name so there is no doubt which will be
// checked out — marked when only the remote has it.
func branchLabel(marks glyphs, branch taskBranch) string {
	named := shownKey(branch.issueKey)
	if branch.summary != "" {
		named += " " + branch.summary
	}

	named += marks.separator + branch.name

	switch {
	case branch.worktreeGone:
		named += " (worktree gone)"
	case branch.worktree != "":
		named += " (worktree at " + branch.worktreeShown + ")"
	case branch.remote:
		named += " (remote)"
	}

	return named
}

// outcome says how switching is going, if it was tried, or why the branch
// chosen cannot be switched to. A refusal is wrapped rather than cut, as
// the task line's is, so git's own words are all seen.
func (p branchPicker) outcome(kit renderKit, width int) []string {
	if p.send.InFlight() {
		return []string{"", "switching" + kit.marks.ellipsis}
	}

	return kit.refusalOrProblem(p.send.Reason(), p.problem, width)
}

// footer offers what works in the switcher now. A switch in flight only offers
// quitting, so nothing interrupts it.
func (p branchPicker) footer(keys keyMap) []key.Binding {
	if p.send.InFlight() {
		return []key.Binding{keys.interrupt}
	}

	return keys.listKeys()
}

// handleKey answers a key while the switcher has the keyboard.
func (p branchPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if p.send.InFlight() {
		return m, nil
	}

	if listed, answered := m.listKey(p, msg); answered {
		return listed, nil
	}

	if key.Matches(msg, m.keys.confirm) {
		return p.choose(m)
	}

	return m, nil
}

// which names the branch picker.
func (branchPicker) which() overlayKind { return overlayBranchPicker }

// openedAs is the round the switcher opened in.
func (p branchPicker) openedAs() int { return p.opened }

// sent is the switch's write.
func (p branchPicker) sent() send.State { return p.send }

// withSend is the switcher with its switch's write as state.
func (p branchPicker) withSend(state send.State) branchPicker {
	p.send = state

	return p
}

// step moves the choice of branch by delta, unless a switch is being sent.
// Another branch chosen drops why the one before could not be switched to; a
// move past either end of the list keeps the same branch and its problem.
// git's refusal of a switch sent stays until another is sent.
func (p branchPicker) step(m Model, delta int) Model {
	if p.send.InFlight() {
		return m
	}

	if moved := p.branches().moved(delta).selected; moved != p.selected {
		p.selected, p.problem = moved, nil
		m.overlay = p
	}

	return m
}

// choose reads the working tree before switching to the selected branch, as
// the web does, so a file edited since the Commits pane last loaded is refused
// rather than carried across. With no way to read the tree it reads as clean.
// A branch another worktree has checked out is switched to by leaving for that
// worktree, which carries nothing across, or refused when that worktree is
// gone, saying how to free the branch.
func (p branchPicker) choose(m Model) (Model, tea.Cmd) {
	branch, ok := p.branches().chosen()
	if !ok {
		return m, nil
	}

	switch {
	case branch.worktreeGone:
		p.send, p.problem = p.send.Cleared(), errBranchHeldByGone
		m.overlay = p

		return m, nil
	case branch.worktree != "":
		return m.closeOverlay().leaveFor(branch.worktree)
	}

	p.send, _ = p.send.Sent()
	p.problem = nil
	m.overlay = p
	ctx, read, opened := m.session, m.deps.Git.Changes, p.opened

	return m, func() tea.Msg {
		if read == nil {
			return treeChecked{opened: opened, name: branch.name}
		}

		changes, err := read(ctx)

		return treeChecked{opened: opened, name: branch.name, changes: changes, err: err}
	}
}

// treeChecked is the working tree as it stood when a switch was chosen, for
// the switcher opened as opened.
type treeChecked struct {
	opened  int
	name    string
	changes []gitrepo.Change
	err     error
}

var _ applier = treeChecked{}

// refusal is why the tree keeps the switch from going: it could not be read,
// or it holds uncommitted work the switch would carry onto the other branch.
func (msg treeChecked) refusal() error {
	if msg.err != nil {
		return msg.err
	}

	return loop.RefuseDirty(msg.changes)
}

// apply switches to the chosen branch, or keeps the switcher open with the
// reason it will not.
// A late answer, its switcher no longer the one open, changes nothing; none
// arrives, since the switcher takes no key but ctrl+c, which quits, while its
// switch is out.
func (msg treeChecked) apply(m Model) (Model, tea.Cmd) {
	m, picker, done := answered[branchPicker](m, msg.opened, msg.refusal())
	if !done {
		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would switch to " + msg.name), nil
	}

	return picker.checkingOut(m, msg.name)
}

// checkingOut puts the switcher back with its switch out again, now as the
// checkout, and is the command that checks name out. Its answer carries the
// round the switcher opened in.
func (p branchPicker) checkingOut(m Model, name string) (Model, tea.Cmd) {
	p.send, _ = p.send.Sent()
	m = m.withAnswered(p)
	ctx, checkout, opened := m.session, m.deps.Git.Checkout, p.opened
	lister := branchLister{links: m.deps.Git.IssueLinks}

	return m, func() tea.Msg {
		err := checkout(ctx, name)

		return taskSwitched{opened: opened, name: name, link: lister.linked(ctx)[name], err: err}
	}
}

// taskSwitched reports how switching to a branch went, with the issue it was
// linked to by hand, if it was, for the switcher opened as opened.
type taskSwitched struct {
	opened int
	name   string
	link   string
	err    error
}

var _ applier = taskSwitched{}

// apply reloads the panes for the branch switched to, or keeps the switcher open
// with git's reason. A branch that names an issue offers to start its task too,
// as creating the branch would have.
// A late answer, its switcher no longer the one open, changes nothing; none
// arrives, since the switcher takes no key but ctrl+c, which quits, while its
// switch is out.
func (msg taskSwitched) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[branchPicker](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " switched to " + msg.name)
	if issueKey, named := loop.NamedIssue(msg.name, map[string]string{msg.name: msg.link}, m.cfg.Jira.Project); named {
		m.followUp = m.offerStart(m.issues.listedIssue(jira.Key(issueKey.Key)))
	}

	m, status := m.readingChanges()
	m, read := m.readingBranch()

	return m, tea.Batch(read, status)
}
