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

// recording answers every request with status and answer, and records the post
// it got.
func recording(status int, answer string) (http.Handler, *atomic.Value) {
	var seen atomic.Value

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any

		_ = json.NewDecoder(request.Body).Decode(&body)
		seen.Store(posted{
			path: request.URL.Path, auth: request.Header.Get("Authorization"),
			contentType: request.Header.Get("Content-Type"), body: body,
		})

		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(answer))
	}), &seen
}

// slackReceiving is a Slack API over plain HTTP that answers, and records, one
// post.
func slackReceiving(t *testing.T, status int, answer string) (*httptest.Server, *atomic.Value) {
	t.Helper()

	handler, seen := recording(status, answer)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, seen
}

// webhookReceiving is an incoming webhook, which must be HTTPS, that answers,
// and records, one post.
func webhookReceiving(t *testing.T, status int, answer string) (*httptest.Server, *atomic.Value) {
	t.Helper()

	handler, seen := recording(status, answer)
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	return server, seen
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

// errNeverSent is what a transport that should never be used answers with.
var errNeverSent = errors.New("this transport should not have been used")

// counting is a transport that records whether it was used, and fails.
func counting(sent *atomic.Bool) slack.Doer {
	return func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errNeverSent
	}
}

func TestPostWithABotTokenUsesChatPostMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := slackReceiving(t, http.StatusOK, `{"ok":true,"channel":"C1","ts":"1.2"}`)
	client := slack.New(server.Client().Do, server.URL, botCredentials())

	// Act
	err := client.Post(t.Context(), message)
	if err != nil {
		t.Fatalf("Post returned %v, want nil", err)
	}

	// Assert
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

	cases := map[string]struct {
		status int
		answer string
		want   error
		reason string
	}{
		// Slack's own convention: 200, and the answer is still no.
		"a refusal inside a 200": {
			status: http.StatusOK, answer: `{"ok":false,"error":"not_in_channel"}`,
			want: slack.ErrPostRefused, reason: "the bot is not in #dev",
		},
		"an unexpected status": {status: http.StatusServiceUnavailable, answer: `busy`, want: slack.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, tt.status, tt.answer)
			client := slack.New(server.Client().Do, server.URL, botCredentials())

			// Act
			err := client.Post(t.Context(), message)

			// Assert
			if !errors.Is(err, tt.want) || !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("Post returned %v, want %v with %q", err, tt.want, tt.reason)
			}
		})
	}
}

// webhookCredentials posts through a webhook at address.
func webhookCredentials(address string) config.Slack {
	return config.Slack{Token: "", WebhookURL: address, Channel: ""}
}

func TestPostWithAWebhookSendsJustTheText(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := webhookReceiving(t, http.StatusOK, "ok")
	client := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(server.URL+"/services/T0/B0/x"))

	// Act
	err := client.Post(t.Context(), message)
	if err != nil {
		t.Fatalf("Post returned %v, want nil", err)
	}

	// Assert
	got := received(t, seen)
	if got.path != "/services/T0/B0/x" || got.auth != "" || got.body["text"] != message {
		t.Errorf("posted %+v, want the text to the webhook with no Authorization", got)
	}
}

func TestAWebhookRefusalNeverShowsTheWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := webhookReceiving(t, http.StatusNotFound, "channel_not_found")
	client := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(server.URL+"/services/T0/B0/secret-part"))

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if !errors.Is(err, slack.ErrRejected) || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("Post returned %v, want Slack's reason", err)
	}

	// The URL is the credential: anyone holding it can post to that channel.
	if strings.Contains(err.Error(), "secret-part") {
		t.Errorf("the error carried the webhook: %v", err)
	}
}

func TestAnUnreachableWebhookNeverShowsTheWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := webhookReceiving(t, http.StatusOK, "ok")
	webhook := server.URL + "/services/T0/B0/secret-part"
	server.Close()

	client := slack.New(server.Client().Do, slack.APIBase, webhookCredentials(webhook))

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	// net/http's own error quotes the URL it was asked for.
	if !errors.Is(err, slack.ErrUnreachable) || strings.Contains(err.Error(), "secret-part") {
		t.Errorf("Post returned %v, want ErrUnreachable without the webhook", err)
	}
}

func TestAWebhookMustBeHTTPS(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"plain http":     "http://hooks.slack.com/services/T0/B0/x",
		"not a url":      "://nope",
		"with no scheme": "hooks.slack.com/services",
	}

	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var sent atomic.Bool

			client := slack.New(counting(&sent), slack.APIBase, webhookCredentials(address))

			// Act
			err := client.Post(t.Context(), message)

			// Assert
			if !errors.Is(err, slack.ErrInsecureWebhook) || strings.Contains(err.Error(), "hooks.slack.com") {
				t.Errorf("Post(%q) returned %v, want ErrInsecureWebhook without the webhook", address, err)
			}

			if sent.Load() {
				t.Error("a message went to a webhook that is not HTTPS")
			}
		})
	}
}

func TestPostWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	client := slack.New(counting(&sent), slack.APIBase, config.Slack{})

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if !errors.Is(err, slack.ErrNoCredential) || sent.Load() {
		t.Errorf("Post returned %v and sent %v, want ErrNoCredential and nothing sent", err, sent.Load())
	}
}

func TestAnnouncementText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		announcement slack.Announcement
		want         string
	}{
		"the pull request and the issue, both linked": {
			announcement: slack.Announcement{
				Author:           "jacob",
				PullRequestURL:   "https://github.com/example/repo/pull/42",
				PullRequestTitle: "fix(config): redact tokens",
				IssueKey:         "PROJ-412",
				IssueSummary:     "Fix token redaction",
				IssueURL:         "https://jira.example.com/browse/PROJ-412",
			},
			want: "jacob opened a pull request: <https://github.com/example/repo/pull/42|fix(config): redact tokens>\n" +
				"<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction",
		},
		"no author and no issue": {
			announcement: slack.Announcement{PullRequestURL: "https://x/pull/1", PullRequestTitle: "docs: y"},
			want:         "A pull request is ready for review: <https://x/pull/1|docs: y>",
		},
		"an issue with no link to it": {
			announcement: slack.Announcement{
				PullRequestURL: "https://x/pull/1", PullRequestTitle: "t", IssueKey: "OPS-1", IssueSummary: "s",
			},
			want: "A pull request is ready for review: <https://x/pull/1|t>\nOPS-1 s",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.announcement.Text(); got != tt.want {
				t.Errorf("Text() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestAnnouncementEscapesWhatSlackWouldReadAsMarkup(t *testing.T) {
	t.Parallel()

	// Arrange
	// A title is anyone's to write. Unescaped, <!channel> pings everyone in it,
	// and a > ends the link early.
	announcement := slack.Announcement{
		Author:           "a&b",
		PullRequestURL:   "https://x/pull/1?a=1&b=2",
		PullRequestTitle: "fix: <!channel> handle a > b",
	}

	// Act
	got := announcement.Text()

	// Assert
	if strings.Contains(got, "<!channel>") || !strings.Contains(got, "&lt;!channel&gt;") ||
		!strings.Contains(got, "a &gt; b") || !strings.Contains(got, "a&amp;b") || !strings.Contains(got, "a=1&amp;b=2") {
		t.Errorf("Text() = %q, want &, < and > escaped", got)
	}
}

func TestPostReportsAnAnswerThatIsNotJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := slackReceiving(t, http.StatusOK, `{not json`)
	client := slack.New(server.Client().Do, server.URL, botCredentials())

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("Post returned %v, want the malformed answer's syntax error", err)
	}
}

func TestPostReportsAnAnswerThatBreaksOff(t *testing.T) {
	t.Parallel()

	// Arrange
	dropped := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(brokenBody{})}, nil
	}

	client := slack.New(dropped, slack.APIBase, botCredentials())

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want the read failure", err)
	}
}

func TestPostReportsATransportFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := func(*http.Request) (*http.Response, error) { return nil, errBrokeOff }
	client := slack.New(failing, slack.APIBase, botCredentials())

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if !errors.Is(err, slack.ErrUnreachable) || !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestPostToAMalformedAPIBaseIsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	client := slack.New(counting(&sent), "https://slack.example.com/\x7f", botCredentials())

	// Act
	err := client.Post(t.Context(), message)

	// Assert
	if !errors.Is(err, slack.ErrUnreachable) || sent.Load() {
		t.Errorf("Post returned %v and sent %v, want ErrUnreachable before sending", err, sent.Load())
	}
}

func TestABotPostRefusalNamesTheFix(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		code string
		want string
	}{
		"not in the channel":      {code: "not_in_channel", want: "the bot is not in #dev"},
		"no such channel":         {code: "channel_not_found", want: "no channel #dev"},
		"the channel is archived": {code: "is_archived", want: "#dev is archived"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"`+tt.code+`"}`)
			client := slack.New(server.Client().Do, server.URL, botCredentials())

			// Act
			err := client.Post(t.Context(), message)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Post returned %v, want a sentence naming the fix %q", err, tt.want)
			}
		})
	}
}
