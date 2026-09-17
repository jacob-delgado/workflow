// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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

	return s.loaded && s.err == nil && name != "" && name != strings.TrimPrefix(s.branch.Base, "origin/")
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
	case branch.Upstream != "origin/"+branch.Name:
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

	if branchKey, named := convention.IssueKey(branch.Name); named {
		issue, listed := m.issues.find(branchKey)
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

	if m.canPush() {
		keys = append(keys, m.keys.push)
	}

	return keys
}

// canPush reports a branch with something to push.
func (m Model) canPush() bool {
	return m.branch.onFeatureBranch() && !m.branch.branch.Pushed() && len(m.branch.branch.Commits) > 0 &&
		m.deps.Git.Push != nil
}

// previewPush shows what a push would send, so an outward-facing act reachable
// from a single key still gets a last look before it leaves.
func (m Model) previewPush() (Model, tea.Cmd) {
	m.overlay = pushPreview{branch: m.branch.branch.Name, remote: m.branch.remote()}

	return m, nil
}

// remote is the remote the branch's upstream lives on, or origin by default.
func (s branchState) remote() string {
	if before, _, found := strings.Cut(s.branch.Upstream, "/"); found && before != "" {
		return before
	}

	return "origin"
}

// handleBranchKey answers the Branch pane's own keys.
func (m Model) handleBranchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.newBranch):
		return m.openBranchCreator()
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
	problem  error
	sending  bool
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
		name = convention.BranchName(issue.Type, issue.Key, issue.Summary)
	}

	m.overlay = branchCreator{
		marks: m.marks, styles: m.styles, input: newInput(name), issue: issue, forIssue: forIssue,
		base: m.branch.branch.Base, baseAge: m.baseAge(), problem: nil, sending: false,
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
	c.input.Width = max(1, width-len(c.input.Prompt)-1)

	lines := pinnedOutcome(c.styles, c.marks, c.sending, "creating", c.problem, width)
	if c.forIssue {
		lines = append(lines, "for "+c.issue.Key+" "+c.issue.Summary, "")
	}

	lines = append(lines, c.input.View(), "", c.start())

	return c.title(), strings.Join(lines, "\n")
}

// title names the creator for the issue it is for, when it is for one.
func (c branchCreator) title() string {
	if c.forIssue {
		return "New branch for " + c.issue.Key
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

// footer offers creating the branch or not.
func (c branchCreator) footer(keys keyMap) []key.Binding {
	if c.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "create"), relabel(keys.closeOverlay, "discard")}
}

// handleKey answers a key while the branch is named. Every key typed checks the
// name, so a name git would refuse says so before enter is pressed.
func (c branchCreator) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case c.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return c.create(m)
	}

	c.input, _ = c.input.Update(msg)
	c.problem = convention.ValidateBranchName(c.input.Value())
	m.overlay = c

	return m, nil
}

// create creates and switches to the branch.
func (c branchCreator) create(m Model) (Model, tea.Cmd) {
	name := strings.TrimSpace(c.input.Value())

	c.problem = convention.ValidateBranchName(name)
	if c.problem != nil {
		m.overlay = c

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would create " + name + " " + c.start()), nil
	}

	c.sending = true
	m.overlay = c
	createBranch, base := m.deps.Git.CreateBranch, c.base

	return m, func() tea.Msg {
		return branchCreated{name: name, err: createBranch(name, base)}
	}
}

// branchCreated reports how creating a branch went.
type branchCreated struct {
	name string
	err  error
}

// apply switches the panes to the new branch, or keeps the creator open with
// git's reason.
func (msg branchCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		creator, open := m.overlay.(branchCreator)
		if open {
			creator.sending, creator.problem = false, msg.err
			m.overlay = creator
		}

		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " created and switched to " + msg.name)

	return m, tea.Batch(m.loadBranch(), m.loadChanges())
}
