// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slack"
)

// message is what a post sends.
const message = "jacob opened a pull request"

// posted is what a fake Slack received.
type posted struct {
	path, auth, contentType string
	body                    map[string]any
}

// slackReceiving serves an answer and records the post it got.
func slackReceiving(t *testing.T, status int, answer string, tls bool) (*httptest.Server, *atomic.Value) {
	t.Helper()

	var seen atomic.Value

	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any

		_ = json.NewDecoder(request.Body).Decode(&body)
		seen.Store(posted{
			path: request.URL.Path, auth: request.Header.Get("Authorization"),
			contentType: request.Header.Get("Content-Type"), body: body,
		})

		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(answer))
	})

	server := httptest.NewServer(handler)
	if tls {
		server = httptest.NewTLSServer(handler)
	}

	t.Cleanup(server.Close)

	return server, &seen
}

// received is what the fake Slack last saw.
func received(t *testing.T, seen *atomic.Value) posted {
	t.Helper()

	got, ok := seen.Load().(posted)
	if !ok {
		t.Fatal("nothing was posted")
	}

	return got
}

func TestPostWithABotTokenUsesChatPostMessage(t *testing.T) {
	t.Parallel()

	server, seen := slackReceiving(t, http.StatusOK, `{"ok":true,"channel":"C1","ts":"1.2"}`, false)

	err := slack.New(server.Client().Do, server.URL, botCredentials()).Post(t.Context(), message)
	if err != nil {
		t.Fatalf("Post returned %v, want nil", err)
	}

	got := received(t, seen)
	if got.path != "/chat.postMessage" || got.auth != "Bearer "+botToken ||
		!strings.HasPrefix(got.contentType, "application/json") {
		t.Errorf("posted %+v, want a JSON chat.postMessage with the bot token", got)
	}

	// A link preview of the pull request would bury the message under it.
	if got.body["channel"] != "#dev" || got.body["text"] != message || got.body["unfurl_links"] != false {
		t.Errorf("sent %v", got.body)
	}
}

func TestPostWithABotTokenReportsSlacksError(t *testing.T) {
	t.Parallel()

	// Slack's own convention: 200, and the answer is still no.
	server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"not_in_channel"}`, false)

	err := slack.New(server.Client().Do, server.URL, botCredentials()).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrRejected) || !strings.Contains(err.Error(), "not_in_channel") {
		t.Errorf("Post returned %v, want Slack's error", err)
	}

	server, _ = slackReceiving(t, http.StatusServiceUnavailable, `busy`, false)

	err = slack.New(server.Client().Do, server.URL, botCredentials()).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrUnexpectedStatus) {
		t.Errorf("Post returned %v, want ErrUnexpectedStatus", err)
	}
}

// webhookCredentials posts through a webhook at address.
func webhookCredentials(address string) config.Slack {
	return config.Slack{Token: "", WebhookURL: address, Channel: ""}
}

func TestPostWithAWebhookSendsJustTheText(t *testing.T) {
	t.Parallel()

	server, seen := slackReceiving(t, http.StatusOK, "ok", true)

	err := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(server.URL+"/services/T0/B0/x")).
		Post(t.Context(), message)
	if err != nil {
		t.Fatalf("Post returned %v, want nil", err)
	}

	got := received(t, seen)
	if got.path != "/services/T0/B0/x" || got.auth != "" || got.body["text"] != message {
		t.Errorf("posted %+v, want the text to the webhook with no Authorization", got)
	}
}

func TestAWebhookRefusalNeverShowsTheWebhook(t *testing.T) {
	t.Parallel()

	server, _ := slackReceiving(t, http.StatusNotFound, "channel_not_found", true)
	webhook := server.URL + "/services/T0/B0/secret-part"

	err := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(webhook)).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrRejected) || !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("Post returned %v, want Slack's reason", err)
	}

	// The URL is the credential: anyone holding it can post to that channel.
	if err != nil && strings.Contains(err.Error(), "secret-part") {
		t.Errorf("the error carried the webhook: %v", err)
	}
}

func TestAnUnreachableWebhookNeverShowsTheWebhook(t *testing.T) {
	t.Parallel()

	server, _ := slackReceiving(t, http.StatusOK, "ok", true)
	webhook := server.URL + "/services/T0/B0/secret-part"
	server.Close()

	// net/http's own error quotes the URL it was asked for.
	err := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(webhook)).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrUnreachable) {
		t.Errorf("Post returned %v, want ErrUnreachable", err)
	}

	if err != nil && strings.Contains(err.Error(), "secret-part") {
		t.Errorf("the error carried the webhook: %v", err)
	}
}

func TestAWebhookMustBeHTTPS(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	recording := func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errBrokeOff
	}

	for _, address := range []string{"http://hooks.slack.com/services/T0/B0/x", "://nope", "hooks.slack.com/services"} {
		err := slack.New(recording, slack.APIBase, webhookCredentials(address)).Post(t.Context(), message)
		if !errors.Is(err, slack.ErrInsecureWebhook) {
			t.Errorf("Post(%q) returned %v, want ErrInsecureWebhook", address, err)
		}

		if strings.Contains(err.Error(), "hooks.slack.com") {
			t.Errorf("the error carried the webhook: %v", err)
		}
	}

	if sent.Load() {
		t.Error("a message went to a webhook that is not HTTPS")
	}
}

func TestPostWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	err := slack.New(http.DefaultClient.Do, slack.APIBase, config.Slack{}).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrNoCredential) {
		t.Errorf("Post returned %v, want ErrNoCredential", err)
	}
}

func TestAnnouncementLinksThePullRequestAndTheIssue(t *testing.T) {
	t.Parallel()

	announcement := slack.Announcement{
		Author:           "jacob",
		PullRequestURL:   "https://github.com/example/repo/pull/42",
		PullRequestTitle: "fix(config): redact tokens",
		IssueKey:         "PROJ-412",
		IssueSummary:     "Fix token redaction",
		IssueURL:         "https://jira.example.com/browse/PROJ-412",
	}

	want := "jacob opened a pull request: <https://github.com/example/repo/pull/42|fix(config): redact tokens>\n" +
		"<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction"
	if got := announcement.Text(); got != want {
		t.Errorf("Text() =\n%s\nwant\n%s", got, want)
	}
}

func TestAnnouncementLeavesOutWhatItDoesNotKnow(t *testing.T) {
	t.Parallel()

	bare := slack.Announcement{PullRequestURL: "https://x/pull/1", PullRequestTitle: "docs: y"}
	if got := bare.Text(); got != "A pull request is ready for review: <https://x/pull/1|docs: y>" {
		t.Errorf("Text() = %q", got)
	}

	unlinked := slack.Announcement{
		PullRequestURL: "https://x/pull/1", PullRequestTitle: "t", IssueKey: "OPS-1", IssueSummary: "s",
	}
	if got := unlinked.Text(); !strings.HasSuffix(got, "\nOPS-1 s") {
		t.Errorf("Text() = %q, want the issue named without a link", got)
	}
}

func TestAnnouncementEscapesWhatSlackWouldReadAsMarkup(t *testing.T) {
	t.Parallel()

	// A title is anyone's to write. Unescaped, <!channel> pings everyone in it,
	// and a > ends the link early.
	announcement := slack.Announcement{
		Author:           "a&b",
		PullRequestURL:   "https://x/pull/1?a=1&b=2",
		PullRequestTitle: "fix: <!channel> handle a > b",
	}

	got := announcement.Text()
	if strings.Contains(got, "<!channel>") || !strings.Contains(got, "&lt;!channel&gt;") ||
		!strings.Contains(got, "a &gt; b") || !strings.Contains(got, "a&amp;b") || !strings.Contains(got, "a=1&amp;b=2") {
		t.Errorf("Text() = %q, want &, < and > escaped", got)
	}
}

func TestPostReportsAnswersThatCannotBeRead(t *testing.T) {
	t.Parallel()

	server, _ := slackReceiving(t, http.StatusOK, `{not json`, false)

	err := slack.New(server.Client().Do, server.URL, botCredentials()).Post(t.Context(), message)
	if err == nil {
		t.Error("Post accepted a malformed answer")
	}

	dropped := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(brokenBody{})}, nil
	}

	err = slack.New(dropped, slack.APIBase, botCredentials()).Post(t.Context(), message)
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want the read failure", err)
	}
}

func TestPostReportsATransportFailure(t *testing.T) {
	t.Parallel()

	failing := func(*http.Request) (*http.Response, error) { return nil, errBrokeOff }

	err := slack.New(failing, slack.APIBase, botCredentials()).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrUnreachable) || !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want ErrUnreachable wrapping the cause", err)
	}

	err = slack.New(failing, "https://slack.example.com/\x7f", botCredentials()).Post(t.Context(), message)
	if !errors.Is(err, slack.ErrUnreachable) {
		t.Errorf("Post to a malformed API base returned %v, want ErrUnreachable", err)
	}
}
