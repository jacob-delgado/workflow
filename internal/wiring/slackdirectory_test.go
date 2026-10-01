// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The Slack directory — a channel's members, the workspace's users and user
// groups — is read once and held for the session, so the announcement preview
// can open many times without paging Slack's directory each time. These tests
// drive the cache against a stand-in Slack and count what it was asked.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// fakeSlack is a stand-in Slack Web API answering each directory read from a
// body keyed by its path, and counting the requests made to each.
type fakeSlack struct {
	lock   sync.Mutex
	bodies map[string]string
	asked  map[string]int
}

// directoryBodies are the answers a small workspace gives: one channel, three
// members of it (one a bot users.list leaves out), and two user groups.
func directoryBodies() map[string]string {
	return map[string]string{
		"/users.conversations":   `{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`,
		"/conversations.members": `{"ok":true,"members":["U0ADA","U0BOB","U0BOT"]}`,
		"/users.list": `{"ok":true,"members":[
			{"id":"U0BOB","name":"bob","real_name":"Bob B","profile":{}},
			{"id":"U0ADA","name":"ada","profile":{"display_name":"Ada"}},
			{"id":"U0CY","name":"cy","profile":{}},
			{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}]}`,
		"/usergroups.list": `{"ok":true,"usergroups":[{"id":"S0CP","handle":"control-plane-pod"}]}`,
	}
}

// startSlack serves bodies and returns the fake and a client pointed at it.
func startSlack(t *testing.T, bodies map[string]string) (*fakeSlack, messaging.Client) {
	t.Helper()

	slack := &fakeSlack{bodies: bodies, asked: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		slack.lock.Lock()
		defer slack.lock.Unlock()

		slack.asked[request.URL.Path]++
		_, _ = writer.Write([]byte(slack.bodies[request.URL.Path]))
	}))
	t.Cleanup(server.Close)

	settings := config.Messaging{Kind: config.KindSlack, ClientID: "1234.5678", Channel: "#dev"}
	client := messaging.New(server.Client().Do, server.URL, settings).WithToken(
		func(context.Context, config.Secret) (config.Secret, error) { return "slack-token-for-tests", nil })

	return slack, client
}

// requests is how many requests the fake has answered, over every path.
func (s *fakeSlack) requests() int {
	s.lock.Lock()
	defer s.lock.Unlock()

	total := 0
	for _, count := range s.asked {
		total += count
	}

	return total
}

// count is how many requests the fake has answered at path.
func (s *fakeSlack) count(path string) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.asked[path]
}

// clock is a settable now.
type clock struct {
	lock sync.Mutex
	at   time.Time
}

func (c *clock) now() time.Time {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.at
}

func (c *clock) advance(by time.Duration) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.at = c.at.Add(by)
}

// directoryOver is a session directory over client, on a clock that starts now.
func directoryOver(client messaging.Client) (*wiring.SlackDirectory, *clock) {
	moment := &clock{at: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}

	return wiring.NewSlackDirectory(func() messaging.Client { return client }, moment.now), moment
}

func TestChannelMembersAreTheChannelsPeopleByTheirSlackNames(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	directory, _ := directoryOver(client)

	// Act
	members, err := directory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := []loop.SlackTarget{{ID: "U0ADA", Label: "Ada"}, {ID: "U0BOB", Label: "Bob B"}}
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}
}

func TestTheDirectoryIsReadOnceWithinTenMinutes(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	directory, moment := directoryOver(client)
	_, _ = directory.ChannelMembers(t.Context(), "dev")
	_, _ = directory.UserGroups(t.Context())
	before := slack.requests()

	moment.advance(9 * time.Minute)

	// Act
	_, _ = directory.ChannelMembers(t.Context(), "dev")
	_, _ = directory.UserGroups(t.Context())

	// Assert
	if after := slack.requests(); after != before {
		t.Errorf("Slack was asked %d more times, want none within the ten minutes", after-before)
	}
}

func TestTheDirectoryIsReadAgainOnceTenMinutesPass(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	directory, moment := directoryOver(client)
	_, _ = directory.UserGroups(t.Context())

	moment.advance(10 * time.Minute)

	// Act
	_, _ = directory.UserGroups(t.Context())

	// Assert
	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want 2 once the cache ran out", asked)
	}
}

func TestRefreshReadsTheDirectoryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	directory, _ := directoryOver(client)
	_, _ = directory.ChannelMembers(t.Context(), "dev")

	directory.Refresh()

	// Act
	_, _ = directory.ChannelMembers(t.Context(), "dev")

	// Assert
	if asked := slack.count("/users.list"); asked != 2 {
		t.Errorf("users.list was asked %d times, want 2 after a refresh", asked)
	}
}

func TestUserGroupsAreTheWorkspacesGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	directory, _ := directoryOver(client)

	// Act
	groups, err := directory.UserGroups(t.Context())

	// Assert
	want := []loop.SlackTarget{{ID: "S0CP", Label: "control-plane-pod"}}
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
	directory, _ := directoryOver(client)
	_, _ = directory.UserGroups(t.Context())

	// Act
	_, err := directory.UserGroups(t.Context())

	// Assert
	var missing *messaging.MissingScopeError
	if !errors.As(err, &missing) || missing.Needed != "usergroups:read" {
		t.Errorf("UserGroups = %v, want the missing usergroups:read scope", err)
	}

	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want a failure asked again", asked)
	}
}

func TestTheDirectorySeamsAreBoundForASlackUserToken(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)

	// Act
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Assert
	if seams.ChannelMembers == nil || seams.UserGroups == nil || seams.RefreshDirectory == nil {
		t.Error("a directory seam is nil under a Slack user token")
	}
}

func TestThereAreNoDirectorySeamsForAWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{Messaging: config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example.com/x"}}

	// Act
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Assert
	if seams.ChannelMembers != nil || seams.UserGroups != nil || seams.RefreshDirectory != nil {
		t.Error("a directory seam is bound for a webhook, which cannot read the directory")
	}
}
