// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// Tagging follows the configuration in effect, not the seams the server was
// started with, and a post tags only whom its preview showed.

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// otherChannel is a channel a preview picks other than the configured one.
const otherChannel = "#ops"

// slackWebhookConfig is a configuration that posts to a Slack incoming
// webhook, which cannot tag.
func slackWebhookConfig() config.Config {
	cfg := config.Default()
	cfg.Messaging.WebhookURL = "https://hooks.slack.example/T0/B0/x"

	return cfg
}

// keptOver is a server over fake whose posts land in posted, under cfg.
func keptOver(t *testing.T, fake *fakeKept, cfg config.Config, posted *string) http.Handler {
	t.Helper()

	deps := filledDeps()
	fake.wire(&deps)
	deps.Messaging.Post = func(_, text string) error {
		*posted = text

		return nil
	}

	return serve(t, deps, cfg)
}

func TestGetAnnouncementTagsNoOneWhenTheConfigurationPostsByWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	handler := keptOver(t, newFakeKept(), slackWebhookConfig(), &posted)

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	if got := decode[api.Announcement](t, recorder).Tagging; got != nil {
		t.Errorf("tagging = %+v, want none while posting by webhook", got)
	}
}

func TestGetAnnouncementTagsNoOneWhenSlackHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	fake.membersErr = messaging.ErrNoCredential
	fake.groupsErr = messaging.ErrNoCredential
	handler := keptOver(t, fake, slackUserConfig(), &posted)

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	if got := decode[api.Announcement](t, recorder).Tagging; got != nil {
		t.Errorf("tagging = %+v, want none while Slack has no credential", got)
	}
}

func TestGetAnnouncementChecksTheScopeAgainstTheChannelPreviewed(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	deps := filledDeps()
	newFakeKept().wire(&deps)
	deps.Messaging.ChannelMembers = func(channel string) ([]loop.SlackTarget, error) {
		asked = append(asked, channel)

		return nil, &messaging.MissingScopeError{Needed: usersRead}
	}
	handler := serve(t, deps, slackUserConfig())

	// Act
	recorder := get(t, handler, "/api/announcement?channel=%23ops")

	// Assert
	got := decode[api.Announcement](t, recorder)
	elsewhere := func(channel string) bool { return channel != otherChannel }

	everyRead := len(asked) > 0 && !slices.ContainsFunc(asked, elsewhere)

	if got.Channel != otherChannel || !everyRead {
		t.Errorf("channel %q, members read in %v; want #ops for the preview and every read", got.Channel, asked)
	}
}

func TestAnnounceRefusesMentionsWhileTheConfigurationPostsByWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	handler := keptOver(t, newFakeKept(), slackWebhookConfig(), &posted)

	// Act
	recorder := announceWith(t, handler, []string{carla().ID}, []string{apiGroupID})

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || posted != "" {
		t.Errorf("status %d, posted %q; want 422 and nothing posted", recorder.Code, posted)
	}
}

func TestAnnounceRefusesWhenWhomItTagsChangedSinceThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	fake.links = append(fake.links, ownerLinkedTo(benOwner, ben()))
	handler := keptOver(t, fake, slackUserConfig(), &posted)

	// Act
	recorder := announceWith(t, handler, []string{carla().ID}, []string{apiGroupID})

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || posted != "" || !strings.Contains(failure.Detail, "preview it again") {
		t.Errorf("status/detail %d/%q, posted %q; want 409 and nothing posted", recorder.Code, failure.Detail, posted)
	}
}

func TestAnnounceTagsWhenTheUsersMatchThePreviewInAnyOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	fake.links = append(fake.links, ownerLinkedTo(benOwner, ben()))
	handler := keptOver(t, fake, slackUserConfig(), &posted)

	// Act
	recorder := announceWith(t, handler, []string{ben().ID, carla().ID}, []string{})

	// Assert
	if recorder.Code != http.StatusOK || !strings.HasSuffix(posted, "cc <@U0BEN> <@U0CARLA>") {
		t.Errorf("status %d, posted %q; want 200 tagging ben and carla", recorder.Code, posted)
	}
}

func TestSlackDirectoryNeedsAUserTokenInEffect(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cfg    config.Config
		failed error
	}{
		"members by webhook":         {cfg: slackWebhookConfig(), failed: nil},
		"groups by webhook":          {cfg: slackWebhookConfig(), failed: nil},
		"members with no credential": {cfg: slackUserConfig(), failed: messaging.ErrNoCredential},
		"groups with no credential":  {cfg: slackUserConfig(), failed: messaging.ErrNoCredential},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var posted string

			fake := newFakeKept()
			fake.membersErr, fake.groupsErr = test.failed, test.failed
			handler := keptOver(t, fake, test.cfg, &posted)
			target := map[bool]string{true: "/api/slack/members", false: groupsPath}[strings.HasPrefix(name, "members")]

			// Act
			recorder := get(t, handler, target)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(failure.Detail, "Slack user token") {
				t.Errorf("status/detail = %d/%q, want 422 asking for a Slack user token", recorder.Code, failure.Detail)
			}
		})
	}
}

func TestLinkPersonChecksTheMemberInTheChannelPickedFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	deps := filledDeps()
	newFakeKept().wire(&deps)
	deps.Messaging.ChannelMembers = func(channel string) ([]loop.SlackTarget, error) {
		asked = append(asked, channel)

		return []loop.SlackTarget{ben()}, nil
	}
	handler := serveWith(t, deps, slackUserConfig(), webserver.Info{Version: testVersion})
	link := `{"owner":"ben","slack_id":"U0BEN","not_on_slack":false,"channel":"#ops"}`

	// Act
	recorder := send(t, handler, http.MethodPut, peoplePath, link)

	// Assert
	if recorder.Code != http.StatusOK || len(asked) != 1 || asked[0] != otherChannel {
		t.Errorf("status %d, members read in %v; want 200 reading #ops", recorder.Code, asked)
	}
}
