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
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// activityPostPath is where the Summary is posted, and summaryMarkdown a
// Summary as GET /api/activity writes it.
const (
	activityPostPath = "/api/activity/post"
	tuesday          = "2026-09-15"
	teamChannel      = "#team"
	summaryMarkdown  = "# " + tuesday + "\n\n- committed abc1234 Fix the leak\n"
)

// postedSummary is one post a fake messaging service took.
type postedSummary struct {
	channel string
	text    string
}

// recordingPost is a post seam that keeps each post in posts.
func recordingPost(posts *[]postedSummary) func(channel, text string) error {
	return func(channel, text string) error {
		*posts = append(*posts, postedSummary{channel: channel, text: text})

		return nil
	}
}

// postActivity posts the Summary request fields against handler.
func postActivity(t *testing.T, handler http.Handler, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}

	return send(t, handler, http.MethodPost, activityPostPath, string(body))
}

// tuesdaysSummary is the request posting summaryMarkdown for Tuesday.
func tuesdaysSummary() map[string]string {
	return map[string]string{"from": tuesday, "to": tuesday, textField: summaryMarkdown}
}

func TestTheSummaryIsPostedRenderedForTheService(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts []postedSummary

	deps := filledDeps()
	deps.Post = recordingPost(&posts)
	fields := tuesdaysSummary()
	fields[channelField] = teamChannel

	// Act
	recorder := postActivity(t, serve(t, deps, config.Default()), fields)

	// Assert
	want := postedSummary{channel: teamChannel, text: "*" + tuesday + "*\n\n• committed abc1234 Fix the leak\n"}
	if recorder.Code != http.StatusOK || len(posts) != 1 || posts[0] != want {
		t.Fatalf("POST %s = %d, posted %+v; want %+v once", activityPostPath, recorder.Code, posts, want)
	}

	answer := decode[api.ActivityPost](t, recorder)
	if answer.Channel != teamChannel || answer.Destination != teamChannel || answer.Text != summaryMarkdown ||
		answer.From != tuesday {
		t.Errorf("answer = %+v, want the Markdown posted to #team for Tuesday", answer)
	}
}

func TestAWebhookSummaryPostSaysItWentToTheWebhooksChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts []postedSummary

	deps := filledDeps()
	deps.Post = recordingPost(&posts)
	cfg := config.Default()
	cfg.Messaging = config.Messaging{Kind: config.KindTeams, WebhookURL: "https://example.com/hook"}

	// Act
	recorder := postActivity(t, serve(t, deps, cfg), tuesdaysSummary())

	// Assert
	answer := decode[api.ActivityPost](t, recorder)
	if len(posts) != 1 || !strings.HasPrefix(posts[0].text, "**"+tuesday+"**") || answer.Channel != "" ||
		answer.Destination != "the channel its webhook is bound to" {
		t.Errorf("posted %+v and answered %+v, want it rendered for Teams, to the webhook's own channel", posts, answer)
	}
}

func TestASummaryPostIsRefusedBeforeItGoes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		field, value string
		detail       string
	}{
		"a blank text":           {field: textField, value: " \n", detail: "the summary was empty"},
		"a day that is no date":  {field: "from", value: "Tuesday", detail: "YYYY-MM-DD"},
		"a period run backwards": {field: "to", value: "2026-09-14", detail: "ends before it starts"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var posts []postedSummary

			deps := filledDeps()
			deps.Post = recordingPost(&posts)
			fields := tuesdaysSummary()
			fields[tt.field] = tt.value

			// Act
			recorder := postActivity(t, serve(t, deps, config.Default()), fields)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity || len(posts) != 0 ||
				!strings.Contains(decode[api.Problem](t, recorder).Detail, tt.detail) {
				t.Errorf("POST = %d %s after %d posts, want 422 naming %q and nothing posted",
					recorder.Code, recorder.Body, len(posts), tt.detail)
			}
		})
	}
}

func TestAFailedSummaryPostNamesNeitherTheWebhookNorTheHost(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err    error
		status int
	}{
		"the service refused it": {
			err: fmt.Errorf("%w: https://hooks.slack.example/%s", messaging.ErrRejected, webhookSecret), status: 422,
		},
		"the service could not be reached": {
			err: fmt.Errorf("%w: hooks.slack.example/%s", messaging.ErrUnreachable, webhookSecret), status: 502,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Post = func(string, string) error { return tt.err }

			// Act
			recorder := postActivity(t, serve(t, deps, config.Default()), tuesdaysSummary())

			// Assert
			body := recorder.Body.String()
			if recorder.Code != tt.status || strings.Contains(body, webhookSecret) || strings.Contains(body, "hooks.slack") {
				t.Errorf("POST = %d %s, want %d naming neither the webhook nor its host", recorder.Code, body, tt.status)
			}
		})
	}
}

func TestASummaryPostWithNothingToPostThroughIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Post = nil

	// Act
	recorder := postActivity(t, serve(t, deps, config.Default()), tuesdaysSummary())

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST = %d %s, want 422", recorder.Code, recorder.Body)
	}
}

func TestASummaryPostIsRefusedUnderADryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts []postedSummary

	deps := filledDeps()
	deps.Post = recordingPost(&posts)
	handler := serveWith(t, deps, config.Default(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	recorder := postActivity(t, handler, tuesdaysSummary())

	// Assert
	if recorder.Code != http.StatusForbidden || len(posts) != 0 {
		t.Errorf("POST under --dry-run = %d after %d posts, want 403 and nothing posted", recorder.Code, len(posts))
	}
}
