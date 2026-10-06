// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// webhookSecret is the secret path of a Slack webhook, embedded in a post error
// so a test can prove the response never carries it back to the client.
const webhookSecret = "T00000000/B00000000/SECRETSECRETSECRETSECRET"

// postAnnounce posts an announce request of fields against handler, encoded
// as JSON so a channel or a text that needs escaping reaches the server intact.
func postAnnounce(t *testing.T, handler http.Handler, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encoding the announce request: %v", err)
	}

	return send(t, handler, http.MethodPost, "/api/announce", string(body))
}

// doAnnounce posts an announcement to channel against a server over deps and cfg.
func doAnnounce(t *testing.T, deps webserver.Deps, cfg config.Config, channel string) *httptest.ResponseRecorder {
	t.Helper()

	return postAnnounce(t, serve(t, deps, cfg), map[string]string{channelField: channel})
}

// announcePreviewed posts the announcement to channel against handler, carrying
// the text its preview showed.
func announcePreviewed(t *testing.T, handler http.Handler, channel, previewed string) *httptest.ResponseRecorder {
	t.Helper()

	return postAnnounce(t, handler, map[string]string{"channel": channel, "text": previewed})
}

// previewAnnouncement is the announcement's text as GET /api/announcement shows
// it from handler.
func previewAnnouncement(t *testing.T, handler http.Handler) string {
	t.Helper()

	recorder := get(t, handler, "/api/announcement")
	if recorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200", recorder.Code)
	}

	return decode[api.Announcement](t, recorder).Text
}

func TestAnnounceRefusesAnAnnouncementThatChangedSinceItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes when the page previews the announcement and has failed by the
	// time it posts, so the announcement composed now is not the one shown.
	ciState := forge.CIPassed
	posts := 0

	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{State: ciState}, nil }
	deps.Post = func(string, string) error {
		posts++

		return nil
	}

	handler := serve(t, deps, config.Default())
	previewed := previewAnnouncement(t, handler)
	ciState = forge.CIFailed

	// Act
	recorder := announcePreviewed(t, handler, "#dev", previewed)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || !strings.Contains(failure.Detail, "changed since it was previewed") {
		t.Errorf("status/detail = %d/%q, want 409 saying it changed since it was previewed", recorder.Code, failure.Detail)
	}

	if posts != 0 {
		t.Errorf("posted %d times, want nothing sent", posts)
	}
}

func TestAnnouncePostsTheTextItsPreviewShowed(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	deps := filledDeps()
	deps.Post = func(_, text string) error {
		posted = text

		return nil
	}

	handler := serve(t, deps, config.Default())
	previewed := previewAnnouncement(t, handler)

	// Act
	recorder := announcePreviewed(t, handler, "#dev", previewed)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an announcement unchanged since its preview", recorder.Code)
	}

	if posted != previewed {
		t.Errorf("posted %q, want the previewed %q", posted, previewed)
	}
}

func TestAnnouncePostsThePreviewWhenTheAuthorReadFailsAfterIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge says who the author is for the preview and cannot say by the
	// time of the post, so only the author kept from the preview can match it.
	var posted string

	asked := 0
	deps := filledDeps()
	deps.Author = func() (string, error) {
		asked++
		if asked > 1 {
			return "", errSeam
		}

		return testAuthor, nil
	}
	deps.Post = func(_, text string) error {
		posted = text

		return nil
	}

	handler := serve(t, deps, config.Default())
	previewed := previewAnnouncement(t, handler)

	// Act
	recorder := announcePreviewed(t, handler, "#dev", previewed)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for the announcement its preview showed", recorder.Code)
	}

	if posted != previewed || !strings.Contains(posted, testAuthor) {
		t.Errorf("posted %q, want the previewed %q, naming %s", posted, previewed, testAuthor)
	}
}

func TestGetAnnouncementComposesThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.Channel = testChannel

	// Act
	// filledDeps has a pull request, an author, and the branch's issue.
	recorder := get(t, serve(t, filledDeps(), cfg), "/api/announcement")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	preview := decode[api.Announcement](t, recorder)
	if !strings.Contains(preview.Text, "redact") || !strings.Contains(preview.Text, "PROJ-412") {
		t.Errorf("preview = %q, want the pull request title and the issue key", preview.Text)
	}

	if preview.Channel != testChannel {
		t.Errorf("preview channel = %q, want the configured %s", preview.Channel, testChannel)
	}
}

func TestGetAnnouncementUsesTheForgeNounAndIssueLink(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.BrowseURL = func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }
	info := webserver.Info{Version: testVersion, ForgeKind: forge.KindGitLab}

	// Act
	recorder := get(t, serveWith(t, deps, config.Default(), info), "/api/announcement")

	// Assert
	text := decode[api.Announcement](t, recorder).Text
	if !strings.Contains(text, "merge request") || !strings.Contains(text, "browse/PROJ-412") {
		t.Errorf("preview = %q, want the merge-request noun and a link to the issue", text)
	}
}

func TestGetAnnouncementUsesThePullRequestNounForGitHub(t *testing.T) {
	t.Parallel()

	// Arrange
	info := webserver.Info{Version: testVersion, ForgeKind: forge.KindGitHub}

	// Act
	recorder := get(t, serveWith(t, filledDeps(), config.Default(), info), "/api/announcement")

	// Assert
	text := decode[api.Announcement](t, recorder).Text
	if !strings.Contains(text, "pull request") || strings.Contains(text, "merge request") {
		t.Errorf("preview = %q, want the pull-request noun for a GitHub remote", text)
	}
}

func TestGetAnnouncementMarksAMergedPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's pull request has merged, so the announcement marks the merge.
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{Number: 42, URL: "https://x/42", Title: "redact", State: forge.StateMerged}, true, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/announcement")

	// Assert
	if text := decode[api.Announcement](t, recorder).Text; !strings.Contains(text, "merged a pull request") {
		t.Errorf("preview = %q, want a merge announcement", text)
	}
}

func TestGetAnnouncementMarksRedCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// The open pull request's CI has failed, so the announcement marks that.
	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed}, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/announcement")

	// Assert
	if text := decode[api.Announcement](t, recorder).Text; !strings.Contains(text, "CI is red on the pull request") {
		t.Errorf("preview = %q, want a red-CI announcement", text)
	}
}

func TestGetAnnouncementFallsBackToReadyWhenCICannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The CI read fails, so the open pull request's announcement stays "ready for
	// review" rather than marking a red CI on a result it could not read.
	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed}, errSeam
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/announcement")

	// Assert
	text := decode[api.Announcement](t, recorder).Text
	if strings.Contains(text, "CI is red") || !strings.Contains(text, "opened a pull request") {
		t.Errorf("preview = %q, want a ready announcement when CI cannot be read", text)
	}
}

func TestAnnouncePostsToTheChosenChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		toChannel string
		posted    string
	)

	deps := filledDeps()
	deps.Post = func(channel, text string) error {
		toChannel = channel
		posted = text

		return nil
	}

	// Act
	recorder := doAnnounce(t, deps, config.Default(), "#releases")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if toChannel != "#releases" {
		t.Errorf("posted to %q, want #releases", toChannel)
	}

	if !strings.Contains(posted, "redact") {
		t.Errorf("posted %q, want the announcement text", posted)
	}

	if channel := decode[api.Announcement](t, recorder).Channel; channel != "#releases" {
		t.Errorf("response channel = %q, want the chosen #releases", channel)
	}
}

func TestAnnounceFallsBackToTheConfiguredChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	var toChannel string

	deps := filledDeps()
	deps.Post = func(channel, _ string) error {
		toChannel = channel

		return nil
	}

	cfg := config.Default()
	cfg.Messaging.Channel = testChannel

	// Act
	// An empty channel in the request falls back to the configured one.
	recorder := doAnnounce(t, deps, cfg, "")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if toChannel != testChannel {
		t.Errorf("posted to %q, want the configured %s", toChannel, testChannel)
	}
}

func TestAnnounceNeverForwardsTheWebhook(t *testing.T) {
	t.Parallel()

	// Every failure the messaging client can answer a post with, carrying the
	// webhook the way a client's error can; the answer says what to do, in words
	// that tell the failures apart, and never names the webhook.
	unprocessable, unreachable := http.StatusUnprocessableEntity, http.StatusBadGateway
	cases := map[string]struct {
		cause      error
		wantStatus int
		want       string
	}{
		"nothing set up": {
			messaging.ErrNoCredential, unprocessable,
			"set up the Slack user token or a webhook URL in Settings, or run workflow slack login",
		},
		"a webhook not on https": {messaging.ErrInsecureWebhook, unprocessable, "not an https address"},
		"the service refused":    {messaging.ErrRejected, unprocessable, "or that the webhook URL is current"},
		"the message refused":    {messaging.ErrPostRefused, unprocessable, "post from a terminal to see its reason"},
		"an undocumented answer": {messaging.ErrUnexpectedStatus, unreachable, "answered with a status it does not document"},
		"no answer":              {messaging.ErrUnreachable, unreachable, "check the network, then try again"},
		"asked to wait":          {httpx.ErrRateLimited, unreachable, waitAndTryAgain},
		"a redirect refused":     {httpx.ErrRedirected, unreachable, "check its configured address"},
		"a failure of no kind":   {errSeam, http.StatusInternalServerError, tryAgain},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Post = func(string, string) error {
				return fmt.Errorf("posting to https://hooks.slack.com/services/%s: %w", webhookSecret, tt.cause)
			}

			// Act
			recorder := doAnnounce(t, deps, config.Default(), "#dev")

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.wantStatus || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status/detail = %d/%q, want %d saying %q", recorder.Code, failure.Detail, tt.wantStatus, tt.want)
			}

			body := recorder.Body.String()
			if strings.Contains(body, "hooks.slack.com") || strings.Contains(body, webhookSecret) {
				t.Errorf("body = %q, leaks the webhook", body)
			}
		})
	}
}

func TestAnnounceIsUnavailableWithoutASlackSeam(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps leaves Post nil.
	deps := filledDeps()

	// Act
	recorder := doAnnounce(t, deps, config.Default(), "#dev")

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when announcing is not available", recorder.Code)
	}
}

func TestGetAnnouncementIsAConflictWithoutAPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/announcement")

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 when there is no pull request to preview", recorder.Code)
	}
}

func TestAnnouncingIsAConflictWithoutAPullRequest(t *testing.T) {
	t.Parallel()

	// There is nothing to announce without a pull request, whether the forge
	// finds none or there is no forge to ask.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no pull request found": func(deps webserver.Deps) webserver.Deps {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

			return deps
		},
		noForgeSeam: func(deps webserver.Deps) webserver.Deps {
			deps.FindPull = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := mutate(filledDeps())
			deps.Post = func(string, string) error { return nil }

			// Act
			recorder := doAnnounce(t, deps, config.Default(), "#dev")

			// Assert
			if recorder.Code != http.StatusConflict {
				t.Errorf("status = %d, want 409 when there is no pull request", recorder.Code)
			}
		})
	}
}
