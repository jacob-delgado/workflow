// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// SlackWorkspace is the Slack workspace the fake Slack's token is for, which
// a test keeps its owners' links in.
const SlackWorkspace = "T0ACME"

// SlackAuthTest is the path of Slack's auth.test, which names the token's
// workspace and user.
const SlackAuthTest = "/auth.test"

// SlackPostMessage is the path of Slack's chat.postMessage, which a post is
// sent to.
const SlackPostMessage = "/chat.postMessage"

// SlackLoggedInConfig is a GitHub forge through its CLI, announcing to Slack with a user
// token good for an hour, whose credentials the file keeps, so no keychain is
// read and no refresh is due.
func SlackLoggedInConfig() string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
		`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",` +
		`"refresh_token":"xoxe-1-refresh","access_token":"xoxe.xoxp-1-held",` +
		`"expires_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `","channel":"#dev"}}`
}

// TaggingScopes are the scopes workflow slack login asks for, which the fake
// Slack's auth.test lists the token as granted.
const TaggingScopes = "chat:write,users:read,channels:read,groups:read,usergroups:read"

// SlackAnswer is what the fake Slack answers on one path.
type SlackAnswer struct {
	Status int
	Body   string
}

// SlackFake is a Slack Web API on this machine, which every Slack request of
// the test's commands is sent to; it keeps the text of every post, and the
// channel each was posted in.
type SlackFake struct {
	lock    sync.Mutex
	answers map[string]SlackAnswer
	// first answers the first request on a path in place of answers.
	first    map[string]SlackAnswer
	granted  string
	asked    map[string]int
	posted   []string
	channels []string
}

// FakeSlack serves Slack's answers for the token SlackLoggedInConfig keeps —
// ana and a bot in #dev, the api-reviewers group, workspace SlackWorkspace,
// granted TaggingScopes — with changed replacing any of them by path, and
// points the commands at it until the test ends.
func FakeSlack(t *testing.T, changed map[string]SlackAnswer) *SlackFake {
	t.Helper()

	identity := `{"ok":true,"team":"Acme","user":"ana","team_id":"` + SlackWorkspace + `"}`
	answers := map[string]SlackAnswer{
		SlackAuthTest:            {http.StatusOK, identity},
		"/users.conversations":   {http.StatusOK, `{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`},
		"/conversations.members": {http.StatusOK, `{"ok":true,"members":["U0ANA","U0BOT"]}`},
		"/users.list": {http.StatusOK, `{"ok":true,"members":[` +
			`{"id":"U0ANA","name":"ana","profile":{"display_name":"Ana Souza"}},` +
			`{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}]}`},
		"/usergroups.list": {http.StatusOK, `{"ok":true,"usergroups":[{"id":"S0API","handle":"api-reviewers"}]}`},
		SlackPostMessage:   {http.StatusOK, `{"ok":true}`},
	}
	maps.Copy(answers, changed)

	slack := &SlackFake{answers: answers, first: map[string]SlackAnswer{}, granted: TaggingScopes, asked: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(slack.answer))
	t.Cleanup(server.Close)
	SetVariable(t, wiring.SlackAPIVariable, server.URL)

	return slack
}

// AnswerFirst makes the fake answer the first request on path with answer,
// and every later one as it did.
func (s *SlackFake) AnswerFirst(path string, answer SlackAnswer) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.first[path] = answer
}

// Grant makes auth.test list the token as granted scopes instead.
func (s *SlackFake) Grant(scopes string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.granted = scopes
}

// Count is how many requests the fake answered at path.
func (s *SlackFake) Count(path string) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.asked[path]
}

// Post is the one post the fake was sent, failing the test unless there was
// exactly one.
func (s *SlackFake) Post(t *testing.T) string {
	t.Helper()

	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.posted) != 1 {
		t.Fatalf("Slack was sent %d posts (%q), want one", len(s.posted), s.posted)
	}

	return s.posted[0]
}

// Channel is the channel of the one post the fake was sent, failing the test
// unless there was exactly one.
func (s *SlackFake) Channel(t *testing.T) string {
	t.Helper()

	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.channels) != 1 {
		t.Fatalf("Slack was sent %d posts (in %q), want one", len(s.channels), s.channels)
	}

	return s.channels[0]
}

// answer keeps a post's text and channel and answers from the path's answer,
// or with Slack's unknown_method.
func (s *SlackFake) answer(writer http.ResponseWriter, request *http.Request) {
	s.lock.Lock()
	s.asked[request.URL.Path]++
	granted := s.granted
	answer, known := s.answers[request.URL.Path]

	if first, once := s.first[request.URL.Path]; once && s.asked[request.URL.Path] == 1 {
		answer, known = first, true
	}
	s.lock.Unlock()

	if request.URL.Path == SlackAuthTest {
		writer.Header().Set("X-Oauth-Scopes", granted)
	}

	if request.URL.Path == SlackPostMessage {
		var message struct {
			Channel string `json:"channel"`
			Text    string `json:"text"`
		}

		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &message)

		s.lock.Lock()
		s.posted = append(s.posted, message.Text)
		s.channels = append(s.channels, message.Channel)
		s.lock.Unlock()
	}

	if !known {
		answer = SlackAnswer{http.StatusOK, `{"ok":false,"error":"unknown_method"}`}
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(answer.Status)
	_, _ = writer.Write([]byte(answer.Body))
}

// SlackClientID is the Slack app's client ID the login is given.
const SlackClientID = "1234.5678"

// Webhook is a local incoming webhook over https that accepts every post, as
// Slack's does, and counts them in posts. The process trusts its certificate
// (see Main). The count is atomic because `task test` runs -race and the
// handler runs on its own goroutine.
func Webhook(t *testing.T, posts *atomic.Int32) string {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)

		posts.Add(1)

		_, _ = writer.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	return server.URL + "/services/x"
}

// fakePull is the number of the pull request the fake forges answer with.
const fakePull = 7

// AnnouncedEarlier is a home whose store remembers the fake forge's pull request
// 7 announced at moment, as an earlier session — of the interface or of
// announce — leaves it: keyed by the repository's host and path.
func AnnouncedEarlier(t *testing.T, moment messaging.Moment) string {
	t.Helper()

	home := t.TempDir()

	err := RecordAnnounced(t.Context(), home, moment)
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	return home
}

// RecordAnnounced records, in the store under home, the fake forge's pull
// request 7 announced at moment.
func RecordAnnounced(ctx context.Context, home string, moment messaging.Moment) error {
	env := IsolatedEnvironment(Place{Home: home})

	dir, err := store.Dir(runtime.GOOS, home, func(name string) (string, bool) {
		value, ok := env[name]

		return value, ok
	})
	if err != nil {
		return fmt.Errorf("finding the store under %s: %w", home, err)
	}

	return store.New(dir, false).RecordAnnounce(ctx, "github.com/owner/repo",
		store.Announce{Pull: fakePull, Moment: int(moment)}, time.Now())
}
