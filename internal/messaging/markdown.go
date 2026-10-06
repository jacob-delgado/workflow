// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
		return blockMarkup{escape: markdownUnescape, heading: headingWords, item: markdownItem}
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
		escape:  func(line string) string { return slackEscape(markdownUnescape(line)) },
		heading: func(_ int, text string) string { return "*" + text + "*" },
		item:    func(text string) string { return "• " + text },
	}
}

// markdownUnescape drops the backslash from each character a summary's text
// escaped, for a service that reads no Markdown and would show the backslash.
func markdownUnescape(text string) string {
	return strings.NewReplacer(
		`\\`, `\`, "\\`", "`", "\\[", "[", "\\]", "]",
		"\\(", "(", "\\)", ")", "\\*", "*", "\\_", "_",
		"\\~", "~", "\\|", "|", "\\#", "#", "\\>", ">",
	).Replace(text)
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

// ErrTooLong reports a message longer than its service takes.
var ErrTooLong = errors.New("too long")

// The most each service takes in one message, measured as it measures it.
const (
	// discordLimit is Discord's: a message's content is "up to 2000
	// characters" (Create Message and Execute Webhook,
	// https://discord.com/developers/docs/resources/message#create-message).
	discordLimit = 2000
	// slackLimit is Slack's: it "will truncate messages containing more than
	// 40,000 characters" (chat.postMessage, Truncating content,
	// https://api.slack.com/methods/chat.postMessage#truncating); an incoming
	// webhook's message is held to the same.
	slackLimit = 40000
	// teamsLimit is Teams': "The message size limit is 28 KB. When the size
	// exceeds 28 KB, you receive an error" (Create an Incoming Webhook,
	// https://learn.microsoft.com/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook),
	// read as 28,000 bytes of the payload, the stricter reading of KB.
	teamsLimit = 28000
)

// The units a length is counted in.
const (
	unitCharacters = "characters"
	unitBytes      = "bytes"
)

// Length is how long a message is as its service measures it, and the most
// the service takes: Limit is 0 for a plain webhook, which names none.
type Length struct {
	Service string
	Count   int
	Limit   int
	Unit    string
}

// MessageLength is rendered — a message as RenderMarkdown leaves it for kind —
// measured as kind's service measures it: Teams by the bytes of the whole
// payload, as its limit is put; every other service by characters.
func MessageLength(kind config.MessagingKind, rendered string) Length {
	length := Length{Service: kind.Service(), Count: utf8.RuneCountInString(rendered), Unit: unitCharacters}

	switch kind {
	case config.KindDiscord:
		length.Limit = discordLimit
	case config.KindTeams:
		payload, _ := json.Marshal(webhookMessage{Text: rendered}) //nolint:errchkjson // a string field always encodes
		length.Count, length.Limit, length.Unit = len(payload), teamsLimit, unitBytes
	case config.KindWebhook:
	case config.KindSlack:
		length.Limit = slackLimit
	default:
		length.Limit = slackLimit
	}

	return length
}

// Check refuses a message longer than its service takes, saying how long it is
// against what.
func (l Length) Check() error {
	if l.Limit == 0 || l.Count <= l.Limit {
		return nil
	}

	return fmt.Errorf("%w for %s (%s); pick a shorter period", ErrTooLong, l.Service, l)
}

// String is the length against the limit, or alone where there is none.
func (l Length) String() string {
	if l.Limit == 0 {
		return fmt.Sprintf("%d %s", l.Count, l.Unit)
	}

	return fmt.Sprintf("%d of %d %s", l.Count, l.Limit, l.Unit)
}
