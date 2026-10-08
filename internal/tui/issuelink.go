// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// issueLinker offers to record a just-opened pull request as a web link on the
// branch's issue, so the team that watches Jira sees it. It is previewed and
// confirmed, like the comment it is.
type issueLinker struct {
	vocab    reviewVocab
	issueKey jira.Key
	pull     forge.PullRequest
	send     sendState
}

var _ failable[issueLinker] = issueLinker{}

// jiraIssue is the Jira issue the current branch names, and whether it names
// one: never the bare forge issue number a branch can carry instead, which is
// what a pull request's follow-ups on Jira must not reach.
func (m Model) jiraIssue() (jira.Key, bool) {
	return loop.JiraIssue(m.branch.branch, m.cfg.Jira.Project)
}

// pullOffers are what opening a pull request goes on to offer, in order, for a
// dry run to name: to link it on the branch's Jira issue, when Jira can take
// the link, and to move that issue to the review status, when one is
// configured. None when the branch names no Jira issue.
func (m Model) pullOffers() []string {
	issueKey, named := m.jiraIssue()
	if !named {
		return nil
	}

	var offers []string
	if m.deps.Jira.LinkPullRequest != nil {
		offers = append(offers, "to link it on "+shownKey(issueKey))
	}

	if m.offersReviewStatus() {
		offers = append(offers, "to move "+shownKey(issueKey)+" to "+sanitize.Line(m.cfg.Jira.ReviewStatus))
	}

	return offers
}

// view previews the link the confirmation would add.
func (l issueLinker) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedOutcome(l.send, "linking", width)
	lines = append(lines,
		"Add this "+l.vocab.noun+"'s link to "+shownKey(l.issueKey)+"?", "",
		l.vocab.sigil+strconv.Itoa(l.pull.Number)+" "+l.pull.Title, l.pull.URL)

	return "Link on " + shownKey(l.issueKey), strings.Join(lines, "\n")
}

// footer offers linking the pull request or skipping it.
func (l issueLinker) footer(keys keyMap) []key.Binding {
	if l.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "link"), relabel(keys.closeOverlay, escSkip)}
}

// handleKey links the pull request, or skips it, leaving it open in review.
func (l issueLinker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.offerReviewStatus(l.issueKey)
	case key.Matches(msg, m.keys.confirm):
		return l.link(m)
	default:
		return m, nil
	}
}

// link adds the pull request's web link to the issue.
func (l issueLinker) link(m Model) (Model, tea.Cmd) {
	l.send = starting()
	m.overlay = l

	add := m.deps.Jira.LinkPullRequest
	issueKey, pull := l.issueKey, l.pull

	return m, func() tea.Msg {
		return issueLinked{issueKey: issueKey, pull: pull, err: add(issueKey, pull.URL, pull.Title)}
	}
}

// issueLinked reports how linking the pull request on the issue went.
type issueLinked struct {
	issueKey jira.Key
	pull     forge.PullRequest
	err      error
}

var _ applier = issueLinked{}

// apply reports the link, or keeps the confirmation open with Jira's reason.
func (msg issueLinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[issueLinker](m, msg.err), nil
	}

	m = m.noticed(m.marks.done + " linked " + m.vocab.sigil +
		strconv.Itoa(msg.pull.Number) + " on " + shownKey(msg.issueKey))

	return m.offerReviewStatus(msg.issueKey)
}

// failed is the confirmation kept open with the reason the link failed.
func (l issueLinker) failed(err error) issueLinker {
	l.send = l.send.failed(err)

	return l
}
