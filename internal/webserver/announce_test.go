// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// webhookSecret is the secret path of a Slack webhook, embedded in a post error
// so a test can prove the response never carries it back to the client.
const webhookSecret = "T00000000/B00000000/SECRETSECRETSECRETSECRET"

// errPostNamesTheWebhook is a post failure whose message embeds the webhook URL,
// as a real Slack client's error can.
var errPostNamesTheWebhook = errors.New(
	"posting to https://hooks.slack.com/services/" + webhookSecret + " failed: 500",
)

// doAnnounce posts an announcement to channel against a server over deps and cfg.
func doAnnounce(t *testing.T, deps webserver.Deps, cfg config.Config, channel string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, cfg), http.MethodPost, "/api/announce", `{"channel":"`+channel+`"}`)
}

func TestGetAnnouncementComposesThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Slack.Channel = testChannel

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
	cfg.Slack.Channel = testChannel

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

func TestAnnounceReportsAFailedPostWithoutLeakingTheWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	// A real Slack client's error can name the webhook URL it posted to; the
	// response must answer with a generic message rather than pass it through.
	deps := filledDeps()
	deps.Post = func(string, string) error { return errPostNamesTheWebhook }

	// Act
	recorder := doAnnounce(t, deps, config.Default(), "#dev")

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the post fails", recorder.Code)
	}

	body := recorder.Body.String()
	if strings.Contains(body, "hooks.slack.com") || strings.Contains(body, webhookSecret) {
		t.Errorf("response body %q leaked the webhook named in the post error", body)
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

	// There is nothing to announce without a pull request, whichever way it comes
	// back missing.
	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no pull request found": func(deps webserver.Deps) webserver.Deps {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

			return deps
		},
		"the forge cannot be reached": func(deps webserver.Deps) webserver.Deps {
			deps.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errSeam }

			return deps
		},
		"no forge seam": func(deps webserver.Deps) webserver.Deps {
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
