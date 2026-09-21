// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
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

// doAnnounce posts an announcement to channel against a server over deps and cfg.
func doAnnounce(t *testing.T, deps webserver.Deps, cfg config.Config, channel string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serve(t, deps, cfg), http.MethodPost, "/api/announce", `{"channel":"`+channel+`"}`)
}

func TestGetAnnouncementComposesThePreview(t *testing.T) {
	t.Parallel()

	// Act
	// filledDeps has a pull request, an author, and the branch's issue.
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/announcement")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	text := decode[api.Announcement](t, recorder).Text
	if !strings.Contains(text, "redact") || !strings.Contains(text, "PROJ-412") {
		t.Errorf("preview = %q, want the pull request title and the issue key", text)
	}
}

func TestGetAnnouncementUsesTheForgeNounAndIssueLink(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.BrowseURL = func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }

	cfg := config.Default()
	cfg.Forge.Kind = "gitlab"

	// Act
	recorder := get(t, serve(t, deps, cfg), "/api/announcement")

	// Assert
	text := decode[api.Announcement](t, recorder).Text
	if !strings.Contains(text, "merge request") || !strings.Contains(text, "browse/PROJ-412") {
		t.Errorf("preview = %q, want the merge-request noun and a link to the issue", text)
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
	cfg.Slack.Channel = "#dev-workflow"

	// Act
	// An empty channel in the request falls back to the configured one.
	recorder := doAnnounce(t, deps, cfg, "")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if toChannel != "#dev-workflow" {
		t.Errorf("posted to %q, want the configured #dev-workflow", toChannel)
	}
}

func TestAnnounceReportsAFailedPost(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Post = func(string, string) error { return errSeam }

	// Act
	recorder := doAnnounce(t, deps, config.Default(), "#dev")

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the post fails", recorder.Code)
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
