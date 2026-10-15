// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// The Slack directory — a channel's members, the workspace's users and user
// groups — is read once and held for the session, so the announcement preview
// can open many times without paging Slack's directory each time. These tests
// drive the cache against a stand-in Slack and count what it was asked.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

func TestChannelMembersAreTheChannelsPeopleByTheirSlackNames(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}
}

func TestTheDirectoryIsReadOnceWithinTenMinutes(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, moment := directoryOver(client)
	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")
	_, _ = slackDirectory.UserGroups(t.Context())
	before := slack.requests()

	moment.advance(9 * time.Minute)

	// Act
	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")
	_, _ = slackDirectory.UserGroups(t.Context())

	// Assert
	if after := slack.requests(); after != before {
		t.Errorf("Slack was asked %d more times, want none within the ten minutes", after-before)
	}
}

func TestTheDirectoryIsReadAgainOnceTenMinutesPass(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, moment := directoryOver(client)
	_, _ = slackDirectory.UserGroups(t.Context())

	moment.advance(10 * time.Minute)

	// Act
	_, _ = slackDirectory.UserGroups(t.Context())

	// Assert
	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want 2 once the cache ran out", asked)
	}
}

func TestRefreshReadsTheDirectoryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	slackDirectory.Refresh()

	// Act
	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	// Assert
	if asked := slack.count("/users.list"); asked != 2 {
		t.Errorf("users.list was asked %d times, want 2 after a refresh", asked)
	}
}

func TestNoUserTokenReadsAsNoCredentialWhateverIsHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: true}
	slackDirectory := directory.New(settings.current, time.Now)
	_, _ = slackDirectory.UserGroups(t.Context())

	settings.set(false)

	// Act
	_, err := slackDirectory.UserGroups(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) {
		t.Errorf("UserGroups after the user token went = %v, want ErrNoCredential", err)
	}
}

func TestTheDirectoryIsReadAfreshWhenAUserTokenComesBack(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: true}
	slackDirectory := directory.New(settings.current, time.Now)
	_, _ = slackDirectory.UserGroups(t.Context())

	settings.set(false)

	_, _ = slackDirectory.UserGroups(t.Context())

	settings.set(true)

	// Act
	_, _ = slackDirectory.UserGroups(t.Context())

	// Assert
	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want it read again under the new settings", asked)
	}
}

func TestUserGroupsAreTheWorkspacesGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)

	// Act
	groups, err := slackDirectory.UserGroups(t.Context())

	// Assert
	want := []messaging.SlackTarget{{ID: "S0CP", Label: "control-plane-pod"}}
	if err != nil || !slices.Equal(groups, want) {
		t.Errorf("UserGroups = %v, %v; want %v", groups, err, want)
	}
}

func TestAMissingScopeIsReportedAndNotHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	bodies := directoryBodies()
	bodies["/usergroups.list"] = `{"ok":false,"error":"missing_scope","needed":"usergroups:read"}`
	slack, client := startSlack(t, bodies)
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.UserGroups(t.Context())

	// Act
	_, err := slackDirectory.UserGroups(t.Context())

	// Assert
	var missing *messaging.MissingScopeError
	if !errors.As(err, &missing) || missing.Needed != "usergroups:read" {
		t.Errorf("UserGroups = %v, want the missing usergroups:read scope", err)
	}

	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want a failure asked again", asked)
	}
}

func TestAReadLateInTheTenMinutesIsHeldForTenMinutesOfItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, moment := directoryOver(client)
	_, _ = slackDirectory.UserGroups(t.Context())

	moment.advance(9 * time.Minute)

	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	moment.advance(2 * time.Minute)

	// Act
	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	// Assert
	if asked := slack.count("/users.list"); asked != 1 {
		t.Errorf("users.list was asked %d times, want once: it was read two minutes ago", asked)
	}
}
