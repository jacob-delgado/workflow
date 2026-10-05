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
)

// switchTitle titles the detail pane while the task switcher is open.
const switchTitle = "Switch task"

// errDirtyTree is the interface's words for loop.ErrDirtyTree: a switch refused
// for the uncommitted work it would carry onto another branch. Stashing is left
// to the person, so the reason says what to do rather than doing it.
var errDirtyTree = errors.New("uncommitted changes — commit or stash them before switching tasks")

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
}

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

// apply records the branches on the open switcher, or does nothing when it has
// since closed.
func (msg branchesListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(branchPicker)
	if !open {
		return m, nil
	}

	picker.branches = pickList[taskBranch]{items: m.taskBranches(msg)}
	picker.listErr, picker.notAsked, picker.settled = msg.err, msg.notAsked, true
	m.overlay = picker

	return m, nil
}

// taskBranches keeps the branches that name one of your issues, other than the
// one checked out, and names each by its issue.
func (m Model) taskBranches(listed branchesListed) []taskBranch {
	current := m.branch.branch.Name

	var branches []taskBranch

	for _, branch := range listed.candidates() {
		key, named := loop.NamedIssue(branch.name, listed.links, m.cfg.Jira.Project)
		if !named || branch.name == current || !listed.yours(jira.Key(key.Key)) {
			continue
		}

		issue, _ := m.issues.find(jira.Key(key.Key))
		branch.issueKey, branch.summary = jira.Key(key.Key), issue.Summary

		if worktree, elsewhere := listed.worktrees[branch.name]; elsewhere {
			branch.worktree, branch.worktreeShown = worktree.Dir, m.shownDir(worktree.Dir)
			branch.worktreeGone = worktree.Missing
		}

		branches = append(branches, branch)
	}

	return branches
}

// branchPicker lists the issue branches to switch to, and how a switch is going.
type branchPicker struct {
	marks    glyphs
	styles   styles
	branches pickList[taskBranch]
	listErr  error
	notAsked error
	settled  bool
	send     sendState
}

var (
	_ failable[branchPicker] = branchPicker{}
	_ steppable              = branchPicker{}
)

// openBranchPicker opens the task switcher and starts listing the branches.
func (m Model) openBranchPicker() (Model, tea.Cmd) {
	m.overlay = branchPicker{marks: m.marks, styles: m.styles}
	lister := branchLister{
		local: m.deps.Git.Branches, remote: m.deps.Git.RemoteBranches, links: m.deps.Git.IssueLinks,
		search: m.deps.Jira.SearchLenient, project: m.cfg.Jira.Project,
		worktrees: m.deps.Repositories.Worktrees, here: m.deps.Repositories.Here.Root,
	}

	return m, func() tea.Msg {
		listed := lister.list()
		listed.worktrees = lister.otherWorktrees()

		return listed
	}
}

// branchLister lists the branches the switcher offers and asks the tracker which
// of the issues they name are yours. A nil remote lists none there; a nil search
// asks nothing, and a failed one answers nothing, so every issue counts.
type branchLister struct {
	local     func() ([]string, error)
	remote    func() ([]string, error)
	links     func() map[string]string
	search    func(jql string, startAt int) (jira.SearchResult, error)
	project   string
	worktrees func() ([]gitrepo.Worktree, error)
	// here is the root of the worktree this session works in.
	here string
}

// otherWorktrees is each other worktree, gone or not, by the branch it has
// checked out: none outside a repository or when the read fails, when each
// branch is checked out as before.
func (l branchLister) otherWorktrees() map[string]gitrepo.Worktree {
	byBranch := map[string]gitrepo.Worktree{}
	if l.worktrees == nil {
		return byBranch
	}

	worktrees, _ := l.worktrees()
	for _, worktree := range worktrees {
		if worktree.Branch != "" && worktree.Dir != l.here {
			byBranch[worktree.Branch] = worktree
		}
	}

	return byBranch
}

// list is the local branches, then the remote ones, then which issues are yours.
// A listing git could not make is the listing's error; a tracker that could not
// be asked leaves every issue counted, with the reason it was not.
func (l branchLister) list() branchesListed {
	local, err := l.local()
	if err != nil {
		return branchesListed{err: err}
	}

	remote, err := l.remoteNames()
	if err != nil {
		return branchesListed{err: err}
	}

	links := l.linked()
	if l.search == nil {
		return branchesListed{local: local, remote: remote, links: links}
	}

	mine, err := loop.AssignedKeys(l.search, l.issueKeys(slices.Concat(local, remote), links))
	if err != nil {
		return branchesListed{local: local, remote: remote, links: links, notAsked: err}
	}

	return branchesListed{local: local, remote: remote, links: links, mine: mine}
}

// linked is every branch linked to an issue by hand, or none with no
// repository to ask.
func (l branchLister) linked() map[string]string {
	if l.links == nil {
		return nil
	}

	return l.links()
}

// remoteNames lists the remote's branches, or none with no repository to ask.
func (l branchLister) remoteNames() ([]string, error) {
	if l.remote == nil {
		return nil, nil
	}

	return l.remote()
}

// issueKeys is the issue each of names is for, by its link in links or its
// name, for those that are for one.
func (l branchLister) issueKeys(names []string, links map[string]string) []jira.Key {
	var keys []jira.Key

	for _, name := range names {
		if key, named := loop.NamedIssue(name, links, l.project); named {
			keys = append(keys, jira.Key(key.Key))
		}
	}

	return keys
}

// view draws the switcher in as many rows as fit.
func (p branchPicker) view(_, rows int) (string, string) {
	lines := slices.Concat([]string{"Switch to another task's branch.", ""}, p.notAskedNote())

	switch {
	case !p.settled:
		lines = append(lines, "loading branches"+p.marks.ellipsis)
	case p.listErr != nil:
		lines = append(lines, failureLine(p.styles, p.marks, p.listErr))
	case len(p.branches.items) == 0:
		lines = append(lines, "No other task branch to switch to.")
	default:
		lines = append(lines, p.branches.rows(p.marks, rows-len(lines)-outcomeRows, p.label)...)
		lines = append(lines, p.outcome()...)
	}

	return switchTitle, strings.Join(lines, "\n")
}

// notAskedNote is the one line saying the tracker could not be asked which
// issues are yours, and why, so the list under it holds every issue's branch,
// set apart from the list by a blank line; nothing when it was asked, or had no
// need to be.
func (p branchPicker) notAskedNote() []string {
	if p.notAsked == nil {
		return nil
	}

	// A failure with no sentence of its own already says what was being asked,
	// and a second lead would push its reason off the row.
	if _, known := errorSentence(p.notAsked); !known {
		return []string{failureLine(p.styles, p.marks, p.notAsked), ""}
	}

	return []string{failedGlyph(p.styles, p.marks) + " could not ask which issues are yours: " + inFull(p.notAsked), ""}
}

// label names a branch by its issue, with the summary when the Issues pane has
// loaded the issue, and the branch name so there is no doubt which will be
// checked out — marked when only the remote has it.
func (p branchPicker) label(branch taskBranch) string {
	named := string(branch.issueKey)
	if branch.summary != "" {
		named += " " + branch.summary
	}

	named += p.marks.separator + branch.name

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

// outcome says how switching is going, if it was tried.
func (p branchPicker) outcome() []string {
	switch {
	case p.send.sending:
		return []string{"", "switching" + p.marks.ellipsis}
	case p.send.err != nil:
		return []string{"", failureLine(p.styles, p.marks, p.send.err)}
	default:
		return nil
	}
}

// footer offers what works in the switcher now. A switch in flight only offers
// quitting, so nothing interrupts it.
func (p branchPicker) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return keys.listKeys()
}

// handleKey answers a key while the switcher has the keyboard.
func (p branchPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		return p.step(m, 1), nil
	case key.Matches(msg, m.keys.up):
		return p.step(m, -1), nil
	case key.Matches(msg, m.keys.confirm):
		return p.choose(m)
	}

	m.overlay = p

	return m, nil
}

// step moves the choice of branch by delta, unless a switch is being sent.
func (p branchPicker) step(m Model, delta int) Model {
	if p.send.sending {
		return m
	}

	p.branches = p.branches.moved(delta)
	m.overlay = p

	return m
}

// choose reads the working tree before switching to the selected branch, as
// the web does, so a file edited since the Commits pane last loaded is refused
// rather than carried across. With no way to read the tree it reads as clean.
// A branch another worktree has checked out is switched to by leaving for that
// worktree, which carries nothing across, or refused when that worktree is
// gone, saying how to free the branch.
func (p branchPicker) choose(m Model) (Model, tea.Cmd) {
	branch, ok := p.branches.chosen()
	if !ok {
		return m, nil
	}

	switch {
	case branch.worktreeGone:
		return keepOpenWith[branchPicker](m, errBranchHeldByGone), nil
	case branch.worktree != "":
		return m.closeOverlay().leaveFor(branch.worktree)
	}

	p.send = starting()
	m.overlay = p
	read := m.deps.Git.Changes

	return m, func() tea.Msg {
		if read == nil {
			return treeChecked{name: branch.name}
		}

		changes, err := read()

		return treeChecked{name: branch.name, changes: changes, err: err}
	}
}

// treeChecked is the working tree as it stood when a switch was chosen.
type treeChecked struct {
	name    string
	changes []gitrepo.Change
	err     error
}

// apply switches to the chosen branch, or keeps the switcher open with the
// reason it will not: the tree could not be read, or it holds uncommitted work
// the switch would carry onto the other branch.
func (msg treeChecked) apply(m Model) (Model, tea.Cmd) {
	if _, open := m.overlay.(branchPicker); !open {
		return m, nil
	}

	switch {
	case msg.err != nil:
		return keepOpenWith[branchPicker](m, msg.err), nil
	case loop.RefuseDirty(msg.changes) != nil:
		return keepOpenWith[branchPicker](m, errDirtyTree), nil
	case m.dryRun:
		return m.closeOverlay().noticed("dry run: would switch to " + msg.name), nil
	}

	checkout, name, lister := m.deps.Git.Checkout, msg.name, branchLister{links: m.deps.Git.IssueLinks}

	return m, func() tea.Msg {
		err := checkout(name)

		return taskSwitched{name: name, link: lister.linked()[name], err: err}
	}
}

// taskSwitched reports how switching to a branch went, with the issue it was
// linked to by hand, if it was.
type taskSwitched struct {
	name string
	link string
	err  error
}

// apply reloads the panes for the branch switched to, or keeps the switcher open
// with git's reason. A branch that names an issue offers to start its task too,
// as creating the branch would have.
func (msg taskSwitched) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchPicker](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " switched to " + msg.name)
	if issueKey, named := loop.NamedIssue(msg.name, map[string]string{msg.name: msg.link}, m.cfg.Jira.Project); named {
		m.followUp = m.offerStart(m.listedIssue(jira.Key(issueKey.Key)))
	}

	return m, tea.Batch(m.loadBranch(), m.loadChanges())
}

// failed is the switcher kept open with the reason it could not switch.
func (p branchPicker) failed(err error) branchPicker {
	p.send = p.send.failed(err)

	return p
}
