// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// CommentMarkup is what a comment on an issue is written in, and so what it is
// posted as.
type CommentMarkup int

const (
	// MarkupWiki is Jira's own wiki markup, posted as typed.
	MarkupWiki CommentMarkup = iota + 1
	// MarkupJiraMarkdown is Markdown, posted to Jira as the wiki markup
	// jira.WikiFromMarkdown makes of it.
	MarkupJiraMarkdown
	// MarkupForgeMarkdown is Markdown posted to a forge as typed, since the
	// forge renders Markdown itself.
	MarkupForgeMarkdown
)

// CommentMarkupOf is the markup a comment on issueKey is written in: Markdown
// on a forge issue, and on a Jira issue as settings say, read as the surface
// asking reads them now, so a Settings save applies to the next comment.
func CommentMarkupOf(settings config.Jira, issueKey jira.Key) CommentMarkup {
	if ref, known := convention.RefOf(string(issueKey)); known && ref.Tracker == convention.TrackerForge {
		return MarkupForgeMarkdown
	}

	if settings.MarkdownComments {
		return MarkupJiraMarkdown
	}

	return MarkupWiki
}

// Stored is text as its tracker will store it, which is what a surface shows
// for a last look and then posts unchanged.
func (m CommentMarkup) Stored(text string) string {
	switch m {
	case MarkupJiraMarkdown:
		return jira.WikiFromMarkdown(text)
	case MarkupWiki, MarkupForgeMarkdown:
		return text
	}

	return text
}
