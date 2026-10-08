// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

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
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

// fakeSlack is a stand-in Slack Web API answering each directory read from a
// body keyed by its path, and counting the requests made to each. A path it
// holds answers only once released.
type fakeSlack struct {
	lock    sync.Mutex
	bodies  map[string]string
	asked   map[string]int
	held    map[string]*gate
	limited map[string]limit
}

// limit is a path and user Slack asks to wait on: the next times answers
// are 429s naming retryAfter.
type limit struct {
	times      int
	retryAfter string
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

	slack := &fakeSlack{bodies: bodies, asked: map[string]int{}, held: map[string]*gate{}, limited: map[string]limit{}}
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
	held, body, limited := s.held[request.URL.Path], s.bodies[key], s.limited[key]

	if limited.times > 0 {
		s.limited[key] = limit{times: limited.times - 1, retryAfter: limited.retryAfter}
	}
	s.lock.Unlock()

	if limited.times > 0 {
		writer.Header().Set("Retry-After", limited.retryAfter)
		writer.WriteHeader(http.StatusTooManyRequests)

		return
	}

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

// limit makes the next times answers to key — a path, and ?user= and an ID
// when it names one — 429s asking to wait retryAfter seconds.
func (s *fakeSlack) limit(key string, times int, retryAfter string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.limited[key] = limit{times: times, retryAfter: retryAfter}
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
func directoryOver(client messaging.Client) (*directory.Slack, *clock) {
	moment := &clock{at: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}

	return directory.New(func() (messaging.Client, error) { return client, nil }, moment.now), moment
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

// joinWait is how long a test waits for a second reader to find a read in
// flight before failing: a failsafe, never the pace of the test.
const joinWait = 5 * time.Second

// watchedClock is a fixed clock that says when it is read. A reader reads the
// clock when it finds a read of what it asks for already begun, to see
// whether it has expired, and not before then.
type watchedClock struct {
	at   time.Time
	read chan struct{}
}

// newWatchedClock is a watchedClock that has not been read.
func newWatchedClock() watchedClock {
	return watchedClock{at: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), read: make(chan struct{}, 1)}
}

func (c watchedClock) now() time.Time {
	select {
	case c.read <- struct{}{}:
	default:
	}

	return c.at
}

func TestConcurrentReadsOfAChannelShareOneSetOfRequests(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second reader begins once the first is waiting on Slack, and the
	// read is let go once the second has found it in flight.
	slack, client := startSlack(t, directoryBodies())
	arrived, release := slack.hold(t, "/users.conversations")
	clock := newWatchedClock()
	slackDirectory := directory.New(func() (messaging.Client, error) { return client, nil }, clock.now)

	var readers sync.WaitGroup

	readers.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })
	<-arrived
	readers.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })

	select {
	case <-clock.read:
	case <-time.After(joinWait):
		t.Fatal("the second reader never found the read in flight")
	}

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
	slackDirectory, _ := directoryOver(client)

	var reader sync.WaitGroup

	reader.Go(func() { _, _ = slackDirectory.ChannelMembers(t.Context(), "dev") })

	<-arrived

	refreshed := make(chan struct{})

	// Act
	go func() { slackDirectory.Refresh(); close(refreshed) }()

	// Assert
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("Refresh waited for the read in flight")
	}

	release()
	reader.Wait()

	_, _ = slackDirectory.ChannelMembers(t.Context(), "dev")

	if asked := slack.count("/users.list"); asked != 2 {
		t.Errorf("users.list was asked %d times, want the read begun before Refresh not kept", asked)
	}
}

func TestAChannelInAWorkspaceTooLargeToListIsLabeledOneByOne(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slackDirectory, _ := directoryOver(client)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}

	if asked := slack.count("/users.info"); asked != 3 {
		t.Errorf("users.info was asked %d times, want once per member", asked)
	}
}

// channelMembers are #dev's members as the directory labels them.
func channelMembers() []messaging.SlackTarget {
	return []messaging.SlackTarget{{ID: "U0ADA", Label: "Ada"}, {ID: "U0BOB", Label: "Bob B"}}
}

// tooLargeToList are the answers of a workspace past users.list's page cap,
// whose channel's members users.info labels one by one.
func tooLargeToList() map[string]string {
	bodies := directoryBodies()
	bodies["/users.list"] = `{"ok":true,"members":[],"response_metadata":{"next_cursor":"more"}}`
	bodies["/users.info?user=U0ADA"] = `{"ok":true,"user":{"id":"U0ADA","name":"ada","profile":{"display_name":"Ada"}}}`
	bodies["/users.info?user=U0BOB"] = `{"ok":true,"user":{"id":"U0BOB","name":"bob","real_name":"Bob B","profile":{}}}`
	bodies["/users.info?user=U0BOT"] = `{"ok":true,"user":{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}}`

	return bodies
}

func TestAMemberSlackAsksToWaitForIsLabeledOnceTheWaitIsOver(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", 1, "1")

	slackDirectory, _ := directoryOver(client)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v after waiting as Slack asked", members, err, want)
	}
}

func TestAWaitTooLongFailsTheReadAndKeepsWhoWasLabeled(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", 1, "3600")

	slackDirectory, _ := directoryOver(client)
	started := time.Now()

	// Act
	_, failed := slackDirectory.ChannelMembers(t.Context(), "#dev")
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	if !errors.Is(failed, httpx.ErrRateLimited) || time.Since(started) > time.Minute {
		t.Errorf("the first read = %v after %s, want Slack's rate limit at once", failed, time.Since(started))
	}

	want := channelMembers()
	if err != nil || !slices.Equal(members, want) {
		t.Errorf("ChannelMembers = %v, %v; want %v", members, err, want)
	}

	if asked := slack.count("/users.info"); asked != 4 {
		t.Errorf("users.info was asked %d times, want Ada's label kept and Bob's asked again", asked)
	}
}

func TestAWaitForSlackEndsWithTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, tooLargeToList())
	slack.limit("/users.info?user=U0BOB", 1, "30")

	slackDirectory, _ := directoryOver(client)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	t.Cleanup(cancel)

	// Act
	_, err := slackDirectory.ChannelMembers(ctx, "#dev")

	// Assert
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ChannelMembers = %v, want the read's own deadline rather than the whole wait", err)
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

func TestTheWorkspaceIsReadOnceWithTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	// Act
	workspace, err := slackDirectory.Workspace(t.Context())

	// Assert
	if err != nil || workspace != "T0EXAMPLE" {
		t.Errorf("Workspace = %q, %v; want T0EXAMPLE", workspace, err)
	}

	if asked := slack.count("/auth.test"); asked != 1 {
		t.Errorf("auth.test was asked %d times, want once", asked)
	}
}

func TestRefreshReadsTheWorkspaceAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	slackDirectory.Refresh()

	// Act
	_, _ = slackDirectory.Workspace(t.Context())

	// Assert
	if asked := slack.count("/auth.test"); asked != 2 {
		t.Errorf("auth.test was asked %d times, want it asked again after a refresh", asked)
	}
}

func TestAWorkspaceSlackWouldNotNameIsAnErrorAskedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	bodies := directoryBodies()
	bodies["/auth.test"] = `{"ok":false,"error":"invalid_auth"}`
	slack, client := startSlack(t, bodies)
	slackDirectory, _ := directoryOver(client)
	_, _ = slackDirectory.Workspace(t.Context())

	// Act
	workspace, err := slackDirectory.Workspace(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || workspace != "" {
		t.Errorf("Workspace = %q, %v; want Slack's refusal", workspace, err)
	}

	if asked := slack.count("/auth.test"); asked != 2 {
		t.Errorf("auth.test was asked %d times, want a failure asked again", asked)
	}
}

func TestNoUserTokenHasNoWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	_, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: false}
	slackDirectory := directory.New(settings.current, time.Now)

	// Act
	_, err := slackDirectory.Workspace(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) {
		t.Errorf("Workspace with no user token = %v, want ErrNoCredential", err)
	}
}
