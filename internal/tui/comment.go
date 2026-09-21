// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// commentHelp is what the editor shows below a comment being written.
const commentHelp = "Write the comment above this line. Save and quit to preview it before it is\n" +
	"posted; leave it empty to post nothing. Jira's own markup works here."

// startComment hands the user's editor an empty comment on the selected issue.
func (m Model) startComment() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Editor.Edit == nil || m.deps.Jira.Comment == nil {
		return m, nil
	}

	return m, m.editComment(selected, "")
}

// editComment opens the editor on a comment's text.
func (m Model) editComment(issue jira.Issue, text string) tea.Cmd {
	return m.deps.Editor.Edit(text, commentHelp, func(edited string, err error) tea.Msg {
		return commentEdited{issue: issue, text: edited, err: err}
	})
}

// commentEdited is a comment back from the editor.
type commentEdited struct {
	issue jira.Issue
	text  string
	err   error
}

// apply shows the comment for a last look before it is posted. Nothing outward
// facing is sent without one.
func (msg commentEdited) apply(m Model) (Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		// A re-edit that failed keeps the preview it came from, so the comment
		// written the first time is not lost to the editor.
		if preview, editing := m.overlay.(commentPreview); editing {
			preview.send = preview.send.failed(msg.err)
			m.overlay = preview

			return m, nil
		}

		return m.closeOverlay().noticed(m.failure(msg.err)), nil
	case strings.TrimSpace(msg.text) == "":
		return m.closeOverlay().noticed("nothing to post: the comment was empty"), nil
	}

	m.overlay = commentPreview{
		marks: m.marks, styles: m.styles, issue: msg.issue, text: msg.text,
	}

	return m, nil
}

// commentPreview is a comment about to be posted.
type commentPreview struct {
	marks  glyphs
	styles styles
	issue  jira.Issue
	text   string
	send   sendState
}

var _ overlay = commentPreview{}

// view shows the comment as it will be posted, its outcome pinned under the
// title so a long refusal is seen rather than clipped below the fold.
func (p commentPreview) view(width, _ int) (string, string) {
	lines := pinnedOutcome(p.styles, p.marks, p.send, "posting", width)
	lines = append(lines, string(p.issue.Key)+" "+p.issue.Summary, "", wrap(p.text, width))

	return "Comment on " + string(p.issue.Key), strings.Join(lines, "\n")
}

// footer offers posting, another edit, or discarding.
func (p commentPreview) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "post"), keys.edit, relabel(keys.closeOverlay, "discard")}
}

// handleKey answers a key while the comment is previewed.
func (p commentPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay().noticed("comment discarded"), nil
	case key.Matches(msg, m.keys.edit):
		return m, m.editComment(p.issue, p.text)
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	default:
		return m, nil
	}
}

// post sends the comment.
func (p commentPreview) post(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would comment on " + string(p.issue.Key)), nil
	}

	p.send = starting()
	m.overlay = p
	comment, issueKey, text := m.deps.Jira.Comment, p.issue.Key, p.text

	return m, func() tea.Msg {
		_, err := comment(issueKey, text)

		return commentPosted{issueKey: issueKey, err: err}
	}
}

// commentPosted reports how posting a comment went.
type commentPosted struct {
	issueKey jira.Key
	err      error
}

// apply closes the preview once the comment is posted and reads the issue again
// to show it, or keeps the preview open with Jira's reason.
func (msg commentPosted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		preview, open := m.overlay.(commentPreview)
		if open {
			preview.send = preview.send.failed(msg.err)
			m.overlay = preview
		}

		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " commented on " + string(msg.issueKey))

	return m, m.reloadDetail(msg.issueKey)
}
