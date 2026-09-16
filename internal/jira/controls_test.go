// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// clearScreen is an escape sequence as it travels in JSON: ESC, then the rest
// of a "clear the screen" command.
const clearScreen = `\u001b[2J`

// carriesEscape reports whether text still holds an ESC a terminal would obey.
func carriesEscape(text string) bool {
	return strings.ContainsRune(text, 0x1b)
}

func TestAnIssueCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Anyone who can edit an issue chooses its summary, and the summary is
	// about to be drawn in someone else's terminal.
	body := `{"total":1,"issues":[{"key":"OPS-1","fields":{"summary":"Fix ` + clearScreen + `login",` +
		`"status":{"name":"In ` + clearScreen + `Progress","statusCategory":{"key":"indeterminate"}}}}]}`

	result, err := serve(t, answer(body, "fred")).Search(t.Context(), jira.AssignedToMe)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	found := result.Issues[0]
	if carriesEscape(found.Summary) || carriesEscape(found.Status) {
		t.Errorf("an escape survived into %+v", found)
	}

	if !strings.Contains(found.Summary, "[2J") {
		t.Errorf("neutralizing the escape dropped the text around it: %q", found.Summary)
	}
}

func TestAReasonCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	body := `{"errorMessages":["no ` + clearScreen + `such field"],"errors":{}}`

	_, err := serve(t, failWith(http.StatusBadRequest, body, "fred")).Search(t.Context(), jira.AssignedToMe)
	if err == nil || carriesEscape(err.Error()) {
		t.Errorf("Search returned %q, want Jira's reason without the escape", err)
	}
}

func TestAUserNameCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// doctor prints this, and its output is what bug reports ask people to paste.
	body := `{"name":"fred","displayName":"Fred ` + clearScreen + `User","active":true}`

	user, err := serve(t, answer(body, "fred")).Myself(t.Context())
	if err != nil {
		t.Fatalf("Myself returned %v, want nil", err)
	}

	if carriesEscape(user.DisplayName) {
		t.Errorf("an escape survived into %q", user.DisplayName)
	}
}

func TestATransitionCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	body := `{"transitions":[{"id":"1","name":"Do ` + clearScreen + `it",` +
		`"to":{"name":"Done","statusCategory":{"key":"done"}}}]}`

	found, err := serve(t, answer(body, "fred")).Transitions(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	if carriesEscape(found[0].Name) {
		t.Errorf("an escape survived into %q", found[0].Name)
	}
}

// brokenBody fails partway through, as a connection dropped mid-answer does.
type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, errFixtureTimeout }

// droppedMidAnswer is a transport whose answer breaks off while being read.
func droppedMidAnswer(status int) jira.Doer {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {jsonMediaType}},
			Body:       io.NopCloser(brokenBody{}),
		}, nil
	}
}

func TestAnAnswerThatBreaksOffIsAnError(t *testing.T) {
	t.Parallel()

	_, err := jira.New(droppedMidAnswer(http.StatusOK), bearerConfig(exampleBaseURL)).
		Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, errFixtureTimeout) {
		t.Errorf("Search returned %v, want the read failure", err)
	}
}

func TestARefusalThatBreaksOffKeepsItsStatus(t *testing.T) {
	t.Parallel()

	// With no reason to read, the status is still worth reporting.
	_, err := jira.New(droppedMidAnswer(http.StatusBadRequest), bearerConfig(exampleBaseURL)).
		Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrUnexpectedStatus) {
		t.Errorf("Search returned %v, want ErrUnexpectedStatus", err)
	}
}
