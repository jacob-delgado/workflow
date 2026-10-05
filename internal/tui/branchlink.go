// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errNotAnIssue is text typed into the link form that names no issue.
var errNotAnIssue = errors.New("name an issue: a Jira key like PROJ-7, or a forge number like #42")

// branchLinker links the checked-out branch to an issue by hand, for work
// begun outside workflow on a branch whose name names none. With a pull
// request open from the branch, it shows the description with the issue's
// line added before anything is sent, and sends both together.
type branchLinker struct {
	marks   glyphs
	styles  styles
	vocab   reviewVocab
	branch  string
	pull    forge.PullRequest
	hasPull bool
	input   textinput.Model
	// body is the pull request's description with the issue's line added,
	// once the issue is chosen and the description needs it; empty before.
	body    string
	chosen  jira.Key
	send    sendState
	problem error
}

var (
	_ failable[branchLinker] = branchLinker{}
	_ pasteable              = branchLinker{}
)

// canLinkIssue reports a checked-out branch and a repository that can keep a
// link for it.
func (m Model) canLinkIssue() bool {
	return m.deps.Git.LinkIssue != nil && m.branch.loaded && !m.branch.branch.Detached && m.branch.branch.Name != ""
}

// openBranchLink opens the link form on the checked-out branch, holding the
// issue selected in the Issues list to start with.
func (m Model) openBranchLink() (Model, tea.Cmd) {
	start := ""
	if selected, ok := m.issues.current(); ok {
		start = shownKey(selected.Key)
	}

	// A merged or closed pull request stays out of the form, so linking the
	// branch neither edits its description nor offers it on the issue.
	var pull forge.PullRequest

	hasPull := m.review.found && m.review.pull.IsOpen()
	if hasPull {
		pull = m.review.pull
	}

	m.overlay = branchLinker{
		marks: m.marks, styles: m.styles, vocab: m.vocab, branch: m.branch.branch.Name,
		pull: pull, hasPull: hasPull, input: newInput(start),
	}

	return m, nil
}

// view draws the issue being chosen, or the description it will add itself
// to, and how sending is going.
func (l branchLinker) view(width, _ int) (string, string) {
	l.input.SetWidth(max(1, width-len(l.input.Prompt)-1))

	lines := []string{"Link " + l.branch + " to an issue: a Jira key, or a forge number.", "", l.input.View()}
	if l.body != "" {
		lines = append(lines, "", "Its "+l.vocab.noun+"'s description becomes:", "", l.body)
	}

	lines = append(lines, l.outcome()...)

	return "Link " + l.branch, strings.Join(lines, "\n")
}

// outcome says how sending is going, or why the form cannot send.
func (l branchLinker) outcome() []string {
	switch {
	case l.send.sending:
		return []string{"", "linking" + l.marks.ellipsis}
	case l.send.err != nil:
		return []string{"", failureLine(l.styles, l.marks, l.send.err)}
	case l.problem != nil:
		return []string{"", failureLine(l.styles, l.marks, l.problem)}
	default:
		return nil
	}
}

// footer offers choosing the issue, then linking it, or canceling.
func (l branchLinker) footer(keys keyMap) []key.Binding {
	if l.send.sending {
		return []key.Binding{keys.interrupt}
	}

	confirm := relabel(keys.confirm, "link")
	if l.body != "" {
		confirm = relabel(keys.confirm, "link and update the description")
	}

	return []key.Binding{confirm, relabel(keys.closeOverlay, "cancel")}
}

// handleKey answers a key while the form has the keyboard.
func (l branchLinker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm) && l.body != "":
		return l.link(m)
	case key.Matches(msg, m.keys.confirm):
		return l.choose(m)
	default:
		l.input, _ = l.input.Update(msg)
		l.problem, l.body = nil, ""
		m.overlay = l

		return m, nil
	}
}

// pasted types a paste into the issue key, as typing it would.
func (l branchLinker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if l.send.sending {
		return m, nil
	}

	l.input, _ = l.input.Update(paste)
	l.problem, l.body = nil, ""
	m.overlay = l

	return m, nil
}

// choose reads the issue typed, then shows the description it needs adding
// to, or links at once when there is none to change.
func (l branchLinker) choose(m Model) (Model, tea.Cmd) {
	ref, known := convention.RefOf(strings.TrimSpace(l.input.Value()))
	if !known {
		l.send, l.problem = sendState{}, errNotAnIssue
		m.overlay = l

		return m, nil
	}

	l.chosen = jira.Key(ref.Key)

	// Only a description the forge can edit is shown as about to change.
	if l.hasPull && m.deps.Forge.EditPullRequest != nil {
		if body, changed := convention.WithIssueLine(l.pull.Body, ref.Key, m.issueBrowseURL(l.chosen)); changed {
			l.body = body
			m.overlay = l

			return m, nil
		}
	}

	return l.link(m)
}

// link adds the issue's line to the pull request's description when it was
// shown, then keeps the link, holding both back in a dry run. The edit goes
// first so a forge that refuses it leaves the branch as it was.
func (l branchLinker) link(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would link " + l.branch + " to " + string(l.chosen)), nil
	}

	l.send = starting()
	m.overlay = l
	linkIssue, edit := m.deps.Git.LinkIssue, m.deps.Forge.EditPullRequest
	branch, issueKey, pull, body := l.branch, l.chosen, l.pull, l.body

	return m, func() tea.Msg {
		var err error
		if body != "" {
			_, err = edit(pull, forge.PullRequestEdit{Title: pull.Title, Body: body})
		}

		if err == nil {
			err = linkIssue(branch, string(issueKey))
		}

		return branchLinked{branch: branch, issueKey: issueKey, pull: pull, err: err}
	}
}

// failed pins a refusal in the form, so it is read before the form closes.
func (l branchLinker) failed(err error) branchLinker {
	l.send = l.send.failed(err)

	return l
}

// issueBrowseURL is the issue's page, for the line naming it, or empty when
// the tracker gives none.
func (m Model) issueBrowseURL(issueKey jira.Key) string {
	if m.deps.Jira.BrowseURL == nil {
		return ""
	}

	return m.deps.Jira.BrowseURL(issueKey)
}

// branchLinked reports how linking the branch went.
type branchLinked struct {
	branch   string
	issueKey jira.Key
	pull     forge.PullRequest
	err      error
}

var _ applier = branchLinked{}

// apply keeps the form open with the reason the link failed, or closes it,
// reads the branch again, and — for a Jira issue with a pull request open —
// offers to link the pull request on the issue, as opening one would have.
func (msg branchLinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[branchLinker](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " linked " + msg.branch + " to " + string(msg.issueKey))

	ref, _ := convention.RefOf(string(msg.issueKey))
	if ref.Tracker == convention.TrackerJira && msg.pull.Number != 0 && m.deps.Jira.LinkPullRequest != nil {
		m.overlay = issueLinker{marks: m.marks, styles: m.styles, vocab: m.vocab, issueKey: msg.issueKey, pull: msg.pull}
	}

	return m, m.loadBranch()
}

// reviewIssue is the line naming the issue the branch and its pull request are
// for — by the link the branch was given, its name, or the pull request's own
// words — with its summary when the Issues list holds it; empty when none is
// named.
func (m Model) reviewIssue(pull forge.PullRequest) string {
	ref, _, found := loop.BranchIssue(loop.IssueSource{
		Branch: m.branch.branch.Name, Link: m.branch.branch.IssueLink, Pull: &pull, Project: m.cfg.Jira.Project,
	})
	if !found {
		return ""
	}

	issueKey := jira.Key(ref.Key)
	issue, _ := m.issues.find(issueKey)

	return strings.TrimSpace(m.styles.label.Render("issue  ") + shownKey(issueKey) + " " + issue.Summary)
}
