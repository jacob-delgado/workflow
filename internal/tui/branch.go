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

// rail is the branch's name and where it stands against its upstream.
func (s branchState) rail(kit renderKit) string {
	switch {
	case !s.loaded:
		return kit.marks.reading()
	case s.err != nil:
		return kit.unreadRow(s.err, "could not read the branch")
	case s.branch.Detached:
		return "detached HEAD"
	}

	return kit.styles.strong.Render(s.branch.Name) + "\n" + s.upstream(kit)
}

// upstream says where the branch stands against the remote it pushes to.
func (s branchState) upstream(kit renderKit) string {
	branch := s.branch

	switch {
	case branch.Pushed():
		return "pushed"
	case branch.Upstream != branch.PushTarget():
		return "not pushed yet"
	default:
		return kit.marks.ahead + strconv.Itoa(branch.Ahead) + " " + kit.marks.behind + strconv.Itoa(branch.Behind) +
			" against " + branch.Upstream
	}
}

// outsideRepository reports the interface started outside a git work tree, the
// one branch fault every repository-backed pane shows alone, in the failure
// table's words, and offers no repository key for.
func (s branchState) outsideRepository() bool {
	return s.loaded && errors.Is(s.err, gitrepo.ErrNotARepository)
}

// canCreateBranch reports whether a branch can be started: git can create one,
// and the directory is a repository to create it in.
func (m Model) canCreateBranch() bool {
	return m.deps.Git.CreateBranch != nil && !m.branch.outsideRepository()
}

// branchView is what the Branch pane draws with beside its own state: the
// glyphs and styles, the keys it names, the Jira project a branch's name is
// read for an issue in, and the issues the Issues pane lists.
type branchView struct {
	kit     renderKit
	keys    keyMap
	project string
	issues  issueList
}

// branchView is the Branch pane's view of the rest of the interface.
func (m Model) branchView() branchView {
	return branchView{kit: m.kit(), keys: m.keys, project: m.cfg.Jira.Project, issues: m.issues}
}

// detail describes the branch and what to do with it.
func (s branchState) detail(view branchView, width int) string {
	kit := view.kit

	switch {
	case !s.loaded:
		return s.rail(kit)
	case s.outsideRepository():
		return kit.failureBlock(s.err, width)
	case s.err != nil:
		// Why, in the words of whatever refused: a directory that is no
		// repository is one reason among several, and only the reason says
		// what to do about it.
		return wrap(s.rail(kit), width) + "\n\n" + kit.failureBlock(s.err, width)
	case s.branch.Detached:
		return wrap(s.rail(kit)+"\n\nCheck out a branch, or press "+view.keys.newBranch.Help().Key+
			" to start one for the selected issue.", width)
	}

	label := kit.styles.label
	lines := []string{
		kit.styles.strong.Render(s.branch.Name),
		"",
		label.Render("base      ") + valueOr(kit.styles, s.branch.Base, "(none found)"),
		label.Render("upstream  ") + s.upstream(kit),
		label.Render("commits   ") + strconv.Itoa(len(s.branch.Commits)) + " not on the base",
	}

	if branchKey, named := loop.IssueOf(s.branch, view.project); named {
		issue, listed := view.issues.find(jira.Key(branchKey.Key))
		lines = append(lines, label.Render("issue     ")+branchKey.Key+" "+issue.Summary+unlisted(kit.styles, listed))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// unlisted marks an issue the branch names that is not among those assigned.
func unlisted(sty styles, listed bool) string {
	if listed {
		return ""
	}

	return sty.label.Render("(not among your open issues)")
}

// valueOr is a value, or what to say when there is none.
func valueOr(sty styles, value, none string) string {
	if value == "" {
		return sty.label.Render(none)
	}

	return value
}

// branchOffers are the Branch pane's keys: starting a branch, switching to
// another task's, linking it to an issue by hand, catching it up with its
// base, pushing it, and reading it again. Outside a repository there is no
// branch to act on, and none is offered.
func (m Model) branchOffers() []offer {
	if m.branch.outsideRepository() {
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

// issue is the issue the branch is for, by its link or its name in the
// Jira project, typed as a jira.Key, and whether it is for one.
func (s branchState) issue(project string) (jira.Key, bool) {
	key, ok := loop.IssueOf(s.branch, project)

	return jira.Key(key.Key), ok
}

// handleBranchKey answers the Branch pane's own keys, as its footer offers
// them.
func (m Model) handleBranchKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	return m.answer(m.branchOffers(), msg)
}

// branchBehavior is the Branch pane's behavior.
func branchBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.branch.rail(m.kit()) },
		detail: func(m Model, width int) string { return m.branch.detail(m.branchView(), width) }, narrow: nil,
		keys: Model.branchKeys, handle: Model.handleBranchKey, pick: nil,
		refresh: Model.refreshBranch, loading: func(m Model) bool { return m.branch.loading },
		scroll: func(m *Model) *int { return &m.branch.scroll }, readsBranch: true,
		answers: []string{"new-branch", "switch-branch", "link-issue", "rebase", "push", actionRefresh},
	}
}
