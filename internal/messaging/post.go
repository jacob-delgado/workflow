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
	"strconv"
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

// PostRefusedError is a message the service would not deliver, with the
// reason: from Slack, the fix for its channel, or Slack's own code for one this
// does not explain; from a webhook, the service's own words, cut short, with
// the webhook masked. The reason never names the webhook, so every surface can
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

	return c.deliver(request, refusedPost)
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

// deliver performs a request and returns the body of any 2xx answer, and a 4xx
// as refused says it is, from its status and reason. Neither error names the
// address: for a webhook, the address is the credential.
func (c Client) deliver(request *http.Request, refused func(status int, reason string) error) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, httpx.Unreachable(ErrUnreachable, "", err)
	}
	defer func() { _ = response.Body.Close() }()

	switch status := response.StatusCode; {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return c.accepted(response.Body)
	case status == http.StatusTooManyRequests:
		return nil, httpx.RateLimited(response.Header)
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return nil, refused(status, c.reasonIn(response.Body))
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}

// accepted reads the body of an answer that accepted the post.
func (c Client) accepted(body io.Reader) ([]byte, error) {
	read, err := httpx.Read(body, bodyLimit)
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.creds.Service(), err)
	}

	return read, nil
}

// reasonLimit bounds how much of a refusal is read for its reason: a
// service's reason is a line, and a page longer than this is none.
const reasonLimit = 512

// reasonIn is the reason a refusal's body gives, cut at reasonLimit, with
// every terminal control taken out and the webhook masked wherever it is
// quoted: a webhook server's error page can quote the request it refused.
func (c Client) reasonIn(body io.Reader) string {
	read, err := io.ReadAll(io.LimitReader(body, reasonLimit))
	if err != nil {
		return ""
	}

	return c.masked(strings.TrimSpace(sanitize.Text(strings.ToValidUTF8(string(read), ""))))
}

// masked is text with the webhook's address, and its path and query alone,
// each put as config.Redact shows it. A path of one slash is no secret, and
// masking it would mask every slash.
func (c Client) masked(text string) string {
	address, err := url.Parse(c.creds.WebhookURL.Reveal())
	if c.creds.WebhookURL == "" || err != nil {
		return text
	}

	for _, secret := range []string{
		c.creds.WebhookURL.Reveal(), address.String(), address.EscapedPath(),
		address.Path, address.RawQuery,
	} {
		if len(secret) > 1 {
			text = strings.ReplaceAll(text, secret, config.Redact(secret))
		}
	}

	return text
}

// refusedPost is a 4xx a post was answered with, and reason, what the answer
// said: a credential not accepted — a 401, a 403, a 404 at an address with
// nothing behind it, or one of Slack's codes for a webhook that no longer
// works — is ErrRejected, and anything else, such as a message too long, is a
// message refused.
func refusedPost(status int, reason string) error {
	credential := status == http.StatusUnauthorized || status == http.StatusForbidden ||
		status == http.StatusNotFound || slices.Contains(webhookCredentialCodes(), reason)
	if credential {
		return fmt.Errorf("%w: %s", ErrRejected, reason)
	}

	if reason == "" {
		reason = "status " + strconv.Itoa(status)
	}

	return PostRefusedError{Reason: reason}
}

// refusedRead is a 4xx a directory read was answered with: a read has no
// message to refuse, so it is the credential the service would not accept.
func refusedRead(_ int, reason string) error {
	return fmt.Errorf("%w: %s", ErrRejected, reason)
}

// webhookCredentialCodes are the codes a Slack incoming webhook answers with
// when the webhook itself no longer works, whatever its status.
func webhookCredentialCodes() []string {
	return []string{"invalid_token", "no_service", "no_service_id", "no_team", "team_disabled"}
}
