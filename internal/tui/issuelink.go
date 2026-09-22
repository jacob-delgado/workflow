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
)

// issueLinker offers to record a just-opened pull request as a web link on the
// branch's issue, so the team that watches Jira sees it. It is previewed and
// confirmed, like the comment it is.
type issueLinker struct {
	marks    glyphs
	styles   styles
	vocab    reviewVocab
	issueKey jira.Key
	pull     forge.PullRequest
	send     sendState
}

var _ overlay = issueLinker{}

// issueToLink is the issue a just-opened pull request should be linked to: the
// branch's issue, when Jira can take the link. Empty when there is neither.
func (m Model) issueToLink() jira.Key {
	issueKey, named := m.branchIssue()
	if !named || m.deps.Jira.LinkPullRequest == nil {
		return ""
	}

	return issueKey
}

// view previews the link the confirmation would add.
func (l issueLinker) view(width, _ int) (string, string) {
	lines := pinnedOutcome(l.styles, l.marks, l.send, "linking", width)
	lines = append(lines,
		"Add this "+l.vocab.noun+"'s link to "+string(l.issueKey)+"?", "",
		l.vocab.sigil+strconv.Itoa(l.pull.Number)+" "+l.pull.Title, l.pull.URL)

	return "Link on " + string(l.issueKey), strings.Join(lines, "\n")
}

// footer offers linking the pull request or skipping it.
func (l issueLinker) footer(keys keyMap) []key.Binding {
	if l.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "link"), relabel(keys.closeOverlay, "skip")}
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

// apply reports the link, or keeps the confirmation open with Jira's reason.
func (msg issueLinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		linker, open := m.overlay.(issueLinker)
		if open {
			linker.send = linker.send.failed(msg.err)
			m.overlay = linker
		}

		return m, nil
	}

	m = m.noticed(m.marks.done + " linked " + m.vocab.sigil +
		strconv.Itoa(msg.pull.Number) + " on " + string(msg.issueKey))

	return m.offerReviewStatus(msg.issueKey)
}
