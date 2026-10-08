// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errEmptyComment turns back a comment saved empty: guidance, not a failure.
var errEmptyComment = errors.New("nothing to post: the comment was empty")

// commentHelp is what the editor shows below a comment being written. Its last
// sentence names which markup the text is read as, which depends on whether the
// instance is configured to rewrite a Markdown comment before posting it.
const (
	commentHelpWiki = "Write the comment above this line. Save and quit to take it back to the\n" +
		"comment you are writing; it is shown to you before it is posted.\n" +
		"Jira's own markup works here."
	commentHelpMarkdown = "Write the comment above this line. Save and quit to take it back to the\n" +
		"comment you are writing; it is shown to you before it is posted.\n" +
		"Markdown works here; it is converted to Jira's markup when posted."
	commentHelpForge = "Write the comment above this line. Save and quit to take it back to the\n" +
		"comment you are writing; it is shown to you before it is posted.\n" +
		"Markdown works here; the forge renders it."
)

// commentHelp is the editor guidance for the markup a comment is written in.
func commentHelp(markup loop.CommentMarkup) string {
	switch markup {
	case loop.MarkupJiraMarkdown:
		return commentHelpMarkdown
	case loop.MarkupForgeMarkdown:
		return commentHelpForge
	case loop.MarkupWiki:
		return commentHelpWiki
	}

	return commentHelpWiki
}

// commentPreview is a comment about to be posted, shown for a last look:
// nothing outward facing is sent without one. It holds the composer it was
// opened from, so esc goes back to the draft as it was, and the comment is the
// composer's text: post sends what the tracker stores of it, and the preview
// shows that with any terminal control neutralized, as every text the screen
// draws is.
type commentPreview struct {
	composer commentComposer
	send     sendState
}

var _ failable[commentPreview] = commentPreview{}

// stored is the comment as its tracker will store it: what the preview shows
// and what post sends, so the two differ only where the screen must not draw a
// control the text holds.
func (p commentPreview) stored() string {
	return p.composer.markup.Stored(p.composer.text.Value())
}

// view shows the comment as it will be stored, its outcome pinned under the
// title so a long refusal is seen rather than clipped below the fold.
func (p commentPreview) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedOutcome(p.send, "posting", width)
	issue := p.composer.issue
	lines = append(lines, shownKey(issue.Key)+" "+issue.Summary, "", wrap(sanitize.Text(p.stored()), width))

	return "Comment on " + shownKey(issue.Key), strings.Join(lines, "\n")
}

// footer offers posting, or going back to the draft.
func (p commentPreview) footer(keys keyMap) []key.Binding {
	if p.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "post"), relabel(keys.closeOverlay, escBack)}
}

// handleKey answers a key while the comment is previewed.
func (p commentPreview) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.overlay = p.composer

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return p.post(m)
	default:
		return m, nil
	}
}

// post sends the comment.
func (p commentPreview) post(m Model) (Model, tea.Cmd) {
	issueKey := p.composer.issue.Key
	if m.dryRun {
		m.commentDrafts = m.commentDrafts.keeping(issueKey, p.composer.text.Value())

		return m.closeOverlay().noticed("dry run: would comment on " + shownKey(issueKey)), nil
	}

	p.send = starting()
	m.overlay = p
	comment, text := m.deps.Jira.Comment, p.stored()

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

var _ applier = commentPosted{}

// apply closes the preview once the comment is posted and reads the issue again
// to show it, or keeps the preview open with Jira's reason.
func (msg commentPosted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[commentPreview](m, msg.err), nil
	}

	m.commentDrafts = m.commentDrafts.keeping(msg.issueKey, "")
	m = m.closeOverlay().noticed(m.marks.done + " commented on " + shownKey(msg.issueKey))

	return m.reloadDetail(msg.issueKey)
}

// failed is the preview kept open with the reason, the comment still in it.
func (p commentPreview) failed(err error) commentPreview {
	p.send = p.send.failed(err)

	return p
}

// comments draws the most recent comments, oldest of them first.
func (m Model) comments(detail jira.IssueDetail) []string {
	if detail.CommentTotal == 0 {
		return nil
	}

	shown := detail.Comments[max(0, len(detail.Comments)-cmp.Or(m.cfg.UI.CommentsShown, defaultCommentsShown)):]
	heading := fmt.Sprintf("Comments %s of %s", strconv.Itoa(len(shown)), strconv.Itoa(detail.CommentTotal))
	lines := []string{"", m.styles.strong.Render(heading)}

	for _, comment := range shown {
		lines = append(lines, "",
			m.styles.label.Render(sanitize.Line(comment.Author)+m.marks.separator+age(m.deps.now(), comment.Created)),
			sanitize.Text(comment.Body))
	}

	return lines
}
