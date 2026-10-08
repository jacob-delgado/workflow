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
)

// branchCreator is a branch about to be created, named for the selected issue.
type branchCreator struct {
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
	// applyKey is the key apply is bound to, which the offer after a failed
	// fetch names.
	applyKey string
}

var (
	_ failable[branchCreator] = branchCreator{}
	_ pasteable               = branchCreator{}
)

// openBranchCreator proposes a branch for the selected issue, started from the
// branch work merges into.
func (m Model) openBranchCreator() (Model, tea.Cmd) {
	issue, forIssue := m.issues.current()

	name := ""
	if forIssue {
		name = m.cfg.Branch.Naming().Name(issue.Type, string(issue.Key), issue.Summary)
	}

	m.overlay = branchCreator{
		input: newInput(name), issue: issue, forIssue: forIssue,
		base: m.branch.branch.Base, baseAge: m.branch.baseAge(m.deps.now()),
		canWorktree: m.deps.Git.CreateWorktree != nil, applyKey: m.keys.confirm.Help().Key,
	}

	return m, nil
}

// baseAge says how long ago the base last moved, or nothing when git could not
// say — so a branch started from a stale base reads as such.
func (s branchState) baseAge(now time.Time) string {
	if s.branch.BaseUpdated.IsZero() {
		return ""
	}

	return age(now, s.branch.BaseUpdated)
}

// view shows the name and where the branch will start.
func (c branchCreator) view(kit renderKit, width, _ int) (string, string) {
	c.input.SetWidth(max(1, width-len(c.input.Prompt)-1))

	lines := kit.pinnedOutcome(c.send, "creating", width)
	if c.forIssue {
		lines = append(lines, "for "+shownKey(c.issue.Key)+" "+c.issue.Summary, "")
	}

	lines = append(lines, c.input.View(), "", c.start())

	if c.worktree {
		lines = append(lines, "as a worktree beside the repository")
	}

	if c.fetchProblem != nil {
		lines = append(lines, "", kit.failureLine(c.fetchProblem),
			"could not fetch; "+c.applyKey+" branches from what you already have")
	}

	return c.title(), strings.Join(lines, "\n")
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

// pasted types a paste into the branch name, which is checked as typing is.
func (c branchCreator) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if c.send.sending {
		return m, nil
	}

	c.input, _ = c.input.Update(paste)
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
		return m.closeOverlay().noticed(c.dryRunNotice(name)), nil
	}

	c.send, c.fetchProblem = starting(), nil
	m.overlay = c

	if c.willFetch(m) {
		fetch := m.deps.Git.Fetch

		return m, func() tea.Msg { return fetched{name: name, err: fetch()} }
	}

	return m, c.createCommand(m, name)
}

// dryRunNotice says what creating name would do: a fetch when there is a base
// to refresh, then a branch it switches to, as the live path does, or a
// worktree beside the repository, which leaves the checkout as it is.
func (c branchCreator) dryRunNotice(name string) string {
	made := "create " + name + " " + c.start() + " and switch to it"
	if c.worktree {
		made = "create a worktree for " + name + " " + c.start()
	}

	if c.willFetchBase() {
		made = "fetch origin, then " + made
	}

	return "dry run: would " + made
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
