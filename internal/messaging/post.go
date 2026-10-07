// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// postMessagePath posts as the user the token belongs to.
const postMessagePath = "/chat.postMessage"

// jsonContent is the media type both transports take. Slack's Web API asks for
// the charset to be named.
const jsonContent = "application/json; charset=utf-8"

// ErrInsecureWebhook reports a webhook that is not an https URL. The URL is the
// credential, and it is not sent in the clear.
var ErrInsecureWebhook = errors.New("messaging.webhook_url is not an https URL")

// userMessage is what chat.postMessage takes. Unfurling is off: a preview of the
// pull request would bury the message under it.
type userMessage struct {
	Channel     string `json:"channel"`
	Text        string `json:"text"`
	UnfurlLinks bool   `json:"unfurl_links"`
	UnfurlMedia bool   `json:"unfurl_media"`
}

// webhookMessage is what a Slack, Teams or plain incoming webhook takes; its
// channel is its own.
type webhookMessage struct {
	Text string `json:"text"`
}

// discordMessage is what a Discord webhook takes: the body key is "content"
// rather than "text", and allowed_mentions is pinned to parse nothing.
type discordMessage struct {
	Content string `json:"content"`
	// AllowedMentions tells Discord which mentions in Content to resolve.
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

// allowedMentions with an empty (non-nil) Parse tells Discord to ping no one,
// whatever the content says. Escaping alone cannot stop this: Discord parses
// "@everyone", "@here" and "<@id>" out of the raw content regardless of any
// surrounding Markdown, so a hostile PR title or issue summary would otherwise
// ping the whole server. The empty slice must serialize as [] (parse nothing),
// never null (parse everything).
type allowedMentions struct {
	Parse []string `json:"parse"`
}

// webhookBody wraps text in the JSON body the kind's webhook expects. Discord
// names the field "content" and needs its mentions pinned off; Slack, Teams and
// a plain webhook name it "text".
func webhookBody(kind config.MessagingKind, text string) any {
	if kind == config.KindDiscord {
		return discordMessage{Content: text, AllowedMentions: allowedMentions{Parse: []string{}}}
	}

	return webhookMessage{Text: text}
}

// verdict is chat.postMessage's answer.
type verdict struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// Post sends text to the configured messaging service: to channel when it is a
// Slack user-token post naming one, or to the configured default otherwise. A webhook
// carries its own channel, so channel does not apply to it.
func (c Client) Post(ctx context.Context, channel, text string) error {
	//nolint:exhaustive // MessagingNone has no transport by design; its lookup miss is the not-configured path.
	transports := map[config.MessagingMode]func(context.Context, string, string) error{
		config.MessagingUser:    c.postAsUser,
		config.MessagingWebhook: c.postToWebhook,
	}

	post, configured := transports[c.creds.Mode()]
	if !configured {
		return ErrNoCredential
	}

	return post(ctx, channel, text)
}

// postAsUser posts through chat.postMessage, as the user the token belongs to,
// with a newer token when Slack calls the first one expired.
func (c Client) postAsUser(ctx context.Context, channel, text string) error {
	if channel == "" {
		channel = c.creds.Channel
	}

	return c.withFreshToken(ctx, func(token config.Secret) error {
		return c.postMessage(ctx, token, channel, text)
	})
}

// postMessage posts text to channel with token. Slack answers a refusal with
// 200 and ok:false, so the status alone says nothing.
func (c Client) postMessage(ctx context.Context, token config.Secret, channel, text string) error {
	message := userMessage{Channel: channel, Text: text, UnfurlLinks: false, UnfurlMedia: false}
	header := http.Header{"Authorization": {"Bearer " + token.Reveal()}}

	body, err := c.postJSON(ctx, c.base+postMessagePath, message, header)
	if err != nil {
		return err
	}

	var answer verdict

	err = json.Unmarshal(sanitize.JSON(body), &answer)
	if err != nil {
		return fmt.Errorf("reading the answer from Slack: %w", err)
	}

	if !answer.OK {
		return refusal(answer.Error, channel)
	}

	return nil
}

// refusal is Slack's ok:false to a post: a credential it would not take, which
// is ErrRejected as auth.test's own no is, or a message it would not deliver.
func refusal(code, channel string) error {
	if slices.Contains(credentialCodes(), code) {
		return credentialRefusal(code)
	}

	return PostRefusedError{Reason: rejectionReason(code, channel)}
}

// PostRefusedError is a message Slack would not deliver, with the reason: the
// fix for its channel, or Slack's own code for one this does not explain. The
// reason names a channel and a code, never an address, so every surface can
// show it.
type PostRefusedError struct {
	Reason string
}

var _ error = PostRefusedError{}

// Error says the message was refused, and why.
func (e PostRefusedError) Error() string {
	return ErrPostRefused.Error() + ": " + e.Reason
}

// Unwrap lets errors.Is match ErrPostRefused.
func (PostRefusedError) Unwrap() error {
	return ErrPostRefused
}

// credentialRefusal is Slack turning a token down with code: ErrRejected, and
// ErrTokenExpired as well when the token has only run out of time.
func credentialRefusal(code string) error {
	if code == "token_expired" {
		return fmt.Errorf("%w: %w: %s", ErrRejected, ErrTokenExpired, code)
	}

	return fmt.Errorf("%w: %s", ErrRejected, code)
}

// credentialCodes are the error codes Slack answers a post with when the fault
// is the token or the app it belongs to, not the message or its channel.
func credentialCodes() []string {
	return []string{"not_authed", "invalid_auth", "account_inactive", "token_revoked", "token_expired", "missing_scope"}
}

// postToWebhook posts to an incoming webhook. Each kind answers a delivered post
// with its own 2xx: Slack 200 with "ok" as plain text, Discord 204 with no body,
// Teams 200 or 202, and a plain webhook whichever 2xx it chooses. A refusal is a
// status with its reason as plain text.
func (c Client) postToWebhook(ctx context.Context, _, text string) error {
	address, err := url.Parse(c.creds.WebhookURL.Reveal())
	if err != nil || address.Scheme != "https" || address.Host == "" {
		// Unwrapped, whatever went wrong: a parse error quotes the URL.
		return ErrInsecureWebhook
	}

	_, err = c.postJSON(ctx, address.String(), webhookBody(c.creds.Kind, text), http.Header{})

	return err
}

// postJSON posts payload as JSON with the given headers, and returns the body of
// an accepted answer.
func (c Client) postJSON(ctx context.Context, address string, payload any, header http.Header) ([]byte, error) {
	// Trade-off TRADE-13: every payload is one of this package's message
	// types, which always encode.
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding the message: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(encoded))
	if err != nil {
		// Unwrapped: the error quotes the address, which may be the webhook.
		return nil, fmt.Errorf("%w: building the request", ErrUnreachable)
	}

	request.Header = header
	request.Header.Set("Content-Type", jsonContent)

	return c.deliver(request)
}

// rejectionReason turns Slack's error code into a sentence that names the fix,
// falling back to the code for one it does not explain. It names no key to
// press: a command line has none, and an overlay's footer offers its own.
func rejectionReason(code, channel string) string {
	if channel == "" {
		channel = "the channel"
	}

	explained := map[string]string{
		"not_in_channel":    "you are not in " + channel + "; join it",
		"channel_not_found": "there is no channel " + channel + ", or you cannot see it; check the channel name",
		"is_archived":       channel + " is archived; choose an open channel",
	}

	if sentence, known := explained[code]; known {
		return sentence
	}

	return code
}

// deliver performs a post and returns the body of any 2xx answer. Neither error
// names the address: for a webhook, the address is the credential.
func (c Client) deliver(request *http.Request) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, httpx.Unreachable(ErrUnreachable, "", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.creds.Service(), err)
	}

	switch {
	case response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices:
		return body, nil
	case response.StatusCode == http.StatusTooManyRequests:
		return nil, httpx.RateLimited(response.Header)
	case response.StatusCode >= http.StatusBadRequest && response.StatusCode < http.StatusInternalServerError:
		return nil, fmt.Errorf("%w: %s", ErrRejected, strings.TrimSpace(sanitize.Text(string(body))))
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}
}

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
		return markup{link: markdownLink, escape: markdownEscape}
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
		return markdownEscape(title)
	}

	return "[" + markdownEscape(title) + "](" + strings.ReplaceAll(url, ")", "%29") + ")"
}

// plainLink renders the title followed by its bare URL, for a plain webhook that
// reads no markup but will usually auto-link a URL of its own accord.
func plainLink(url, title string) string {
	return title + " " + url
}

// markdownEscape backslash-escapes the Markdown metacharacters, so a value
// anyone can write is shown literally rather than read as emphasis, code or a
// link. It is the Markdown counterpart to slackEscape.
func markdownEscape(text string) string {
	return strings.NewReplacer(
		`\`, `\\`, "`", "\\`", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "*", "\\*", "_", "\\_",
		"~", "\\~", "|", "\\|", "#", "\\#", ">", "\\>",
	).Replace(text)
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
