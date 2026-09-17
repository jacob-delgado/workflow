// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestAnAccountNameCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	// doctor prints the name, so a forge — or anything answering as one —
	// must not be able to put an escape sequence in front of the reader.
	client := serveForge(t, answerJSON(`{"login":"octo\u001b[2Jcat"}`))

	// Act
	identity, err := client.Whoami(t.Context())
	if err != nil {
		t.Fatalf("Whoami returned %v, want nil", err)
	}

	// Assert
	name := identity.Name()
	if strings.ContainsRune(name, 0x1b) || !strings.HasPrefix(name, "octo") || !strings.HasSuffix(name, "[2Jcat") {
		t.Errorf("Name() = %q, want the escape neutralized and the name around it kept", name)
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
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(brokenBody{}),
		}, nil
	}

	client := forge.New(dropped, "https://api.example.com", secret)

	// Act
	_, err := client.Whoami(t.Context())

	// Assert
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("Whoami returned %v, want the read failure", err)
	}
}
