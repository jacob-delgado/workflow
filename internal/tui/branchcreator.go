// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// branchCreator is a branch about to be created, named for the selected issue.
type branchCreator struct {
	input    textinput.Model
	issue    jira.Issue
	forIssue bool
	base     string
	// remote is the base remote, which a start fetches before branching.
	remote  string
	baseAge string
	plan    basePlan
	// worktree makes the branch in a new worktree beside the repository instead
	// of switching to it in place; canWorktree records that the repository can.
	worktree    bool
	canWorktree bool
	// applyKey is the key apply is bound to, which the offer after a failed
	// fetch names.
	applyKey string
	// opened is the round the creator opened in, which its fetch and its create
	// carry back.
	opened int
	// send is the one write out, the fetch or the create, and step which of
	// the two it is or was refused at; step means something only while send
	// is not idle.
	send send.State
	step creatorStep
	// problem is why the name typed would not do, found as it is typed.
	problem error
}

var (
	_ sender[branchCreator] = branchCreator{}
	_ pasteable             = branchCreator{}
)

// creatorStep is which of the creator's writes is out, or was refused.
type creatorStep int

const (
	// stepFetch fetches the base's remote before branching.
	stepFetch creatorStep = iota
	// stepCreate makes the branch, or the worktree.
	stepCreate
)

// basePlan is what the creator does with the base before it branches.
type basePlan int

const (
	// baseFetchFirst fetches the base's remote, then branches from the base.
	baseFetchFirst basePlan = iota
	// baseAsIs branches from the base as the repository already has it: the
	// fetch failed, and the user chose to branch from what is there.
	baseAsIs
	// baseNone has no base to start from: no default branch was found, so the
	// branch starts from the current commit.
	baseNone
)

// basePlanFor is how a branch begins from base: fetched first, or, with no
// base, from the current commit.
func basePlanFor(base string) basePlan {
	if base == "" {
		return baseNone
	}

	return baseFetchFirst
}

// openBranchCreator proposes a branch for the selected issue, started from the
// branch work merges into.
func (m Model) openBranchCreator() (Model, tea.Cmd) {
	issue, forIssue := m.issues.current()

	name := ""
	if forIssue {
		name = m.cfg.Branch.Naming().Name(issue.Type, string(issue.Key), issue.Summary)
	}

	m, opened := m.nextRound()
	m.overlay = branchCreator{
		input: newInput(name), issue: issue, forIssue: forIssue,
		base: m.branch.checkedOut().Base, remote: m.branch.checkedOut().BaseRemote, baseAge: m.branch.baseAge(m.deps.now()),
		canWorktree: m.deps.Git.CreateWorktree != nil, applyKey: m.keys.confirm.Help().Key,
		plan: basePlanFor(m.branch.checkedOut().Base), opened: opened,
	}

	return m, nil
}

// baseAge says how long ago the base last moved, or nothing when git could not
// say — so a branch started from a stale base reads as such.
func (s branchState) baseAge(now time.Time) string {
	if s.checkedOut().BaseUpdated.IsZero() {
		return ""
	}

	return age(now, s.checkedOut().BaseUpdated)
}

// view shows the name and where the branch will start.
func (c branchCreator) view(kit renderKit, width, _ int) (string, string) {
	c.input.SetWidth(max(1, width-len(c.input.Prompt)-1))

	lines := c.pinned(kit, width)
	if c.forIssue {
		lines = append(lines, "for "+shownKey(c.issue.Key)+" "+c.issue.Summary, "")
	}

	lines = append(lines, c.input.View(), "", c.start())

	if c.worktree {
		lines = append(lines, "as a worktree beside the repository")
	}

	refusal := c.fetchRefusal()
	if refusal != nil {
		lines = append(lines, "", kit.failureLine(refusal),
			"could not fetch; "+c.applyKey+" branches from what you already have")
	}

	return c.title(), strings.Join(lines, "\n")
}

// pinned is what the creator says above the name: why the name typed would
// not do, or how its write stands, creating or refused by git. No name with
// a problem is sent, so none is out while there is one.
func (c branchCreator) pinned(kit renderKit, width int) []string {
	if c.problem != nil {
		return kit.pinnedProblem(c.problem, width)
	}

	return kit.pinnedSend(c.pinnedWrite(), "creating", width)
}

// pinnedWrite is the write pinned draws: the creator's, but idle once the
// fetch is refused, whose refusal the body draws beside its offer.
func (c branchCreator) pinnedWrite() send.State {
	if c.fetchRefusal() != nil {
		return send.State{}
	}

	return c.send
}

// fetchRefusal is why git refused the fetch, while the creator holds that
// refusal; nil otherwise. Its refusal of the create is pinned instead.
func (c branchCreator) fetchRefusal() error {
	if c.step != stepFetch {
		return nil
	}

	return c.send.Reason()
}

// title names the creator for the issue it is for, when it is for one.
func (c branchCreator) title() string {
	if c.forIssue {
		return "Start work on " + shownKey(c.issue.Key)
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
	if c.send.Stage() == send.StageInFlight {
		return []key.Binding{keys.interrupt}
	}

	create := "create"

	switch {
	case c.fetchRefusal() != nil:
		create = "branch from what you have"
	case c.worktree:
		create = "create worktree"
	}

	bindings := []key.Binding{relabel(keys.confirm, create)}
	if c.canWorktree {
		bindings = append(bindings, relabel(keys.worktree, c.worktreeToggleLabel()))
	}

	return append(bindings, relabel(keys.closeOverlay, escDiscard))
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
	case c.send.Stage() == send.StageInFlight:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.worktree) && c.canWorktree:
		m.overlay = c.toggledWorktree()

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return c.create(m)
	}

	c.input, _ = c.input.Update(msg)
	m.overlay = c.renamed()

	return m, nil
}

// renamed is the creator once the name typed changes: checked as it is
// typed, so a name git would refuse says so before enter is pressed, and
// rid of git's refusal of the create, which was of the name before.
func (c branchCreator) renamed() branchCreator {
	c.problem = convention.ValidateBranchName(c.input.Value())

	return c.withoutCreateRefusal()
}

// toggledWorktree is the creator making the other of a branch here and a
// worktree beside the repository, rid of git's refusal to create the one it
// no longer makes.
func (c branchCreator) toggledWorktree() branchCreator {
	c.worktree = !c.worktree

	return c.withoutCreateRefusal()
}

// withoutCreateRefusal is the creator rid of git's refusal of its create,
// once what it would create has changed. A fetch's refusal stays, with its
// offer to branch from what is there, which still holds.
func (c branchCreator) withoutCreateRefusal() branchCreator {
	if c.step == stepCreate {
		c.send = c.send.Cleared()
	}

	return c
}

// which names the branch creator.
func (branchCreator) which() overlayKind { return overlayBranchCreator }

// openedAs is the round the creator opened in.
func (c branchCreator) openedAs() int { return c.opened }

// sent is the creator's write.
func (c branchCreator) sent() send.State { return c.send }

// withSend is the creator with state as its write.
func (c branchCreator) withSend(state send.State) branchCreator {
	c.send = state

	return c
}

// sentAt is the creator with step's write out. It takes no key while one is,
// so none is out already.
func (c branchCreator) sentAt(step creatorStep) branchCreator {
	c.send, _ = c.send.Sent()
	c.step = step

	return c
}

// pasted types a paste into the branch name, which is checked as typing is.
func (c branchCreator) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if c.send.Stage() == send.StageInFlight {
		return m, nil
	}

	c.input, _ = c.input.Update(paste)
	m.overlay = c.renamed()

	return m, nil
}

// create creates and switches to the branch, fetching the base first where
// the plan is to. Enter on the offer a failed fetch makes branches from what
// is there, so the plan no longer fetches.
func (c branchCreator) create(m Model) (Model, tea.Cmd) {
	name := strings.TrimSpace(c.input.Value())

	c.problem = convention.ValidateBranchName(name)
	if c.problem != nil {
		m.overlay = c

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed(c.dryRunNotice(name)), nil
	}

	if c.fetchRefusal() != nil {
		c.plan = baseAsIs
	}

	if c.willFetch(m) {
		ctx, fetch, round := m.session, m.deps.Git.Fetch, c.opened
		m.overlay = c.sentAt(stepFetch)

		return m, func() tea.Msg { return fetched{name: name, round: round, err: fetch(ctx)} }
	}

	return c.sentAt(stepCreate).creating(m, name)
}

// dryRunNotice says what creating name would do: a fetch when there is a base
// to refresh, then a branch it switches to, as the live path does, or a
// worktree beside the repository, which leaves the checkout as it is.
func (c branchCreator) dryRunNotice(name string) string {
	made := "create " + name + " " + c.start() + " and switch to it"
	if c.worktree {
		made = "create a worktree for " + name + " " + c.start()
	}

	if c.plan == baseFetchFirst {
		made = "fetch " + c.remote + ", then " + made
	}

	return "dry run: would " + made
}

// willFetch reports that create should fetch first: the plan is to, and there
// is a fetch seam to do it.
func (c branchCreator) willFetch(m Model) bool {
	return c.plan == baseFetchFirst && m.deps.Git.Fetch != nil
}
