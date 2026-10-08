// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// branchState is the checked-out branch, as far as it has loaded, and how far
// the Branch pane's detail is scrolled, which another branch starts at the top.
type branchState struct {
	branch gitrepo.Branch
	loaded bool
	err    error
	scroll int
	// loading is a refresh begun and not yet answered.
	loading bool
}

// branchLoaded carries the branch as git reports it.
type branchLoaded struct {
	branch gitrepo.Branch
	err    error
}

var _ applier = branchLoaded{}

// apply records the branch, selects the issue it is for, and looks for its pull
// request.
func (msg branchLoaded) apply(m Model) (Model, tea.Cmd) {
	previous := m.branch.branch.Name
	m.branch = branchState{branch: msg.branch, loaded: true, err: msg.err, scroll: m.branch.scroll}

	if previous != msg.branch.Name {
		m = m.beginReview(reviewState{})
		m = m.withoutQueuedPost()
		// The failed-post error, its author and its dropped reason belonged to
		// the branch just left; on a new branch they are stale, and so is how far
		// the Branch and messaging details were scrolled into it.
		m.messaging.send.err, m.messaging.author, m.messaging.dropped = nil, "", ""
		m.branch.scroll, m.messaging.scroll = 0, 0
	}

	m, detail := m.resumeIssue().loadDetail()

	find := m.findPullRequest()
	if find == nil {
		// No pull request is looked for, so a Review refresh waits on nothing more.
		m.review.loading = false
	}

	// Your forge name is asked for here, not only once a pull request is
	// found, because assigning a forge issue starts from it on any branch.
	return m, tea.Batch(detail, find, m.loadAuthor())
}

// refreshBranch reads the branch again, and the work tree with it.
func (m Model) refreshBranch() (Model, tea.Cmd) {
	read := m.loadBranch()
	m.branch.loading = read != nil

	return m, tea.Batch(read, m.loadChanges())
}

// loadBranch is the command that reads the branch.
func (m Model) loadBranch() tea.Cmd {
	read := m.deps.Git.Branch
	if read == nil {
		return nil
	}

	return func() tea.Msg {
		branch, err := read()

		return branchLoaded{branch: branch, err: err}
	}
}

// onFeatureBranch reports a named branch other than the one work merges into.
func (s branchState) onFeatureBranch() bool {
	return s.loaded && s.err == nil && loop.OnFeatureBranch(s.branch)
}

// branchRail is the branch's name and where it stands against its upstream.
func (m Model) branchRail(_ int) string {
	switch {
	case !m.branch.loaded:
		return m.marks.reading()
	case m.branch.err != nil:
		return unreadRow(m.styles, m.marks, m.branch.err, "could not read the branch")
	case m.branch.branch.Detached:
		return "detached HEAD"
	}

	return m.styles.strong.Render(m.branch.branch.Name) + "\n" + m.upstreamState()
}

// upstreamState says where the branch stands against the remote it pushes to.
func (m Model) upstreamState() string {
	branch := m.branch.branch

	switch {
	case branch.Pushed():
		return "pushed"
	case branch.Upstream != branch.PushTarget():
		return "not pushed yet"
	default:
		return m.marks.ahead + strconv.Itoa(branch.Ahead) + " " + m.marks.behind + strconv.Itoa(branch.Behind) +
			" against " + branch.Upstream
	}
}

// outsideRepository reports the interface started outside a git work tree, the
// one branch fault every repository-backed pane shows alone, in the failure
// table's words, and offers no repository key for.
func (m Model) outsideRepository() bool {
	return m.branch.loaded && errors.Is(m.branch.err, gitrepo.ErrNotARepository)
}

// canCreateBranch reports whether a branch can be started: git can create one,
// and the directory is a repository to create it in.
func (m Model) canCreateBranch() bool {
	return m.deps.Git.CreateBranch != nil && !m.outsideRepository()
}

// branchDetail describes the branch and what to do with it.
func (m Model) branchDetail(width int) string {
	switch {
	case !m.branch.loaded:
		return m.branchRail(0)
	case m.outsideRepository():
		return m.failureBlock(m.branch.err, width)
	case m.branch.err != nil:
		// Why, in the words of whatever refused: a directory that is no
		// repository is one reason among several, and only the reason says
		// what to do about it.
		return wrap(m.branchRail(0), width) + "\n\n" + m.failureBlock(m.branch.err, width)
	case m.branch.branch.Detached:
		return wrap(m.branchRail(0)+"\n\nCheck out a branch, or press "+m.keys.newBranch.Help().Key+
			" to start one for the selected issue.", width)
	}

	branch := m.branch.branch
	lines := []string{
		m.styles.strong.Render(branch.Name),
		"",
		m.styles.label.Render("base      ") + m.valueOr(branch.Base, "(none found)"),
		m.styles.label.Render("upstream  ") + m.upstreamState(),
		m.styles.label.Render("commits   ") + strconv.Itoa(len(branch.Commits)) + " not on the base",
	}

	if branchKey, named := loop.IssueOf(branch, m.cfg.Jira.Project); named {
		issue, listed := m.issues.find(jira.Key(branchKey.Key))
		lines = append(lines, m.styles.label.Render("issue     ")+branchKey.Key+" "+issue.Summary+m.unlisted(listed))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// unlisted marks an issue the branch names that is not among those assigned.
func (m Model) unlisted(listed bool) string {
	if listed {
		return ""
	}

	return m.styles.label.Render("(not among your open issues)")
}

// valueOr is a value, or what to say when there is none.
func (m Model) valueOr(value, none string) string {
	if value == "" {
		return m.styles.label.Render(none)
	}

	return value
}

// branchOffers are the Branch pane's keys: starting a branch, switching to
// another task's, linking it to an issue by hand, catching it up with its
// base, pushing it, and reading it again. Outside a repository there is no
// branch to act on, and none is offered.
func (m Model) branchOffers() []offer {
	if m.outsideRepository() {
		return nil
	}

	return []offer{
		{binding: m.keys.newBranch, can: m.canCreateBranch(), act: m.openBranchCreator},
		{binding: m.keys.switchBranch, can: m.canSwitchTask(), act: m.openBranchPicker},
		{binding: m.keys.linkIssue, can: m.canLinkIssue(), act: m.openBranchLink},
		{binding: m.keys.rebase, can: m.canRebase(), act: m.previewRebase},
		{binding: m.keys.push, can: m.canPush(), act: m.previewPush},
		{binding: m.keys.refresh, can: true, act: func() (Model, tea.Cmd) { return m.refreshPane(paneBranch) }},
	}
}

// branchKeys is the Branch pane's footer: the offers that act right now.
func (m Model) branchKeys() []key.Binding {
	return liveKeys(m.branchOffers())
}

// canSwitchTask reports that the repository can list and switch branches.
func (m Model) canSwitchTask() bool {
	return m.deps.Git.Branches != nil && m.deps.Git.Checkout != nil
}

// canRebase reports a feature branch with a base to catch up with.
func (m Model) canRebase() bool {
	return m.branch.onFeatureBranch() && loop.CanRebase(m.branch.branch) && m.deps.Git.Rebase != nil
}

// canPush reports a branch with something to push.
func (m Model) canPush() bool {
	return m.branch.onFeatureBranch() && !m.branch.branch.Pushed() && len(m.branch.branch.Commits) > 0 &&
		m.deps.Git.Push != nil
}

// previewPush holds the push for a last look at what it sends and where.
func (m Model) previewPush() (Model, tea.Cmd) {
	m.overlay = lastLook{
		title: "Push branch",
		body:  "Push " + m.branch.branch.Name + " to " + m.branch.branch.PushRemote + "?", verb: "push",
		proceed: func(m Model) (Model, tea.Cmd) { return m.startPush(nil) },
	}

	return m, nil
}

// previewRebase holds the rebase for a last look at the branch whose history it
// rewrites and the base it replays that branch onto.
func (m Model) previewRebase() (Model, tea.Cmd) {
	m.overlay = lastLook{
		title: "Rebase branch",
		body:  "Rebase " + m.branch.branch.Name + " onto " + m.branch.branch.Base + "?", verb: "rebase",
		proceed: Model.startRebase,
	}

	return m, nil
}

// branchIssue is the issue the current branch is for, by its link or its name,
// typed as a jira.Key, and whether it is for one.
func (m Model) branchIssue() (jira.Key, bool) {
	key, ok := loop.IssueOf(m.branch.branch, m.cfg.Jira.Project)

	return jira.Key(key.Key), ok
}

// handleBranchKey answers the Branch pane's own keys, as its footer offers
// them.
func (m Model) handleBranchKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	return m.answer(m.branchOffers(), msg)
}
