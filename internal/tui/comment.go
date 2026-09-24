// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errEmptyComment turns back a comment saved empty: guidance, not a failure.
var errEmptyComment = errors.New("nothing to post: the comment was empty")

// commentHelp is what the editor shows below a comment being written. Its last
// sentence names which markup the text is read as, which depends on whether the
// instance is configured to rewrite a Markdown comment before posting it.
const (
	commentHelpWiki = "Write the comment above this line. Save and quit to preview it before it is\n" +
		"posted; leave it empty to post nothing. Jira's own markup works here."
	commentHelpMarkdown = "Write the comment above this line. Save and quit to preview it before it is\n" +
		"posted; leave it empty to post nothing. Markdown works here; it is\n" +
		"converted to Jira's markup when posted."
)

// commentHelp is the editor guidance matching whether the instance rewrites a
// Markdown comment before posting it.
func (m Model) commentHelp() string {
	if m.cfg.Jira.MarkdownComments {
		return commentHelpMarkdown
	}

	return commentHelpWiki
}

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
	return m.deps.Editor.Edit(text, m.commentHelp(), func(edited string, err error) tea.Msg {
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
		if _, editing := m.overlay.(commentPreview); editing {
			return keepOpenWith[commentPreview](m, msg.err), nil
		}

		return m.closeOverlay().noticedFailure(msg.err), nil
	case strings.TrimSpace(msg.text) == "":
		return m.closeOverlay().noticedGuidance(errEmptyComment), nil
	}

	m.overlay = commentPreview{
		marks: m.marks, styles: m.styles, issue: msg.issue, text: msg.text,
		markdown: m.cfg.Jira.MarkdownComments,
	}

	return m, nil
}

// commentPreview is a comment about to be posted. text stays the source the user
// typed — post sends it and a re-edit reopens it — while markdown records
// whether it will be converted to wiki markup, so the preview can show that
// converted form.
type commentPreview struct {
	marks    glyphs
	styles   styles
	issue    jira.Issue
	text     string
	markdown bool
	send     sendState
}

var _ failable[commentPreview] = commentPreview{}

// view shows the comment as it will be stored — the wiki markup when Markdown
// conversion is on, otherwise the text verbatim — its outcome pinned under the
// title so a long refusal is seen rather than clipped below the fold.
func (p commentPreview) view(width, _ int) (string, string) {
	body := p.text
	if p.markdown {
		body = jira.WikiFromMarkdown(p.text)
	}

	lines := pinnedOutcome(p.styles, p.marks, p.send, "posting", width)
	lines = append(lines, string(p.issue.Key)+" "+p.issue.Summary, "", wrap(body, width))

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
		return keepOpenWith[commentPreview](m, msg.err), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " commented on " + string(msg.issueKey))

	return m, m.reloadDetail(msg.issueKey)
}

// failed is the preview kept open with the reason, the comment still in it.
func (p commentPreview) failed(err error) commentPreview {
	p.send = p.send.failed(err)

	return p
}
