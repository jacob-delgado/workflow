// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"net/url"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Moment is the point in a change's life an announcement marks.
type Moment int

const (
	// MomentReady is the default: the change is open and ready for review.
	MomentReady Moment = iota
	// MomentMerged is that the change has merged.
	MomentMerged
	// MomentCIRed is that the change's CI has gone red.
	MomentCIRed
)

// Announcement is the message telling a channel where a change stands.
type Announcement struct {
	Author           string
	PullRequestURL   string
	PullRequestTitle string
	IssueKey         string
	IssueSummary     string
	IssueURL         string
	// Noun is what the forge calls the change — "pull request" or "merge
	// request". Empty defaults to "pull request".
	Noun string
	// Moment is what the message marks: ready for review, merged, or CI red. The
	// zero value is ready for review.
	Moment Moment
	// Kind is the service the message is rendered for: it decides the link markup
	// — Slack's <url|text>, Markdown's [text](url), or a bare URL — and the
	// escaping. Empty renders for Slack.
	Kind config.MessagingKind
	// Template shapes the ready-for-review Slack message from named placeholders —
	// {author}, {noun}, {title}, {url}, {key}, {summary}, {issue_url} — for a team
	// with a house style. Empty, for any other moment, or for a non-Slack kind,
	// uses the built-in text.
	Template string
}

// markup renders a link and escapes text the way one messaging service reads
// it. Slack uses its own mrkdwn (<url|text>) and must escape &, < and >; Teams
// and Discord use Markdown links and read those characters literally; a plain
// webhook shows a bare URL.
type markup struct {
	link   func(url, title string) string
	escape func(string) string
}

// markupFor is the markup for kind. An empty or unknown kind renders for Slack.
func markupFor(kind config.MessagingKind) markup {
	switch kind {
	case config.KindTeams, config.KindDiscord:
		return markup{link: markdownLink, escape: sanitize.EscapeMarkdown}
	case config.KindWebhook:
		return markup{link: plainLink, escape: keepText}
	case config.KindSlack:
		return slackMarkup()
	default:
		return slackMarkup()
	}
}

// slackMarkup renders Slack mrkdwn: a <url|text> link with every value escaped
// so a hostile title cannot inject markup or ping the whole channel.
func slackMarkup() markup {
	return markup{
		link:   func(url, title string) string { return "<" + slackEscape(url) + "|" + slackEscape(title) + ">" },
		escape: slackEscape,
	}
}

// markdownLink renders a Markdown link, for Teams and Discord. Both halves come
// from the forge and are anyone's to write, so the title is escaped and the URL
// has its ")" percent-encoded, closing the two ways a value could otherwise end
// the link early and inject its own markup. A URL that is not http(s) — a scheme
// the forge should never return — is dropped to the escaped title alone rather
// than trusted as a link target.
func markdownLink(url, title string) string {
	if !isWebURL(url) {
		return sanitize.EscapeMarkdown(title)
	}

	return "[" + sanitize.EscapeMarkdown(title) + "](" + strings.ReplaceAll(url, ")", "%29") + ")"
}

// plainLink renders the title followed by its bare URL, for a plain webhook that
// reads no markup but will usually auto-link a URL of its own accord.
func plainLink(url, title string) string {
	return title + " " + url
}

// keepText is the identity escape, for the plain webhook kind alone: it is plain
// text, assumed to read no markup, so there is nothing to neutralize.
func keepText(text string) string {
	return text
}

// isWebURL reports whether raw is an absolute http or https URL with a host —
// the only shape safe to place in a Markdown link target.
func isWebURL(raw string) bool {
	parsed, err := url.Parse(raw)

	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// Text is the announcement rendered for its kind. Every substituted value is
// escaped for the kind's markup — Slack mrkdwn, or Markdown for Teams and
// Discord — and a plain webhook, which reads no markup, takes it as written: a
// title is anyone's to write, and unescaped, "<!channel>" in one pings a whole
// Slack channel, and a ">" ends a link early.
func (a Announcement) Text() string {
	renderer := markupFor(a.Kind)

	// The template is Slack mrkdwn, so it shapes a Slack announcement only.
	if a.Template != "" && a.Moment == MomentReady && (a.Kind == "" || a.Kind == config.KindSlack) {
		return a.rendered(renderer)
	}

	return a.defaultText(renderer)
}

// rendered fills the configured Slack template. Every value is escaped so a
// value anyone can write cannot break out of the template, while the template's
// own characters — the team's markup — pass through as written.
func (a Announcement) rendered(renderer markup) string {
	return strings.NewReplacer(
		"{author}", renderer.escape(a.Author),
		"{noun}", renderer.escape(a.noun()),
		"{title}", renderer.escape(a.PullRequestTitle),
		"{url}", renderer.escape(a.PullRequestURL),
		"{key}", renderer.escape(a.IssueKey),
		"{summary}", renderer.escape(a.IssueSummary),
		"{issue_url}", renderer.escape(a.IssueURL),
	).Replace(a.Template)
}

// defaultText is the built-in announcement: the moment's lead sentence, linked,
// and the issue it is for, linked where there is a link.
func (a Announcement) defaultText(renderer markup) string {
	lead := a.lead(renderer, renderer.link(a.PullRequestURL, a.PullRequestTitle))
	if a.IssueKey == "" {
		return lead
	}

	issue := renderer.escape(a.IssueKey)
	if a.IssueURL != "" {
		issue = renderer.link(a.IssueURL, a.IssueKey)
	}

	return lead + "\n" + issue + " " + renderer.escape(a.IssueSummary)
}

// lead is the moment's opening sentence: what happened to the change, linked.
func (a Announcement) lead(renderer markup, link string) string {
	switch a.Moment {
	case MomentMerged:
		if a.Author != "" {
			return renderer.escape(a.Author) + " merged a " + a.noun() + ": " + link
		}

		return "A " + a.noun() + " merged: " + link
	case MomentCIRed:
		return "CI is red on the " + a.noun() + ": " + link
	case MomentReady:
	}

	if a.Author != "" {
		return renderer.escape(a.Author) + " opened a " + a.noun() + ": " + link
	}

	return "A " + a.noun() + " is ready for review: " + link
}

// noun is what the forge calls the change, defaulting to "pull request".
func (a Announcement) noun() string {
	if a.Noun == "" {
		return "pull request"
	}

	return a.Noun
}

// slackEscape writes text so Slack shows it rather than reading it as markup:
// the three characters its formatting uses, and nothing else.
func slackEscape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}
