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

	// Arrange
	// Anyone who can edit an issue chooses its summary, and the summary is
	// about to be drawn in someone else's terminal.
	body := `{"total":1,"issues":[{"key":"OPS-1","fields":{"summary":"Fix ` + clearScreen + `login",` +
		`"status":{"name":"In ` + clearScreen + `Progress","statusCategory":{"key":"indeterminate"}}}}]}`

	client := serve(t, answer(body, "fred"))

	// Act
	result, err := client.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if len(result.Issues) != 1 {
		t.Fatalf("got %d issues, want the one in the answer", len(result.Issues))
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

	// Arrange
	body := `{"errorMessages":["no ` + clearScreen + `such field"],"errors":{}}`
	client := serve(t, failWith(http.StatusBadRequest, body, "fred"))

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if !errors.Is(err, jira.ErrRejected) || carriesEscape(err.Error()) || !strings.Contains(err.Error(), "such field") {
		t.Errorf("Search returned %q, want Jira's reason without the escape", err)
	}
}

func TestAUserNameCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	// doctor prints this, and its output is what bug reports ask people to paste.
	body := `{"name":"fred","displayName":"Fred ` + clearScreen + `User","active":true}`
	client := serve(t, answer(body, "fred"))

	// Act
	user, err := client.Myself(t.Context())
	if err != nil {
		t.Fatalf("Myself returned %v, want nil", err)
	}

	// Assert
	if carriesEscape(user.DisplayName) || !strings.HasPrefix(user.DisplayName, "Fred ") ||
		!strings.HasSuffix(user.DisplayName, "[2JUser") {
		t.Errorf("DisplayName = %q, want the escape neutralized and the name around it kept", user.DisplayName)
	}
}

func TestATransitionCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	body := `{"transitions":[{"id":"1","name":"Do ` + clearScreen + `it",` +
		`"to":{"name":"Done","statusCategory":{"key":"done"}}}]}`
	client := serve(t, answer(body, "fred"))

	// Act
	found, err := client.Transitions(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	// Assert
	if len(found) != 1 || carriesEscape(found[0].Name) || !strings.HasSuffix(found[0].Name, "[2Jit") {
		t.Errorf("Transitions = %+v, want one, its name's escape neutralized and the text around it kept", found)
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

	cases := map[string]struct {
		status int
		want   error
	}{
		"an answer is the read failure": {status: http.StatusOK, want: errFixtureTimeout},
		// With no reason to read, the status is still worth reporting.
		"a refusal keeps its status": {status: http.StatusBadRequest, want: jira.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := jira.New(droppedMidAnswer(tt.status), bearerConfig(exampleBaseURL))

			// Act
			_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Search returned %v, want %v", err, tt.want)
			}
		})
	}
}
