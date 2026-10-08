// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// setupToken is the Jira token these tests type into a setup.
const setupToken = "setup-typed-token"

// acceptingJira is a local Jira that knows any token as Fred.
func acceptingJira(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"displayName":"Fred F. User","name":"fred"}`))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func TestSetupChecksTheTokenWithoutLoggingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	var logged bytes.Buffer

	where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}
	deps := wiring.SetupDeps(t.Context(), where, wiring.NewRequestLog(&logged, nil), nil)

	// Act
	who, err := deps.Check(config.Jira{BaseURL: acceptingJira(t), Token: setupToken})

	// Assert
	if err != nil || who != "Fred F. User (fred)" {
		t.Fatalf("Check = %q, %v; want Fred", who, err)
	}

	if !strings.Contains(logged.String(), "/rest/api/2/myself") || strings.Contains(logged.String(), setupToken) {
		t.Errorf("the request log = %q, want the check outlined without the token", logged.String())
	}
}

func TestSetupWritesTheFileWithTheTokenInTheKeychainWhenChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	var kept string

	where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}
	deps := wiring.SetupDeps(t.Context(), where, nil, func(_, secret string) error {
		kept = secret

		return nil
	})
	request := setup.Request{
		Place:    setup.Home,
		Answers:  setup.Answers{Jira: config.Jira{BaseURL: acceptingJira(t), Token: setupToken}},
		Keychain: true,
	}

	// Act
	written, err := deps.Write(request)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	contents, err := os.ReadFile(written.Path)
	if err != nil || strings.Contains(string(contents), setupToken) || kept != setupToken {
		t.Errorf("wrote %q (%v), keychain %q; want the token in the keychain alone", contents, err, kept)
	}

	if offer := deps.Offer(); !offer.Places[1].Keychain || offer.Places[1].Path != written.Path {
		t.Errorf("Offer = %+v, want the home file %s, the keychain offered for it", offer, written.Path)
	}
}
