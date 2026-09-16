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

	// doctor prints the name, so a forge — or anything answering as one —
	// must not be able to put an escape sequence in front of the reader.
	identity, err := serveForge(t, answerJSON(`{"login":"octo\u001b[2Jcat"}`)).Whoami(t.Context())
	if err != nil {
		t.Fatalf("Whoami returned %v, want nil", err)
	}

	if strings.ContainsRune(identity.Name(), 0x1b) {
		t.Errorf("an escape survived into %q", identity.Name())
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
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(brokenBody{}),
		}, nil
	}

	_, err := forge.New(dropped, "https://api.example.com", secret).Whoami(t.Context())
	if !errors.Is(err, errBrokeOff) {
		t.Errorf("Whoami returned %v, want the read failure", err)
	}
}
