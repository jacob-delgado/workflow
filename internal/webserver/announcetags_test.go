// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The announcement preview proposes whom to tag; the post tags the linked
// owners and the groups checked, each one the announcement offered.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// postTagged is a server over fake whose posts land in posted.
func postTagged(t *testing.T, fake *fakeKept, posted *string) http.Handler {
	t.Helper()

	deps := filledDeps()
	fake.wire(&deps)
	deps.Post = func(_, text string) error {
		*posted = text

		return nil
	}

	return serve(t, deps, config.Default())
}

// announceWith posts the announcement previewed, asking for groups.
func announceWith(t *testing.T, handler http.Handler, groups []string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(api.AnnounceRequest{
		Channel: "#dev", Text: nil, Mentions: &api.AnnounceMentions{Groups: groups},
	})
	if err != nil {
		t.Fatalf("encoding the announce request: %v", err)
	}

	return send(t, handler, http.MethodPost, "/api/announce", string(body))
}

func TestGetAnnouncementProposesWhomToTag(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.links = append(fake.links, ownerLinkedTo(ownedTeam, podGroup()))
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	want := &api.AnnouncementTagging{
		Available: true,
		Owners: []api.OwnerTag{
			{Owner: benOwner, Kind: api.User, State: api.Unlinked, Slack: nil},
			{Owner: carlaOwner, Kind: api.User, State: api.Linked, Slack: slackTarget(carla())},
			{Owner: ownedTeam, Kind: api.Team, State: api.Linked, Slack: slackTarget(podGroup())},
		},
		Groups: []api.GroupTag{
			{Slack: api.SlackTarget(apiReviews()), Checked: false, FromOwners: false},
			{Slack: api.SlackTarget(podGroup()), Checked: true, FromOwners: true},
		},
	}
	if got := decode[api.Announcement](t, recorder).Tagging; !reflect.DeepEqual(got, want) {
		t.Errorf("tagging = %+v, want %+v", got, want)
	}
}

func TestGetAnnouncementTagsNoOneWithoutASlackUserToken(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	if got := decode[api.Announcement](t, recorder).Tagging; got != nil {
		t.Errorf("tagging = %+v, want none without a Slack user token", got)
	}
}

func TestGetAnnouncementTagsNoOneWhileTheDirectoryHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	// The directory is bound whatever the settings, and answers no credential
	// while they post with no Slack user token.
	fake := newFakeKept()
	fake.membersErr = messaging.ErrNoCredential
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	if got := decode[api.Announcement](t, recorder).Tagging; got != nil {
		t.Errorf("tagging = %+v, want none while there is no Slack user token", got)
	}
}

func TestAnnounceTagsNoOneWhileTheDirectoryHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	fake.membersErr = messaging.ErrNoCredential
	handler := postTagged(t, fake, &posted)

	// Act
	recorder := announceWith(t, handler, []string{apiGroupID})

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || posted != "" {
		t.Errorf("status %d, posted %q; want 422 and nothing posted", recorder.Code, posted)
	}
}

func TestGetAnnouncementTagsNoOneUnlessReadyForReview(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	newFakeKept().wire(&deps)
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{State: forge.CIFailed}, nil }
	handler := serve(t, deps, config.Default())

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	got := decode[api.Announcement](t, recorder).Tagging
	if got == nil || got.Available || len(got.Owners) != 0 {
		t.Errorf("tagging = %+v, want it unavailable with no owners for red CI", got)
	}
}

func TestGetAnnouncementNamesTheScopeToLinkAnOwner(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := newFakeKept()
	fake.membersErr = &messaging.MissingScopeError{Needed: usersRead}
	handler := keptServer(t, fake, webserver.Info{Version: testVersion})

	// Act
	recorder := get(t, handler, "/api/announcement")

	// Assert
	got := decode[api.Announcement](t, recorder).Tagging
	if got == nil || got.MissingScope == nil || *got.MissingScope != usersRead {
		t.Errorf("tagging = %+v, want it naming users:read", got)
	}
}

func TestAnnounceTagsTheLinkedOwnersAndTheGroupsChecked(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	fake := newFakeKept()
	handler := postTagged(t, fake, &posted)

	// Act
	recorder := announceWith(t, handler, []string{apiGroupID})

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", recorder.Code, recorder.Body.String())
	}

	if !strings.HasSuffix(posted, "\ncc <@U0CARLA> <!subteam^S0API>") {
		t.Errorf("posted %q, want it to end tagging carla() and the group checked", posted)
	}

	if !reflect.DeepEqual(fake.recorded, [][]string{{apiGroupID}}) {
		t.Errorf("recorded groups %v, want the choice remembered", fake.recorded)
	}
}

func TestAnnounceRefusesAGroupTheAnnouncementDidNotOffer(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	handler := postTagged(t, newFakeKept(), &posted)

	// Act
	recorder := announceWith(t, handler, []string{"S0POD"})

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || posted != "" {
		t.Errorf("status %d, posted %q; want 422 and nothing posted", recorder.Code, posted)
	}
}

func TestAnnounceRefusesMentionsWhereNoOneIsTagged(t *testing.T) {
	t.Parallel()

	// Arrange
	var posted string

	deps := filledDeps()
	deps.Post = func(_, text string) error {
		posted = text

		return nil
	}
	handler := serve(t, deps, config.Default())

	// Act
	recorder := announceWith(t, handler, []string{})

	// Assert
	failure := decode[api.Problem](t, recorder)

	refused := recorder.Code == http.StatusUnprocessableEntity && strings.Contains(failure.Detail, "tags no one")
	if !refused || posted != "" {
		t.Errorf("status/detail %d/%q, posted %q; want 422 and nothing posted", recorder.Code, failure.Detail, posted)
	}
}
