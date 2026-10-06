// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
)

// deepestHeading is the most # a Markdown heading opens with.
const deepestHeading = 6

// discordHeadings is how deep a heading Discord shows as one.
const discordHeadings = 3

// listItem opens an item of a Markdown list.
const listItem = "- "

// blockMarkup is how a service shows the blocks of a Markdown text: its
// headings, a list's items, and the escaping every line takes first.
type blockMarkup struct {
	escape  func(string) string
	heading func(level int, text string) string
	item    func(text string) string
}

// RenderMarkdown is Markdown text — the Summary as it is copied — as kind
// shows it: Slack's mrkdwn has no headings or lists, so a heading is a bold
// line and an item a bullet, every line escaped as an announcement's values
// are; a Teams webhook's text shows no heading, so it is bold there too;
// Discord shows three levels and the rest are bold; and a plain webhook,
// which reads no markup, has a heading's words alone. An empty or unknown
// kind renders for Slack.
func RenderMarkdown(kind config.MessagingKind, text string) string {
	markup := blockMarkupFor(kind)
	lines := strings.Split(text, "\n")

	for index, line := range lines {
		lines[index] = markup.line(line)
	}

	return strings.Join(lines, "\n")
}

// line is one line of Markdown as the service shows it.
func (b blockMarkup) line(line string) string {
	line = b.escape(line)

	if level, text, isHeading := heading(line); isHeading {
		return b.heading(level, text)
	}

	if text, isItem := strings.CutPrefix(line, listItem); isItem {
		return b.item(text)
	}

	return line
}

// heading is a Markdown heading's level and words: one to six # and a space.
func heading(line string) (int, string, bool) {
	marks := len(line) - len(strings.TrimLeft(line, "#"))
	text, spaced := strings.CutPrefix(line[marks:], " ")

	return marks, text, spaced && marks >= 1 && marks <= deepestHeading
}

// blockMarkupFor is the block markup for kind.
func blockMarkupFor(kind config.MessagingKind) blockMarkup {
	switch kind {
	case config.KindTeams:
		return blockMarkup{escape: keepText, heading: markdownBold, item: markdownItem}
	case config.KindDiscord:
		return blockMarkup{escape: keepText, heading: discordHeading, item: markdownItem}
	case config.KindWebhook:
		return blockMarkup{escape: keepText, heading: headingWords, item: markdownItem}
	case config.KindSlack:
		return slackBlocks()
	default:
		return slackBlocks()
	}
}

// slackBlocks is Slack's mrkdwn: a heading as a bold line, an item as a
// bullet, and &, < and > escaped so a title cannot ping a channel.
func slackBlocks() blockMarkup {
	return blockMarkup{
		escape:  slackEscape,
		heading: func(_ int, text string) string { return "*" + text + "*" },
		item:    func(text string) string { return "• " + text },
	}
}

// markdownBold is a heading as a bold line of Markdown.
func markdownBold(_ int, text string) string { return "**" + text + "**" }

// discordHeading is a heading Discord shows as one, or a bold line deeper.
func discordHeading(level int, text string) string {
	if level > discordHeadings {
		return markdownBold(level, text)
	}

	return strings.Repeat("#", level) + " " + text
}

// headingWords is a heading's words alone.
func headingWords(_ int, text string) string { return text }

// markdownItem is a list's item as Markdown writes it.
func markdownItem(text string) string { return listItem + text }
