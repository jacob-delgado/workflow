// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// postMessagePath posts as the bot.
const postMessagePath = "/chat.postMessage"

// jsonContent is the media type both transports take. Slack's Web API asks for
// the charset to be named.
const jsonContent = "application/json; charset=utf-8"

// ErrInsecureWebhook reports a webhook that is not an https URL. The URL is the
// credential, and it is not sent in the clear.
var ErrInsecureWebhook = errors.New("slack.webhook_url is not an https URL")

// botMessage is what chat.postMessage takes. Unfurling is off: a preview of the
// pull request would bury the message under it.
type botMessage struct {
	Channel     string `json:"channel"`
	Text        string `json:"text"`
	UnfurlLinks bool   `json:"unfurl_links"`
	UnfurlMedia bool   `json:"unfurl_media"`
}

// webhookMessage is what an incoming webhook takes; its channel is its own.
type webhookMessage struct {
	Text string `json:"text"`
}

// verdict is chat.postMessage's answer.
type verdict struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// Post posts text to Slack through whichever transport is configured.
func (c Client) Post(ctx context.Context, text string) error {
	transports := map[config.SlackMode]func(context.Context, string) error{
		config.SlackBot:     c.postAsBot,
		config.SlackWebhook: c.postToWebhook,
	}

	post, configured := transports[c.creds.Mode()]
	if !configured {
		return ErrNoCredential
	}

	return post(ctx, text)
}

// postAsBot posts through chat.postMessage. Slack answers a refusal with 200
// and ok:false, so the status alone says nothing.
func (c Client) postAsBot(ctx context.Context, text string) error {
	message := botMessage{Channel: c.creds.Channel, Text: text, UnfurlLinks: false, UnfurlMedia: false}
	header := http.Header{"Authorization": {"Bearer " + c.creds.Token}}

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
		return fmt.Errorf("%w: %s", ErrRejected, answer.Error)
	}

	return nil
}

// postToWebhook posts to an incoming webhook, which answers "ok" as plain text,
// and a refusal as a status with its reason as plain text.
func (c Client) postToWebhook(ctx context.Context, text string) error {
	address, err := url.Parse(c.creds.WebhookURL)
	if err != nil || address.Scheme != "https" || address.Host == "" {
		// Unwrapped, whatever went wrong: a parse error quotes the URL.
		return ErrInsecureWebhook
	}

	_, err = c.postJSON(ctx, address.String(), webhookMessage{Text: text}, http.Header{})

	return err
}

// postJSON posts payload as JSON with the given headers, and returns the body of
// an accepted answer.
func (c Client) postJSON(ctx context.Context, address string, payload any, header http.Header) ([]byte, error) {
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

// deliver performs a post and returns the answer's body. Neither error names
// the address: for a webhook, the address is the credential.
func (c Client) deliver(request *http.Request) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, withoutURL(err))
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("reading the answer from Slack: %w", err)
	}

	switch {
	case response.StatusCode == http.StatusOK:
		return body, nil
	case response.StatusCode >= http.StatusBadRequest && response.StatusCode < http.StatusInternalServerError:
		return nil, fmt.Errorf("%w: %s", ErrRejected, strings.TrimSpace(sanitize.Text(string(body))))
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}
}

// withoutURL strips net/http's *url.Error down to its cause, which is the part
// that does not quote the address.
func withoutURL(err error) error {
	transportErr, ok := errors.AsType[*url.Error](err)
	if ok {
		return transportErr.Err
	}

	return err
}

// Announcement is the message telling a channel a pull request is ready.
type Announcement struct {
	Author           string
	PullRequestURL   string
	PullRequestTitle string
	IssueKey         string
	IssueSummary     string
	IssueURL         string
}

// Text is the announcement in Slack's markup: who opened what, linked, and the
// issue it is for, linked where there is a link.
//
// Every value is escaped. A title is anyone's to write, and unescaped,
// "<!channel>" in one pings the whole channel, and a ">" ends a link early.
func (a Announcement) Text() string {
	link := "<" + escape(a.PullRequestURL) + "|" + escape(a.PullRequestTitle) + ">"

	opened := "A pull request is ready for review: " + link
	if a.Author != "" {
		opened = escape(a.Author) + " opened a pull request: " + link
	}

	if a.IssueKey == "" {
		return opened
	}

	issue := escape(a.IssueKey)
	if a.IssueURL != "" {
		issue = "<" + escape(a.IssueURL) + "|" + issue + ">"
	}

	return opened + "\n" + issue + " " + escape(a.IssueSummary)
}

// escape writes text so Slack shows it rather than reading it as markup: the
// three characters its formatting uses, and nothing else.
func escape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}
