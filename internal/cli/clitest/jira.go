// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// JiraFixture is the body Jira Data Center returns for /rest/api/2/myself.
const JiraFixture = `{"key":"JIRAUSER10100","name":"fred","displayName":"Fred F. User","active":true}`

// SlackWebhook is a webhook configuration doctor accepts and never posts to.
const SlackWebhook = `"messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}`

// WriteConfigFor writes a configuration pointing Jira at baseURL.
func WriteConfigFor(t *testing.T, dir, baseURL string) {
	t.Helper()

	WriteFile(t, dir, `{"jira": {"base_url": "`+baseURL+`", "token": "jira-token-for-tests"}, `+SlackWebhook+`}`)
}

// JiraServer answers one myself request with status and body, recording whether
// it was reached at all. The flag is atomic because `task test` runs -race and
// the handler runs on its own goroutine.
func JiraServer(t *testing.T, status int, body string, reached *atomic.Bool) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		reached.Store(true)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

// WorkingJira is a local Jira that accepts the credential, so an online check
// never reaches a real host.
func WorkingJira(t *testing.T) string {
	t.Helper()

	return JiraServer(t, http.StatusOK, JiraFixture, new(atomic.Bool)).URL
}

// SearchFixture is the Jira body for a search of the issues assigned to you.
func SearchFixture(keys ...string) string {
	issues := make([]string, 0, len(keys))
	for _, key := range keys {
		issues = append(issues, `{"key":"`+key+`","fields":{"summary":"x",`+
			`"issuetype":{"name":"Bug"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	}

	return `{"total":` + strconv.Itoa(len(keys)) + `,"issues":[` + strings.Join(issues, ",") + `]}`
}

// StatusInReview is the review status a test configures `pr` to offer.
const StatusInReview = "In Review"

// The writes a fake Jira records, by kind.
const (
	WriteLink       = "link"
	WriteTransition = "transition"
)

// JiraWrite is one write a fake Jira was asked to make: the path it was sent to,
// and the body.
type JiraWrite struct {
	Kind string
	Path string
	Body string
}

// JiraWrites records, in order, the writes a fake Jira was asked to make: each
// link added to an issue and each transition applied. A mutex guards them
// because `task test` runs -race and the handler answers on its own goroutine.
type JiraWrites struct {
	mu     sync.Mutex
	writes []JiraWrite
}

// Record keeps request as a write of kind, with its path and body.
func (w *JiraWrites) Record(kind string, request *http.Request) {
	body, _ := io.ReadAll(request.Body)

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writes = append(w.writes, JiraWrite{Kind: kind, Path: request.URL.Path, Body: string(body)})
}

// Kinds are the kinds of every write, in the order they were made.
func (w *JiraWrites) Kinds() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	kinds := make([]string, 0, len(w.writes))
	for _, write := range w.writes {
		kinds = append(kinds, write.Kind)
	}

	return kinds
}

// Of is every write of kind, in order.
func (w *JiraWrites) Of(kind string) []JiraWrite {
	w.mu.Lock()
	defer w.mu.Unlock()

	var made []JiraWrite

	for _, write := range w.writes {
		if write.Kind == kind {
			made = append(made, write)
		}
	}

	return made
}

// Applied is how many transitions were applied.
func (w *JiraWrites) Applied() int {
	return len(w.Of(WriteTransition))
}

// SentBody is the body of the last transition applied, or empty for none.
func (w *JiraWrites) SentBody() string {
	sent := w.Of(WriteTransition)
	if len(sent) == 0 {
		return ""
	}

	return sent[len(sent)-1].Body
}

// IssueFixture is the Jira Data Center body for one issue: its key, type and
// summary, which is all the branch name is composed from.
func IssueFixture(key, issueType, summary string) string {
	return `{"key":"` + key + `","fields":{"summary":"` + summary + `",` +
		`"issuetype":{"name":"` + issueType + `"},` +
		`"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`
}
