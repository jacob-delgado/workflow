// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"github.com/jacob-delgado/workflow/internal/config"
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
)

// CommentMarkupOf is the markup a comment is written in under settings, as the
// surface asking reads them now: a Settings save applies to the next comment.
func CommentMarkupOf(settings config.Jira) CommentMarkup {
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
	case MarkupWiki:
		return text
	default:
		return text
	}
}
