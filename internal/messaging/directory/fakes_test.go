// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

// The stand-in Slack, the clocks and the answers the directory's tests share.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
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

	slack := newFakeSlack(bodies)
	server := httptest.NewServer(http.HandlerFunc(slack.answer))
	t.Cleanup(server.Close)

	return slack, slackClient(server.Client().Do, server.URL)
}

// newFakeSlack is a stand-in Slack answering from bodies, asked nothing yet.
func newFakeSlack(bodies map[string]string) *fakeSlack {
	return &fakeSlack{bodies: bodies, asked: map[string]int{}, held: map[string]*gate{}, limited: map[string]limit{}}
}

// slackClient is a client reading with a Slack user token, through do, from
// the Slack API at base.
func slackClient(do messaging.Doer, base string) messaging.Client {
	settings := config.Messaging{Kind: config.KindSlack, ClientID: "1234.5678", Channel: "#dev"}

	return messaging.New(do, base, settings).WithToken(
		func(context.Context, config.Secret) (config.Secret, error) { return "slack-token-for-tests", nil })
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

// channelMembers are #dev's members as the directory labels them.
func channelMembers() []messaging.SlackTarget {
	return []messaging.SlackTarget{{ID: "U0ADA", Label: "Ada"}, {ID: "U0BOB", Label: "Bob B"}}
}
