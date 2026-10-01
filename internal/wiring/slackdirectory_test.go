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
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// fakeSlack is a stand-in Slack Web API answering each directory read from a
// body keyed by its path, and counting the requests made to each. A path it
// holds answers only once released.
type fakeSlack struct {
	lock   sync.Mutex
	bodies map[string]string
	asked  map[string]int
	held   map[string]*gate
}

// gate holds a path's answers: arrived closes on the first request to it, and
// its answers wait until release closes.
type gate struct {
	arrived chan struct{}
	release chan struct{}
	once    sync.Once
}

// directoryBodies are the answers a small workspace gives: one channel, three
// members of it (one a bot users.list leaves out), a user group, and the
// workspace's ID.
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
		"/auth.test":       `{"ok":true,"team":"Example","user":"workflow","team_id":"T0EXAMPLE"}`,
	}
}

// startSlack serves bodies and returns the fake and a client pointed at it.
func startSlack(t *testing.T, bodies map[string]string) (*fakeSlack, messaging.Client) {
	t.Helper()

	slack := &fakeSlack{bodies: bodies, asked: map[string]int{}, held: map[string]*gate{}}
	server := httptest.NewServer(http.HandlerFunc(slack.answer))
	t.Cleanup(server.Close)

	settings := config.Messaging{Kind: config.KindSlack, ClientID: "1234.5678", Channel: "#dev"}
	client := messaging.New(server.Client().Do, server.URL, settings).WithToken(
		func(context.Context, config.Secret) (config.Secret, error) { return "slack-token-for-tests", nil })

	return slack, client
}

// answer counts the request, waits while its path is held, and answers from
// the body for its path — or for its path and user, when it names one.
func (s *fakeSlack) answer(writer http.ResponseWriter, request *http.Request) {
	key := request.URL.Path
	if user := request.URL.Query().Get("user"); user != "" {
		key += "?user=" + user
	}

	s.lock.Lock()
	s.asked[request.URL.Path]++
	held, body := s.held[request.URL.Path], s.bodies[key]
	s.lock.Unlock()

	if held != nil {
		held.once.Do(func() { close(held.arrived) })
		<-held.release
	}

	_, _ = writer.Write([]byte(body))
}

// hold makes path's answers wait until the test ends or release is called,
// and returns a channel closed once path is first asked.
func (s *fakeSlack) hold(t *testing.T, path string) (<-chan struct{}, func()) {
	t.Helper()

	held := &gate{arrived: make(chan struct{}), release: make(chan struct{}), once: sync.Once{}}
	release := sync.OnceFunc(func() { close(held.release) })
	t.Cleanup(release)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.held[path] = held

	return held.arrived, release
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

	return wiring.NewSlackDirectory(func() (messaging.Client, error) { return client, nil }, moment.now), moment
}

// switchable is a Slack client that settings can take away and give back.
type switchable struct {
	lock      sync.Mutex
	client    messaging.Client
	available bool
}

// current is the client while it is available, and ErrNoCredential while not.
func (s *switchable) current() (messaging.Client, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.available {
		return messaging.Client{}, messaging.ErrNoCredential
	}

	return s.client, nil
}

// set makes the client available or not.
func (s *switchable) set(available bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.available = available
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

func TestNoUserTokenReadsAsNoCredentialWhateverIsHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: true}
	directory := wiring.NewSlackDirectory(settings.current, time.Now)
	_, _ = directory.UserGroups(t.Context())

	settings.set(false)

	// Act
	_, err := directory.UserGroups(t.Context())

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
	directory := wiring.NewSlackDirectory(settings.current, time.Now)
	_, _ = directory.UserGroups(t.Context())

	settings.set(false)

	_, _ = directory.UserGroups(t.Context())

	settings.set(true)

	// Act
	_, _ = directory.UserGroups(t.Context())

	// Assert
	if asked := slack.count("/usergroups.list"); asked != 2 {
		t.Errorf("usergroups.list was asked %d times, want it read again under the new settings", asked)
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

func TestAWebhookBindsADirectoryThatHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	// Settings may switch to a Slack user token while workflow runs, so the
	// seams are there; until then they answer ErrNoCredential.
	cfg := config.Config{Messaging: config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example.com/x"}}
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	if seams.UserGroups == nil {
		t.Fatal("no directory seam is bound for a webhook, so a switch to a user token could not tag")
	}

	_, err := seams.UserGroups()

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups under a webhook = %v, want ErrNoCredential without asking for a token", err)
	}
}

func TestSettingsSwitchedToASlackUserTokenReadTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// The configuration file is the one Settings saves the user token's
	// settings to; workflow started with a webhook.
	cfg := halfLoggedIn(t)
	userToken := cfg.Messaging
	cfg.Messaging = config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example.com/x"}
	deps, controls := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	controls.UseMessagingSettings(userToken)

	// Act
	_, err := deps.Messaging.UserGroups()

	// Assert
	if !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups after switching to a user token = %v, want the token asked for", err)
	}
}

func TestSettingsSwitchedAwayFromSlackStopReadingTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, controls := wiring.Deps(t.Context(), halfLoggedIn(t), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	controls.UseMessagingSettings(config.Messaging{Kind: config.KindTeams, WebhookURL: "https://teams.example.com/x"})

	// Act
	_, err := deps.Messaging.UserGroups()

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups after switching to Teams = %v, want ErrNoCredential without asking for a token", err)
	}
}

func TestConcurrentReadsOfAChannelShareOneSetOfRequests(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	arrived, release := slack.hold(t, "/users.conversations")
	directory, _ := directoryOver(client)

	var readers sync.WaitGroup

	for range 2 {
		readers.Go(func() { _, _ = directory.ChannelMembers(t.Context(), "dev") })
	}

	<-arrived
	// The second reader has had time to ask too, while the first read waits.
	time.Sleep(50 * time.Millisecond)

	// Act
	release()
	readers.Wait()

	// Assert
	if asked := slack.requests(); asked != 3 {
		t.Errorf("Slack was asked %d times, want one channel, one members and one users.list read", asked)
	}
}

func TestRefreshDoesNotWaitForASlowReadNorKeepItsResult(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	arrived, release := slack.hold(t, "/users.list")
	directory, _ := directoryOver(client)

	var reader sync.WaitGroup

	reader.Go(func() { _, _ = directory.ChannelMembers(t.Context(), "dev") })

	<-arrived

	refreshed := make(chan struct{})

	// Act
	go func() { directory.Refresh(); close(refreshed) }()

	// Assert
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("Refresh waited for the read in flight")
	}

	release()
	reader.Wait()

	_, _ = directory.ChannelMembers(t.Context(), "dev")

	if asked := slack.count("/users.list"); asked != 2 {
		t.Errorf("users.list was asked %d times, want the read begun before Refresh not kept", asked)
	}
}

func TestAChannelInAWorkspaceTooLargeToListIsLabeledOneByOne(t *testing.T) {
	t.Parallel()

	// Arrange
	bodies := directoryBodies()
	bodies["/users.list"] = `{"ok":true,"members":[],"response_metadata":{"next_cursor":"more"}}`
	bodies["/users.info?user=U0ADA"] = `{"ok":true,"user":{"id":"U0ADA","name":"ada","profile":{"display_name":"Ada"}}}`
	bodies["/users.info?user=U0BOB"] = `{"ok":true,"user":{"id":"U0BOB","name":"bob","real_name":"Bob B","profile":{}}}`
	bodies["/users.info?user=U0BOT"] = `{"ok":true,"user":{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}}`
	slack, client := startSlack(t, bodies)
	directory, _ := directoryOver(client)

	// Act
	members, err := directory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := []loop.SlackTarget{{ID: "U0ADA", Label: "Ada"}, {ID: "U0BOB", Label: "Bob B"}}
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}

	if asked := slack.count("/users.info"); asked != 3 {
		t.Errorf("users.info was asked %d times, want once per member", asked)
	}
}
