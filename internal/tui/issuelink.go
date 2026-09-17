// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// issueLinker offers to record a just-opened pull request as a web link on the
// branch's issue, so the team that watches Jira sees it. It is previewed and
// confirmed, like the comment it is.
type issueLinker struct {
	marks    glyphs
	styles   styles
	vocab    reviewVocab
	issueKey string
	pull     forge.PullRequest
	sending  bool
	problem  error
}

var _ overlay = issueLinker{}

// issueToLink is the issue a just-opened pull request should be linked to: the
// branch's issue, when Jira can take the link. Empty when there is neither.
func (m Model) issueToLink() string {
	issueKey, named := convention.IssueKey(m.branch.branch.Name)
	if !named || m.deps.Jira.LinkPullRequest == nil {
		return ""
	}

	return issueKey
}

// view previews the link the confirmation would add.
func (l issueLinker) view(width, _ int) (string, string) {
	lines := pinnedOutcome(l.styles, l.marks, l.sending, "linking", l.problem, width)
	lines = append(lines,
		"Add this "+l.vocab.noun+"'s link to "+l.issueKey+"?", "",
		l.vocab.sigil+strconv.Itoa(l.pull.Number)+" "+l.pull.Title, l.pull.URL)

	return "Link on " + l.issueKey, strings.Join(lines, "\n")
}

// footer offers linking the pull request or skipping it.
func (l issueLinker) footer(keys keyMap) []key.Binding {
	if l.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "link"), relabel(keys.closeOverlay, "skip")}
}

// handleKey links the pull request, or skips it, leaving it open in review.
func (l issueLinker) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case l.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return l.link(m)
	default:
		return m, nil
	}
}

// link adds the pull request's web link to the issue.
func (l issueLinker) link(m Model) (Model, tea.Cmd) {
	l.sending, l.problem = true, nil
	m.overlay = l

	add := m.deps.Jira.LinkPullRequest
	issueKey, pull := l.issueKey, l.pull

	return m, func() tea.Msg {
		return issueLinked{issueKey: issueKey, pull: pull, err: add(issueKey, pull.URL, pull.Title)}
	}
}

// issueLinked reports how linking the pull request on the issue went.
type issueLinked struct {
	issueKey string
	pull     forge.PullRequest
	err      error
}

// apply reports the link, or keeps the confirmation open with Jira's reason.
func (msg issueLinked) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		linker, open := m.overlay.(issueLinker)
		if open {
			linker.sending, linker.problem = false, msg.err
			m.overlay = linker
		}

		return m, nil
	}

	return m.closeOverlay().noticed(m.marks.done + " linked " + m.vocab.sigil +
		strconv.Itoa(msg.pull.Number) + " on " + msg.issueKey), nil
}
