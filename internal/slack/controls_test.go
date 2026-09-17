// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/slack"
)

func TestAWorkspaceNameCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":true,"team":"Ex\u001b[2Jample","user":"work\u009bflow"}`))
	})

	// Act
	identity, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	// Assert
	if strings.ContainsAny(identity.Team+identity.User, "\x1b\xc2\x9b") ||
		!strings.HasPrefix(identity.Team, "Ex") || !strings.HasSuffix(identity.Team, "[2Jample") ||
		!strings.HasPrefix(identity.User, "work") || !strings.HasSuffix(identity.User, "flow") {
		t.Errorf("Team, User = %q, %q; want the controls neutralized and the names around them kept",
			identity.Team, identity.User)
	}
}

func TestSlacksErrorCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error":"invalid_auth\u001b[2J"}`))
	})

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, slack.ErrRejected) || strings.ContainsRune(err.Error(), 0x1b) ||
		!strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("AuthTest returned %q, want Slack's error without the escape", err)
	}
}

// errBrokeOff stands in for a connection dropped while an answer was read.
var errBrokeOff = errors.New("connection reset mid-answer")

// brokenBody fails partway through.
type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, errBrokeOff }

func TestAnAnswerThatBreaksOffIsAnError(t *testing.T) {
	t.Parallel()

	// Arrange
	dropped := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(brokenBody{})}, nil
	}

	client := slack.New(dropped, slack.APIBase, botCredentials())

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("AuthTest returned %v, want the read failure", err)
	}
}
