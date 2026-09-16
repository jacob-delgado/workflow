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

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":true,"team":"Ex\u001b[2Jample","user":"work\u009bflow"}`))
	})

	identity, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	if strings.ContainsAny(identity.Team+identity.User, "\x1b\xc2\x9b") {
		t.Errorf("a control survived into %q and %q", identity.Team, identity.User)
	}
}

func TestSlacksErrorCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error":"invalid_auth\u001b[2J"}`))
	})

	_, err := client.AuthTest(t.Context())
	if err == nil || strings.ContainsRune(err.Error(), 0x1b) {
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

	dropped := func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(brokenBody{})}, nil
	}

	_, err := slack.New(dropped, slack.APIBase, botCredentials()).AuthTest(t.Context())
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("AuthTest returned %v, want the read failure", err)
	}
}
