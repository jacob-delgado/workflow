// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/wiring"
)

// slackWorkspace is the Slack workspace the fake Slack's token is for, which
// keptLinks keeps its links in.
const slackWorkspace = "T0ACME"

// slackLoggedInConfig is ownedRepo's forge, announcing to Slack with a user
// token good for an hour, whose credentials the file keeps, so no keychain is
// read and no refresh is due.
func slackLoggedInConfig() string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
		`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",` +
		`"refresh_token":"xoxe-1-refresh","access_token":"xoxe.xoxp-1-held",` +
		`"expires_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `","channel":"#dev"}}`
}

// slackAnswer is what the fake Slack answers on one path.
type slackAnswer struct {
	status int
	body   string
}

// slackFake is a Slack Web API on this machine, which every Slack request of
// the test's commands is sent to; it keeps the text of every post.
type slackFake struct {
	lock    sync.Mutex
	answers map[string]slackAnswer
	posted  []string
}

// fakeSlack serves Slack's answers for the token slackLoggedInConfig keeps —
// ana and a bot in #dev, the api-reviewers group, workspace slackWorkspace —
// with changed replacing any of them by path, and points the commands at it
// until the test ends.
func fakeSlack(t *testing.T, changed map[string]slackAnswer) *slackFake {
	t.Helper()

	identity := `{"ok":true,"team":"Acme","user":"ana","team_id":"` + slackWorkspace + `"}`
	answers := map[string]slackAnswer{
		"/auth.test":             {http.StatusOK, identity},
		"/users.conversations":   {http.StatusOK, `{"ok":true,"channels":[{"id":"C0DEV","name":"dev"}]}`},
		"/conversations.members": {http.StatusOK, `{"ok":true,"members":["U0ANA","U0BOT"]}`},
		"/users.list": {http.StatusOK, `{"ok":true,"members":[` +
			`{"id":"U0ANA","name":"ana","profile":{"display_name":"Ana Souza"}},` +
			`{"id":"U0BOT","name":"robot","is_bot":true,"profile":{}}]}`},
		"/usergroups.list":  {http.StatusOK, `{"ok":true,"usergroups":[{"id":"S0API","handle":"api-reviewers"}]}`},
		"/chat.postMessage": {http.StatusOK, `{"ok":true}`},
	}
	maps.Copy(answers, changed)

	slack := &slackFake{answers: answers}
	server := httptest.NewServer(http.HandlerFunc(slack.answer))
	t.Cleanup(server.Close)
	t.Setenv(wiring.SlackAPIVariable, server.URL)

	return slack
}

// answer keeps a post's text and answers from the path's answer, or with
// Slack's unknown_method.
func (s *slackFake) answer(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/chat.postMessage" {
		var message struct {
			Text string `json:"text"`
		}

		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &message)

		s.lock.Lock()
		s.posted = append(s.posted, message.Text)
		s.lock.Unlock()
	}

	answer, known := s.answers[request.URL.Path]
	if !known {
		answer = slackAnswer{http.StatusOK, `{"ok":false,"error":"unknown_method"}`}
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(answer.status)
	_, _ = writer.Write([]byte(answer.body))
}

// post is the one post the fake was sent, failing the test unless there was
// exactly one.
func (s *slackFake) post(t *testing.T) string {
	t.Helper()

	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.posted) != 1 {
		t.Fatalf("Slack was sent %d posts (%q), want one", len(s.posted), s.posted)
	}

	return s.posted[0]
}

// untagged reports a post that tags no one on Slack.
func untagged(post string) bool {
	return !strings.Contains(post, "<@") && !strings.Contains(post, "<!subteam^")
}
