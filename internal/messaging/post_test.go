// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// testAuthor is the author the announcement-text tests render.
const testAuthor = "jacob"

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
func counting(sent *atomic.Bool) messaging.Doer {
	return func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errNeverSent
	}
}

func TestPostWithABotTokenUsesChatPostMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := slackReceiving(t, http.StatusOK, `{"ok":true,"channel":"C1","ts":"1.2"}`)
	client := messaging.New(server.Client().Do, server.URL, botCredentials())

	// Act
	err := client.Post(t.Context(), "", message)
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
			want: messaging.ErrPostRefused, reason: "the bot is not in #dev",
		},
		"an unexpected status": {status: http.StatusServiceUnavailable, answer: `busy`, want: messaging.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, tt.status, tt.answer)
			client := messaging.New(server.Client().Do, server.URL, botCredentials())

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, tt.want) || !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("Post returned %v, want %v with %q", err, tt.want, tt.reason)
			}
		})
	}
}

// webhookCredentials posts through a webhook at address.
func webhookCredentials(address string) config.Messaging {
	return config.Messaging{Token: "", WebhookURL: config.Secret(address), Channel: ""}
}

// brokenAnswer answers every post with a body that breaks off mid-read.
func brokenAnswer(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(iotest.ErrReader(errBrokenAnswer))}, nil
}

// errBrokenAnswer is a connection dropped while the answer was being read.
var errBrokenAnswer = errors.New("connection reset")

func TestABrokenAnswerNamesTheServiceItCameFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	client := messaging.New(brokenAnswer, messaging.APIBase,
		messagingWebhook(config.KindTeams, "https://hooks.example.com/teams"))

	// Act
	err := client.Post(t.Context(), "", "hello")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "Teams") || strings.Contains(err.Error(), "Slack") {
		t.Errorf("Post = %v, want the failure to name Teams, the service it came from", err)
	}
}

// messagingWebhook posts through a kind's webhook at address.
func messagingWebhook(kind config.MessagingKind, address string) config.Messaging {
	return config.Messaging{Kind: kind, WebhookURL: config.Secret(address)}
}

// The webhook body keys and the pull request URL the per-kind body test shares,
// named so the same literal is not repeated across cases.
const (
	bodyKeyText    = "text"
	bodyKeyContent = "content"
	readyPullURL   = "https://x/pull/1"
)

// readyAnnouncement is a plain "ready for review" announcement rendered for kind.
func readyAnnouncement(kind config.MessagingKind) string {
	return messaging.Announcement{
		Kind: kind, PullRequestURL: readyPullURL, PullRequestTitle: "fix: redact",
	}.Text()
}

func TestAWebhookPostWrapsTheBodyAndMarkupPerKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind       config.MessagingKind
		status     int
		answer     string
		wantKey    string
		wantAbsent string
		wantMarkup string
	}{
		// Slack keeps its mrkdwn <url|text> link under the "text" key, and answers "ok".
		"slack": {
			kind: config.KindSlack, wantKey: bodyKeyText, wantAbsent: bodyKeyContent,
			wantMarkup: "<https://x/pull/1|fix: redact>", status: http.StatusOK, answer: "ok",
		},
		// Teams reads Markdown and takes the "text" key; a Workflows webhook answers 202.
		"teams": {
			kind: config.KindTeams, wantKey: bodyKeyText, wantAbsent: bodyKeyContent,
			wantMarkup: "[fix: redact](https://x/pull/1)", status: http.StatusAccepted,
		},
		// Discord reads Markdown but names its body key "content", and answers 204.
		"discord": {
			kind: config.KindDiscord, wantKey: bodyKeyContent, wantAbsent: bodyKeyText,
			wantMarkup: "[fix: redact](https://x/pull/1)", status: http.StatusNoContent,
		},
		// A plain webhook takes bare text under "text": title then its URL.
		"webhook": {
			kind: config.KindWebhook, wantKey: bodyKeyText, wantAbsent: bodyKeyContent,
			wantMarkup: "fix: redact https://x/pull/1", status: http.StatusOK,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, seen := webhookReceiving(t, tt.status, tt.answer)
			client := messaging.New(server.Client().Do, messaging.APIBase, messagingWebhook(tt.kind, server.URL+"/hook"))

			// Act
			err := client.Post(t.Context(), "", readyAnnouncement(tt.kind))
			if err != nil {
				t.Fatalf("Post returned %v, want nil", err)
			}

			// Assert
			got := received(t, seen)
			if got.body[tt.wantAbsent] != nil {
				t.Errorf("body carried a %q key: %v", tt.wantAbsent, got.body)
			}

			value, _ := got.body[tt.wantKey].(string)
			if !strings.Contains(value, tt.wantMarkup) {
				t.Errorf("body[%q] = %q, want it to contain %q", tt.wantKey, value, tt.wantMarkup)
			}
		})
	}
}

func TestAMarkdownAnnouncementNeutralizesAHostileValue(t *testing.T) {
	t.Parallel()

	// A title, author, summary and URL are all anyone's to write, and they reach
	// Teams and Discord as Markdown. Unescaped, "](http://evil)" in a title ends
	// the link and injects its own, and a ")" in the URL ends the link target
	// early — so every value is escaped and the URL's ")" is percent-encoded.
	for name, kind := range map[string]config.MessagingKind{
		"teams": config.KindTeams, "discord": config.KindDiscord,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			announcement := messaging.Announcement{
				Kind:             kind,
				Author:           "a[u*thor`",
				PullRequestURL:   "https://x/pull/1)evil",
				PullRequestTitle: "t](http://evil) *x* `code`",
				IssueKey:         "PROJ-1",
				IssueSummary:     "sum[m](ary)~|#>",
				IssueURL:         "https://x/browse)1",
			}

			// Act
			got := announcement.Text()

			// Assert
			if strings.Contains(got, "](http://evil)") {
				t.Errorf("Text() = %q, want the injected Markdown link neutralized", got)
			}

			if strings.Contains(got, "pull/1)evil") || !strings.Contains(got, "%29") {
				t.Errorf("Text() = %q, want the URL's ) percent-encoded so it cannot end the link", got)
			}

			// Emphasis, code, strikethrough, spoiler, heading and quote markers all
			// come from a value anyone can write, so each is shown literally.
			for _, want := range []string{`\*`, "\\`", `\~`, `\|`, `\#`, `\>`} {
				if !strings.Contains(got, want) {
					t.Errorf("Text() = %q, want %q escaped", got, want)
				}
			}
		})
	}
}

func TestADiscordPostPinsMentionsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	// "@everyone" and "@here" in a PR title come from the forge and are anyone's
	// to write; Discord resolves them out of the raw content and would ping the
	// whole server, so the body pins allowed_mentions to parse nothing.
	server, seen := webhookReceiving(t, http.StatusOK, "ok")
	hostile := messaging.Announcement{
		Kind:             config.KindDiscord,
		PullRequestTitle: "@everyone @here ship it",
		PullRequestURL:   readyPullURL,
	}
	client := messaging.New(server.Client().Do, messaging.APIBase,
		messagingWebhook(config.KindDiscord, server.URL+"/hook"))

	// Act
	err := client.Post(t.Context(), "", hostile.Text())
	if err != nil {
		t.Fatalf("Post returned %v, want nil", err)
	}

	// Assert
	got := received(t, seen)

	mentions, ok := got.body["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("Discord body has no allowed_mentions to pin pings off: %v", got.body)
	}

	if parse, isList := mentions["parse"].([]any); !isList || len(parse) != 0 {
		t.Errorf("allowed_mentions.parse = %v (a list: %v), want a present, empty list so nothing is pinged", parse, isList)
	}
}

func TestAMarkdownLinkFallsBackToTextForANonWebURL(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge should only ever return an http(s) URL; a value with any other
	// scheme is not trusted as a link target and is dropped to the escaped title.
	announcement := messaging.Announcement{
		Kind: config.KindTeams, PullRequestURL: "javascript:alert(1)", PullRequestTitle: "fix",
	}

	// Act
	got := announcement.Text()

	// Assert
	if strings.Contains(got, "javascript:") || strings.Contains(got, "](") {
		t.Errorf("Text() = %q, want no link built for a non-web URL", got)
	}
}

func TestPostWithAWebhookSendsJustTheText(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := webhookReceiving(t, http.StatusOK, "ok")
	client := messaging.New(server.Client().Do, messaging.APIBase, webhookCredentials(server.URL+"/services/T0/B0/x"))

	// Act
	err := client.Post(t.Context(), "", message)
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
	client := messaging.New(server.Client().Do, messaging.APIBase,
		webhookCredentials(server.URL+"/services/T0/B0/secret-part"))

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || !strings.Contains(err.Error(), "channel_not_found") {
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

	client := messaging.New(server.Client().Do, messaging.APIBase, webhookCredentials(webhook))

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	// net/http's own error quotes the URL it was asked for.
	if !errors.Is(err, messaging.ErrUnreachable) || strings.Contains(err.Error(), "secret-part") {
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

			client := messaging.New(counting(&sent), messaging.APIBase, webhookCredentials(address))

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, messaging.ErrInsecureWebhook) || strings.Contains(err.Error(), "hooks.slack.com") {
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

	client := messaging.New(counting(&sent), messaging.APIBase, config.Messaging{})

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || sent.Load() {
		t.Errorf("Post returned %v and sent %v, want ErrNoCredential and nothing sent", err, sent.Load())
	}
}

func TestAnnouncementText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		announcement messaging.Announcement
		want         string
	}{
		"the pull request and the issue, both linked": {
			announcement: messaging.Announcement{
				Author:           testAuthor,
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
			announcement: messaging.Announcement{PullRequestURL: "https://x/pull/1", PullRequestTitle: "docs: y"},
			want:         "A pull request is ready for review: <https://x/pull/1|docs: y>",
		},
		"an issue with no link to it": {
			announcement: messaging.Announcement{
				PullRequestURL: "https://x/pull/1", PullRequestTitle: "t", IssueKey: "OPS-1", IssueSummary: "s",
			},
			want: "A pull request is ready for review: <https://x/pull/1|t>\nOPS-1 s",
		},
		"a merged pull request, with author": {
			announcement: messaging.Announcement{
				Moment:           messaging.MomentMerged,
				Author:           testAuthor,
				PullRequestURL:   "https://x/pull/42",
				PullRequestTitle: "fix: redact tokens",
			},
			want: "jacob merged a pull request: <https://x/pull/42|fix: redact tokens>",
		},
		"a merged pull request, no author": {
			announcement: messaging.Announcement{
				Moment: messaging.MomentMerged, PullRequestURL: "https://x/pull/42", PullRequestTitle: "t",
			},
			want: "A pull request merged: <https://x/pull/42|t>",
		},
		"CI is red on the change": {
			announcement: messaging.Announcement{
				Moment: messaging.MomentCIRed, PullRequestURL: "https://x/pull/9", PullRequestTitle: "flaky",
			},
			want: "CI is red on the pull request: <https://x/pull/9|flaky>",
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
	announcement := messaging.Announcement{
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

func TestEveryMomentEscapesWhatSlackWouldReadAsMarkup(t *testing.T) {
	t.Parallel()

	// Every moment builds its link and lead the same way, so the escaping that
	// stops a hostile title injecting Slack markup must hold for all of them.
	for name, moment := range map[string]messaging.Moment{
		"ready": messaging.MomentReady, "merged": messaging.MomentMerged, "ci red": messaging.MomentCIRed,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The author, title and URL are all hostile, so every moment that
			// includes them must escape them — an unescaped one lets a title or a
			// name inject Slack markup.
			announcement := messaging.Announcement{
				Moment: moment, Author: "a&b<!channel>",
				PullRequestURL: "https://x/pull/1?a=1&b=2", PullRequestTitle: "fix: <!here> a > b",
			}

			// Act
			got := announcement.Text()

			// Assert
			if strings.Contains(got, "<!channel>") || strings.Contains(got, "<!here>") ||
				strings.Contains(got, "a > b") || !strings.Contains(got, "&lt;!here&gt;") {
				t.Errorf("Text() = %q, want &, < and > escaped for the %s moment", got, name)
			}
		})
	}
}

func TestAConfiguredTemplateShapesOnlyAReadyAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// The template is the team's "ready for review" wording. A merge is not ready
	// for review, so it uses the built-in merge text rather than the template.
	announcement := messaging.Announcement{
		Moment:           messaging.MomentMerged,
		Template:         "🚀 {author} needs a review of <{url}|{title}>",
		Author:           testAuthor,
		PullRequestURL:   "https://x/pull/7",
		PullRequestTitle: "fix: redact",
	}

	// Act
	got := announcement.Text()

	// Assert
	if want := "jacob merged a pull request: <https://x/pull/7|fix: redact>"; got != want {
		t.Errorf("Text() = %q, want the built-in merge text, not the template", got)
	}
}

func TestAConfiguredTemplateShapesTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	announcement := messaging.Announcement{
		Template:         "🚀 {author} needs a review of <{url}|{title}> for {key}",
		Author:           testAuthor,
		PullRequestURL:   "https://x/pull/7",
		PullRequestTitle: "fix: redact tokens",
		IssueKey:         "PROJ-412",
	}

	// Act
	got := announcement.Text()

	// Assert
	want := "🚀 jacob needs a review of <https://x/pull/7|fix: redact tokens> for PROJ-412"
	if got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestAConfiguredTemplateStillEscapesAHostileValue(t *testing.T) {
	t.Parallel()

	// Arrange
	// The template is the team's own, but a title is anyone's: a substituted
	// value must stay escaped so it cannot break out of the template and ping the
	// whole channel.
	announcement := messaging.Announcement{
		Template:         "review please: {title}",
		PullRequestTitle: "fix: <!channel> ship it",
	}

	// Act
	got := announcement.Text()

	// Assert
	if strings.Contains(got, "<!channel>") || !strings.Contains(got, "&lt;!channel&gt;") {
		t.Errorf("Text() = %q, want the title escaped so it cannot ping the channel", got)
	}
}

func TestPostReportsAnAnswerThatIsNotJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := slackReceiving(t, http.StatusOK, `{not json`)
	client := messaging.New(server.Client().Do, server.URL, botCredentials())

	// Act
	err := client.Post(t.Context(), "", message)

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

	client := messaging.New(dropped, messaging.APIBase, botCredentials())

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want the read failure", err)
	}
}

func TestPostReportsATransportFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := func(*http.Request) (*http.Response, error) { return nil, errBrokeOff }
	client := messaging.New(failing, messaging.APIBase, botCredentials())

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrUnreachable) || !errors.Is(err, errBrokeOff) {
		t.Errorf("Post returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestPostToAMalformedAPIBaseIsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	client := messaging.New(counting(&sent), "https://slack.example.com/\x7f", botCredentials())

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrUnreachable) || sent.Load() {
		t.Errorf("Post returned %v and sent %v, want ErrUnreachable before sending", err, sent.Load())
	}
}

func TestABotPostRefusalNamesTheFix(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		code string
		want string
	}{
		"not in the channel":      {code: "not_in_channel", want: "the message was refused: the bot is not in #dev"},
		"no such channel":         {code: "channel_not_found", want: "no channel #dev"},
		"the channel is archived": {code: "is_archived", want: "#dev is archived. Choose an open channel"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"`+tt.code+`"}`)
			client := messaging.New(server.Client().Do, server.URL, botCredentials())

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Post returned %v, want a sentence naming the fix %q", err, tt.want)
			}
		})
	}
}
