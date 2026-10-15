// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// The writes trackerJira records, by kind, beside clitest's WriteLink and
// WriteTransition.
const (
	writeAssign  = "assign"
	writeWorklog = "worklog"
)

// trackedIssue is the issue the issue commands' fake Jira holds.
const trackedIssue = "PROJ-7"

// movedIssue is the key trackedIssue had in the project it moved from, which
// Jira still reads, answering with the issue under the key it has now.
const movedIssue = "OLD-3"

// trackedDetail is trackedIssue read in full, shaped like Data Center's GET
// /rest/api/2/issue/{key}: two of its three comments folded in.
const trackedDetail = `{"key":"` + trackedIssue + `","fields":{"summary":"Fix login",` +
	`"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},` +
	`"issuetype":{"name":"Bug"},"priority":{"name":"High"},` +
	`"description":"Login fails when the token is empty.",` +
	`"reporter":{"displayName":"Ana Lopez"},"assignee":{"displayName":"Bo Chen"},` +
	`"comment":{"startAt":0,"maxResults":2,"total":3,"comments":[` +
	`{"author":{"displayName":"Ana Lopez"},"body":"Repro'd on 8.2.1","created":"2026-09-03T12:00:00.000+0000"},` +
	`{"author":{"displayName":"Bo Chen"},"body":"Patch up shortly","created":"2026-09-04T12:00:00.000+0000"}]}}}`

// trackerMoves offers In Progress, a fieldless move to In Review, and a move
// to Done that asks for a resolution and a fix version.
const trackerMoves = `{"transitions":[` +
	`{"id":"11","name":"Start Progress",` +
	`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}},` +
	`{"id":"31","name":"Start Review",` +
	`"to":{"name":"` + clitest.StatusInReview + `","statusCategory":{"key":"indeterminate"}},"fields":{}},` +
	`{"id":"41","name":"Close",` +
	`"to":{"name":"Done","statusCategory":{"key":"done"}},"fields":{` +
	`"resolution":{"required":true,"name":"Resolution","schema":{"type":"resolution"}},` +
	`"fixVersions":{"required":true,"name":"Fix Version/s","schema":{"type":"array","items":"version"}}}}]}`

// trackerAnswers are what trackerJira answers: its search's body, and the
// status of each kind of request, where 0 is the success Jira answers with.
type trackerAnswers struct {
	search      string
	searchCode  int
	issueCode   int
	movesCode   int
	writeCode   int
	transitions string
}

// trackerAsked records what a fake Jira was asked: the JQL of each search, and
// each write, by kind. A mutex guards it because `task test` runs -race and
// the handler answers on its own goroutine.
type trackerAsked struct {
	clitest.JiraWrites

	searchMu sync.Mutex
	searched []string
}

// queries are the JQL of every search, in order.
func (a *trackerAsked) queries() []string {
	a.searchMu.Lock()
	defer a.searchMu.Unlock()

	return append([]string(nil), a.searched...)
}

// trackerJira is a Jira for the issue commands: a search answering
// answers.search, trackedIssue's detail, its transitions, and assigning it,
// logging work on it and moving it, each recorded.
func trackerJira(t *testing.T, answers trackerAnswers) (string, *trackerAsked) {
	t.Helper()

	asked := &trackerAsked{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if request.URL.Path == "/rest/api/2/search" {
			asked.search(writer, request, answers)

			return
		}

		answerIssue(writer, request, asked, answers)
	}))
	t.Cleanup(server.Close)

	return server.URL, asked
}

// search records a search's JQL and answers it.
func (a *trackerAsked) search(writer http.ResponseWriter, request *http.Request, answers trackerAnswers) {
	jql := request.URL.Query().Get("jql")

	a.searchMu.Lock()
	a.searched = append(a.searched, jql)
	a.searchMu.Unlock()

	writer.WriteHeader(cmp.Or(answers.searchCode, http.StatusOK))
	_, _ = writer.Write([]byte(answers.search))
}

// answerIssue answers a request about trackedIssue: its writes, recorded, its
// transitions, its comments past the first page, and its detail, asked by
// either of its keys.
func answerIssue(writer http.ResponseWriter, request *http.Request, asked *trackerAsked, answers trackerAnswers) {
	path := request.URL.Path

	switch {
	case strings.HasSuffix(path, "/transitions") && request.Method == http.MethodPost:
		asked.Record(clitest.WriteTransition, request)
		writer.WriteHeader(cmp.Or(answers.writeCode, http.StatusNoContent))
	case strings.HasSuffix(path, "/assignee"):
		asked.Record(writeAssign, request)
		writer.WriteHeader(cmp.Or(answers.writeCode, http.StatusNoContent))
	case strings.HasSuffix(path, "/worklog"):
		asked.Record(writeWorklog, request)
		writer.WriteHeader(cmp.Or(answers.writeCode, http.StatusCreated))
		_, _ = writer.Write([]byte(`{"timeSpent":"2h"}`))
	case strings.HasSuffix(path, "/transitions"):
		writer.WriteHeader(cmp.Or(answers.movesCode, http.StatusOK))
		_, _ = writer.Write([]byte(cmp.Or(answers.transitions, trackerMoves)))
	case strings.HasSuffix(path, "/comment"):
		_, _ = writer.Write([]byte(`{"startAt":2,"total":3,"comments":[]}`))
	case path == "/rest/api/2/issue/"+trackedIssue, path == "/rest/api/2/issue/"+movedIssue:
		writer.WriteHeader(cmp.Or(answers.issueCode, http.StatusOK))
		_, _ = writer.Write([]byte(trackedDetail))
	default:
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"errorMessages":["Issue Does Not Exist"]}`))
	}
}

// trackerConfig is a configuration for the Jira at baseURL, with views.
func trackerConfig(baseURL, views string) string {
	return `{"jira":{"base_url":"` + baseURL + `","token":"t"` + views + `}}`
}

// sprintAndTriage configures two views: a sprint board, scoped to you, and a
// triage list, which names the assignee and so stays as it is.
const sprintAndTriage = `,"views":[{"name":"Sprint","jql":"sprint in openSprints()"},` +
	`{"name":"Triage","jql":"assignee is EMPTY"}]`
