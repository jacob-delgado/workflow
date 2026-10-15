// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
