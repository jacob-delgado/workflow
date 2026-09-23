// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// notInRepository is what every repository-backed pane says when the interface
// was started outside a git work tree, so the fact is stated one way.
const notInRepository = "Not inside a git repository. Start `workflow` from one to use this pane."

// branchState is the checked-out branch, as far as it has loaded.
type branchState struct {
	branch gitrepo.Branch
	loaded bool
	err    error
}

// branchLoaded carries the branch as git reports it.
type branchLoaded struct {
	branch gitrepo.Branch
	err    error
}

// apply records the branch, selects the issue it is for, and looks for its pull
// request.
func (msg branchLoaded) apply(m Model) (Model, tea.Cmd) {
	previous := m.branch.branch.Name
	m.branch = branchState{branch: msg.branch, loaded: true, err: msg.err}

	if previous != msg.branch.Name {
		m.review = reviewState{}
		m = m.withoutQueuedPost()
		// The failed-post error, its author and its dropped reason belonged to
		// the branch just left; on a new branch they are stale.
		m.messaging.err, m.messaging.author, m.messaging.dropped = nil, "", ""
	}

	m, detail := m.resumeIssue().loadDetail()

	return m, tea.Batch(detail, m.findPullRequest())
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
	name := s.branch.Name

	return s.loaded && s.err == nil && name != "" && name != s.branch.BaseName()
}

// branchRail is the branch's name and where it stands against its upstream.
func (m Model) branchRail(_ int) string {
	switch {
	case !m.branch.loaded:
		return "loading" + m.marks.ellipsis
	case m.branch.err != nil:
		return m.failedGlyph() + " could not read the branch" + m.marks.separator + "see detail"
	case m.branch.branch.Detached:
		return "detached HEAD"
	}

	return m.styles.strong.Render(m.branch.branch.Name) + "\n" + m.upstreamState()
}

// upstreamState says where the branch stands against origin.
func (m Model) upstreamState() string {
	branch := m.branch.branch

	switch {
	case branch.Pushed():
		return "pushed"
	case branch.Upstream != gitrepo.DefaultRemote+"/"+branch.Name:
		return "not pushed yet"
	default:
		return m.marks.ahead + strconv.Itoa(branch.Ahead) + " " + m.marks.behind + strconv.Itoa(branch.Behind) +
			" against " + branch.Upstream
	}
}

// outsideRepository reports the interface started outside a git work tree, the
// one branch fault a pane speaks to directly rather than by relaying git.
func (m Model) outsideRepository() bool {
	return m.branch.loaded && errors.Is(m.branch.err, gitrepo.ErrNotARepository)
}

// branchDetail describes the branch and what to do with it.
func (m Model) branchDetail(width int) string {
	switch {
	case !m.branch.loaded:
		return m.branchRail(0)
	case m.outsideRepository():
		return wrap(notInRepository, width)
	case m.branch.err != nil:
		// Why, in the words of whatever refused: a directory that is no
		// repository is one reason among several, and only the reason says
		// what to do about it.
		return wrap(m.branchRail(0), width) + "\n\n" + m.failureWithin(m.branch.err, width)
	case m.branch.branch.Detached:
		return wrap(m.branchRail(0)+"\n\nCheck out a branch, or press b to start one for the selected issue.", width)
	}

	branch := m.branch.branch
	lines := []string{
		m.styles.strong.Render(branch.Name),
		"",
		m.styles.label.Render("base      ") + m.valueOr(branch.Base, "(none found)"),
		m.styles.label.Render("upstream  ") + m.upstreamState(),
		m.styles.label.Render("commits   ") + strconv.Itoa(len(branch.Commits)) + " not on the base",
	}

	if branchKey, named := convention.IssueKey(branch.Name, m.cfg.Jira.Project); named {
		issue, listed := m.issues.find(jira.Key(branchKey))
		lines = append(lines, m.styles.label.Render("issue     ")+branchKey+" "+issue.Summary+m.unlisted(listed))
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

// branchKeys offers starting a branch, and pushing one that is not pushed.
func (m Model) branchKeys() []key.Binding {
	if m.outsideRepository() {
		return nil
	}

	keys := []key.Binding{m.keys.newBranch}

	if m.canSwitchTask() {
		keys = append(keys, m.keys.switchTask)
	}

	if m.canRebase() {
		keys = append(keys, m.keys.rebase)
	}

	if m.canPush() {
		keys = append(keys, m.keys.push)
	}

	return append(keys, m.keys.refresh)
}

// canSwitchTask reports that the repository can list and switch branches.
func (m Model) canSwitchTask() bool {
	return m.deps.Git.Branches != nil && m.deps.Git.Checkout != nil
}

// canRebase reports a feature branch with a base to catch up with.
func (m Model) canRebase() bool {
	return m.branch.onFeatureBranch() && m.branch.branch.Base != "" && m.deps.Git.Rebase != nil
}

// canPush reports a branch with something to push.
func (m Model) canPush() bool {
	return m.branch.onFeatureBranch() && !m.branch.branch.Pushed() && len(m.branch.branch.Commits) > 0 &&
		m.deps.Git.Push != nil
}

// previewPush holds the push for a last look at what it sends and where.
func (m Model) previewPush() (Model, tea.Cmd) {
	m.overlay = lastLook{
		marks: m.marks, styles: m.styles, title: "Push branch",
		body: "push " + m.branch.branch.Name + " to " + m.branch.remote(), verb: "push",
		proceed: func(m Model) (Model, tea.Cmd) { return m.startPush(nil) },
	}

	return m, nil
}

// previewRebase holds the rebase for a last look at the branch whose history it
// rewrites and the base it replays that branch onto.
func (m Model) previewRebase() (Model, tea.Cmd) {
	m.overlay = lastLook{
		marks: m.marks, styles: m.styles, title: "Rebase branch",
		body: "rebase " + m.branch.branch.Name + " onto " + m.branch.branch.Base, verb: "rebase",
		proceed: Model.startRebase,
	}

	return m, nil
}

// branchIssue is the issue the current branch names, and whether it names one —
// the one place the interface reads a branch name as an issue key, and where the
// branch-derived string becomes a typed jira.Key.
func (m Model) branchIssue() (jira.Key, bool) {
	key, ok := convention.IssueKey(m.branch.branch.Name, m.cfg.Jira.Project)

	return jira.Key(key), ok
}

// remote is the remote the branch's upstream lives on, or origin by default.
func (s branchState) remote() string {
	if before, _, found := strings.Cut(s.branch.Upstream, "/"); found && before != "" {
		return before
	}

	return gitrepo.DefaultRemote
}

// handleBranchKey answers the Branch pane's own keys.
func (m Model) handleBranchKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.newBranch):
		return m.openBranchCreator()
	case key.Matches(msg, m.keys.switchTask) && m.canSwitchTask():
		return m.openBranchPicker()
	case key.Matches(msg, m.keys.rebase) && m.canRebase():
		return m.previewRebase()
	case key.Matches(msg, m.keys.push) && m.canPush():
		return m.previewPush()
	case key.Matches(msg, m.keys.refresh):
		return m, tea.Batch(m.loadBranch(), m.loadChanges())
	default:
		return m, nil
	}
}

// branchCreator is a branch about to be created, named for the selected issue.
type branchCreator struct {
	marks    glyphs
	styles   styles
	input    textinput.Model
	issue    jira.Issue
	forIssue bool
	base     string
	baseAge  string
	send     sendState
	// skipFetch is set once a fetch has failed and the user chose to branch from
	// what is already there, so the retry does not fetch again.
	skipFetch bool
	// fetchProblem is the reason a fetch failed, shown with the offer to branch
	// from what is there anyway.
	fetchProblem error
	// worktree makes the branch in a new worktree beside the repository instead
	// of switching to it in place; canWorktree records that the repository can.
	worktree    bool
	canWorktree bool
}

var _ overlay = branchCreator{}

// openBranchCreator proposes a branch for the selected issue, started from the
// branch work merges into.
func (m Model) openBranchCreator() (Model, tea.Cmd) {
	if m.deps.Git.CreateBranch == nil {
		return m, nil
	}

	issue, forIssue := m.issues.current()

	name := ""
	if forIssue {
		name = m.cfg.Branch.Naming().Name(issue.Type, string(issue.Key), issue.Summary)
	}

	m.overlay = branchCreator{
		marks: m.marks, styles: m.styles, input: newInput(name), issue: issue, forIssue: forIssue,
		base: m.branch.branch.Base, baseAge: m.baseAge(),
		canWorktree: m.deps.Git.CreateWorktree != nil,
	}

	return m, nil
}

// baseAge says how long ago the base last moved, or nothing when git could not
// say — so a branch started from a stale base reads as such.
func (m Model) baseAge() string {
	if m.branch.branch.BaseUpdated.IsZero() {
		return ""
	}

	return age(m.deps.now(), m.branch.branch.BaseUpdated)
}

// view shows the name and where the branch will start.
func (c branchCreator) view(width, _ int) (string, string) {
	c.input.SetWidth(max(1, width-len(c.input.Prompt)-1))

	lines := pinnedOutcome(c.styles, c.marks, c.send, "creating", width)
	if c.forIssue {
		lines = append(lines, "for "+string(c.issue.Key)+" "+c.issue.Summary, "")
	}

	lines = append(lines, c.input.View(), "", c.start())

	if c.worktree {
		lines = append(lines, "as a worktree beside the repository")
	}

	if c.fetchProblem != nil {
		lines = append(lines, "", failedGlyph(c.styles, c.marks)+
			" could not fetch; enter branches from what you already have")
	}

	return c.title(), strings.Join(lines, "\n")
}

// title names the creator for the issue it is for, when it is for one.
func (c branchCreator) title() string {
	if c.forIssue {
		return "New branch for " + string(c.issue.Key)
	}

	return "New branch"
}

// start says where the branch will begin, and how old that base is.
func (c branchCreator) start() string {
	if c.base == "" {
		return "from the current commit (no default branch found)"
	}

	if c.baseAge == "" {
		return "from " + c.base
	}

	return "from " + c.base + ", fetched " + c.baseAge
}

// footer offers creating the branch or not, and switching between a branch here
// and a worktree beside the repository where that is possible.
func (c branchCreator) footer(keys keyMap) []key.Binding {
	if c.send.sending {
		return []key.Binding{keys.interrupt}
	}

	create := "create"

	switch {
	case c.fetchProblem != nil:
		create = "branch from what you have"
	case c.worktree:
		create = "create worktree"
	}

	bindings := []key.Binding{relabel(keys.confirm, create)}
	if c.canWorktree {
		bindings = append(bindings, relabel(keys.worktree, c.worktreeToggleLabel()))
	}

	return append(bindings, relabel(keys.closeOverlay, "discard"))
}

// worktreeToggleLabel names what the worktree key would switch to.
func (c branchCreator) worktreeToggleLabel() string {
	if c.worktree {
		return "branch here instead"
	}

	return "as a worktree"
}

// handleKey answers a key while the branch is named. Every key typed checks the
// name, so a name git would refuse says so before enter is pressed.
func (c branchCreator) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case c.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.worktree) && c.canWorktree:
		c.worktree = !c.worktree
		m.overlay = c

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return c.create(m)
	}

	c.input, _ = c.input.Update(msg)
	c.send.err = convention.ValidateBranchName(c.input.Value())
	m.overlay = c

	return m, nil
}

// create creates and switches to the branch.
func (c branchCreator) create(m Model) (Model, tea.Cmd) {
	name := strings.TrimSpace(c.input.Value())

	c.send.err = convention.ValidateBranchName(name)
	if c.send.err != nil {
		m.overlay = c

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would " + c.dryRunAction() + name + " " + c.start()), nil
	}

	c.send, c.fetchProblem = starting(), nil
	m.overlay = c

	if c.willFetch(m) {
		fetch := m.deps.Git.Fetch

		return m, func() tea.Msg { return fetched{name: name, err: fetch()} }
	}

	return m, c.createCommand(m, name)
}

// dryRunAction names what a dry run would do: a fetch when there is a base to
// refresh, then a branch in place or a worktree beside the repository.
func (c branchCreator) dryRunAction() string {
	verb := "create "
	if c.worktree {
		verb = "create a worktree for "
	}

	if c.willFetchBase() {
		return "fetch origin, then " + verb
	}

	return verb
}

// willFetch reports that create should fetch first: there is a base to refresh,
// a fetch seam to do it, and the user has not already chosen to skip it.
func (c branchCreator) willFetch(m Model) bool {
	return c.willFetchBase() && m.deps.Git.Fetch != nil && !c.skipFetch
}

// willFetchBase reports that there is a base worth fetching.
func (c branchCreator) willFetchBase() bool {
	return c.base != "" && !c.skipFetch
}
