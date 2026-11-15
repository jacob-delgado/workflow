// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// branchState is the checked-out branch, as the latest read of it found it,
// and how far the Branch pane's detail is scrolled, which another branch
// starts at the top.
type branchState struct {
	read   latest.Read[gitrepo.Branch]
	scroll int
}

// branchLoaded carries the branch as git reports it, to the read begun in
// round.
type branchLoaded struct {
	branch gitrepo.Branch
	err    error
	round  latest.Round
}

var _ applier = branchLoaded{}

// apply records the branch, selects the issue it is for, and looks for its
// pull request. The answer of a read a later one superseded is dropped: it
// would show the branch as it stood before, and look for a pull request of a
// branch since left. A read that failed once the branch was read says nothing
// of it — git answers no branch with its error — so the branch, and all that
// hangs off it, stays, beside the failure.
func (msg branchLoaded) apply(m Model) (Model, tea.Cmd) {
	previous, held := m.branch.read.Value()

	read, taken := m.branch.read.Answered(msg.round, msg.branch, msg.err)
	if !taken {
		return m, nil
	}

	m.branch.read = read

	if msg.err != nil && held {
		return m.noticedFailureLedBy("could not read the branch: ", msg.err), nil
	}

	if previous.Name != msg.branch.Name {
		m = m.beginReview(m.review.find.Cleared())
		// The failed-post error and its author belonged to the branch just
		// left; on a new branch they are stale, and so is how far the Branch
		// and messaging details were scrolled into it. An announcement waiting
		// for CI is bound to the branch it was armed on, and waits on.
		m.messaging.postNow, m.messaging.author = m.messaging.postNow.Cleared(), ""
		m.branch.scroll, m.messaging.scroll = 0, 0
	}

	m, detail := m.resumeIssue().followDetail()
	m, find := m.findingPull()

	// Your forge name is asked for here, not only once a pull request is
	// found, because assigning a forge issue starts from it on any branch.
	return m, tea.Batch(detail, find, m.messaging.loadAuthor(m.session, m.deps))
}

// readingBranch begins a read of the branch, superseding any still out, and
// is the command that reads it; with no git to ask, it begins nothing.
func (m Model) readingBranch() (Model, tea.Cmd) {
	if m.deps.Git.Branch == nil {
		return m, nil
	}

	read, round := m.branch.read.Begin()
	m.branch.read = read

	return m, loadBranch(m.session, m.deps, round)
}

// loadBranch is the command that reads the branch, in round.
func loadBranch(ctx context.Context, deps Deps, round latest.Round) tea.Cmd {
	read := deps.Git.Branch
	if read == nil {
		return nil
	}

	return func() tea.Msg {
		branch, err := read(ctx)

		return branchLoaded{branch: branch, err: err, round: round}
	}
}

// checkedOut is the branch held: the latest successful read's, kept across a
// read that failed since, or no branch before one succeeded.
func (s branchState) checkedOut() gitrepo.Branch {
	branch, _ := s.read.Value()

	return branch
}

// onFeatureBranch reports a branch held that is named and other than the one
// work merges into.
func (s branchState) onFeatureBranch() bool {
	branch, held := s.read.Value()

	return held && loop.OnFeatureBranch(branch)
}

// rail is the branch's name and where it stands against its upstream; after a
// failed read, the failure first, so a rail of one row still says it, then
// the name held.
func (s branchState) rail(kit renderKit) string {
	failed := func() string { return kit.unreadRow(s.read.Err(), "could not read the branch") }

	return drawRead(s.read, drawnRead[string]{
		reading:    kit.reading,
		shown:      func() string { return s.named(kit) },
		failed:     failed,
		failedOver: func() string { return failed() + "\n" + s.heading(kit) },
	})
}

// named is the branch held and where it stands against its upstream.
func (s branchState) named(kit renderKit) string {
	if s.checkedOut().Detached {
		return s.heading(kit)
	}

	return s.heading(kit) + "\n" + s.upstream(kit)
}

// heading is the branch held by name, or that HEAD is detached.
func (s branchState) heading(kit renderKit) string {
	branch := s.checkedOut()
	if branch.Detached {
		return "detached HEAD"
	}

	return kit.styles.strong.Render(branch.Name)
}

// upstream says where the branch stands against the remote it pushes to.
func (s branchState) upstream(kit renderKit) string {
	branch := s.checkedOut()

	switch {
	case branch.Pushed():
		return "pushed"
	case branch.Upstream != branch.PushTarget():
		return "not pushed yet"
	case branch.Uncounted:
		return "not counted against " + branch.Upstream
	default:
		return kit.marks.ahead + strconv.Itoa(branch.Ahead) + " " + kit.marks.behind + strconv.Itoa(branch.Behind) +
			" against " + branch.Upstream
	}
}

// outsideRepository reports the interface started outside a git work tree, the
// one branch fault every repository-backed pane shows alone, in the failure
// table's words, and offers no repository key for. A read that fails so once
// a branch was read leaves that branch shown, beside the failure.
func (s branchState) outsideRepository() bool {
	_, held := s.read.Value()

	return !held && s.read.Stage() == latest.StageFailed && errors.Is(s.read.Err(), gitrepo.ErrNotARepository)
}

// canCreate reports whether a branch can be started: git can create one,
// and the directory is a repository to create it in.
func (s branchState) canCreate(deps Deps) bool {
	return deps.Git.CreateBranch != nil && !s.outsideRepository()
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

// detail describes the branch and what to do with it; a failed read is said
// beneath the branch it kept.
func (s branchState) detail(view branchView, width int) string {
	kit := view.kit
	if s.outsideRepository() {
		return kit.failureBlock(s.read.Err(), width)
	}

	failure := func() string { return kit.failureBlock(s.read.Err(), width) }

	return drawRead(s.read, drawnRead[string]{
		reading: kit.reading,
		shown:   func() string { return s.described(view, width) },
		// Why, in the words of whatever refused: a directory that is no
		// repository is one reason among several, and only the reason says
		// what to do about it.
		failed:     func() string { return wrap(s.rail(kit), width) + "\n\n" + failure() },
		failedOver: func() string { return s.described(view, width) + "\n\n" + failure() },
	})
}

// described is the branch held: its base, upstream, commits and issue, or
// that HEAD is detached and what to do about it.
func (s branchState) described(view branchView, width int) string {
	kit, branch := view.kit, s.checkedOut()
	if branch.Detached {
		return wrap(s.heading(kit)+"\n\nCheck out a branch, or press "+view.keys.newBranch.Help().Key+
			" to start one for the selected issue.", width)
	}

	label := kit.styles.label
	lines := []string{
		kit.styles.strong.Render(branch.Name),
		"",
		label.Render("base      ") + valueOr(kit.styles, branch.Base, "(none found)"),
		label.Render("upstream  ") + s.upstream(kit),
		label.Render("commits   ") + strconv.Itoa(len(branch.Commits)) + " not on the base",
	}

	// A fork's clone pushes somewhere other than where it merges: say where.
	if branch.PushRemote != branch.BaseRemote {
		lines = append(lines, label.Render("push to   ")+branch.PushRemote)
	}

	if branchKey, named := loop.IssueOf(branch, view.project); named {
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
		{binding: m.keys.newBranch, can: m.branch.canCreate(m.deps), act: m.openBranchCreator},
		{binding: m.keys.switchBranch, can: canSwitchTask(m.deps), act: m.openBranchPicker},
		{binding: m.keys.linkIssue, can: m.branch.canLinkIssue(m.deps), act: m.openBranchLink},
		{binding: m.keys.rebase, can: m.branch.canRebase(m.deps), act: m.previewRebase},
		{binding: m.keys.push, can: m.branch.canPush(m.deps), act: m.previewPush},
		{binding: m.keys.refresh, can: true, act: func() (Model, tea.Cmd) { return m.refreshPane(paneBranch) }},
	}
}

// canSwitchTask reports that the repository can list and switch branches.
func canSwitchTask(deps Deps) bool {
	return deps.Git.Branches != nil && deps.Git.Checkout != nil
}

// canRebase reports a feature branch with a base to catch up with.
func (s branchState) canRebase(deps Deps) bool {
	return s.onFeatureBranch() && loop.CanRebase(s.checkedOut()) && deps.Git.Rebase != nil
}

// canPush reports a branch with something to push.
func (s branchState) canPush(deps Deps) bool {
	branch := s.checkedOut()

	return s.onFeatureBranch() && !branch.Pushed() && len(branch.Commits) > 0 &&
		deps.Git.Push != nil
}

// previewPush holds the push for a last look at what it sends and where.
func (m Model) previewPush() (Model, tea.Cmd) {
	m.overlay = lastLook{
		title: "Push branch",
		body:  "Push " + m.branch.checkedOut().Name + " to " + m.branch.checkedOut().PushRemote + "?", verb: "push",
		proceed: func(m Model) (Model, tea.Cmd) { return m.startPush() },
	}

	return m, nil
}

// previewRebase holds the rebase for a last look at the branch whose history it
// rewrites and the base it replays that branch onto.
func (m Model) previewRebase() (Model, tea.Cmd) {
	m.overlay = lastLook{
		title: "Rebase branch",
		body:  "Rebase " + m.branch.checkedOut().Name + " onto " + m.branch.checkedOut().Base + "?", verb: "rebase",
		proceed: Model.startRebase,
	}

	return m, nil
}

// issue is the issue the branch is for, by its link or its name in the
// Jira project, typed as a jira.Key, and whether it is for one.
func (s branchState) issue(project string) (jira.Key, bool) {
	key, ok := loop.IssueOf(s.checkedOut(), project)

	return jira.Key(key.Key), ok
}

// branchBehavior is the Branch pane's behavior.
func branchBehavior() behavior {
	return behavior{
		rail:   func(m Model, _ int) string { return m.branch.rail(m.kit()) },
		detail: func(m Model, width int) string { return m.branch.detail(m.branchView(), width) }, narrow: nil,
		keys:   func(m Model) []key.Binding { return liveKeys(m.branchOffers()) },
		handle: func(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) { return m.answer(m.branchOffers(), msg) },
		pick:   nil,
		// refresh reads the branch again, and the work tree with it.
		refresh: func(m Model) (Model, tea.Cmd) {
			m, read := m.readingBranch()
			m, status := m.readingChanges()

			return m, tea.Batch(read, status)
		},
		loading: func(m Model) bool { return m.branch.read.Refreshing() },
		scroll:  func(m *Model) *int { return &m.branch.scroll }, readsBranch: true,
		answers: []string{"new-branch", "switch-branch", "link-issue", "rebase", "push", actionRefresh},
	}
}
