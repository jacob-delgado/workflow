// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// jsonServer answers every request with body and application/json, so a client
// that insists on JSON — the forge does — is satisfied.
func jsonServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, body)
	}))
	t.Cleanup(server.Close)

	return server
}

func TestAskForgeReportsTheAuthenticatedUser(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jsonServer(t, `{"login": "octocat"}`)

	var out bytes.Buffer

	// Act
	// The Doer points at the fake forge — the seam this injection exists for; the
	// success arm never ran outside the forge package before.
	err := askForge(t.Context(), &out, server.Client().Do, server.URL, forge.Token("t"), forge.SourceEnvironment)

	// Assert
	if err != nil || !strings.Contains(out.String(), "octocat") {
		t.Errorf("askForge = %v, out %q; want it to report the authenticated user", err, out.String())
	}
}

func TestCheckJiraReportsWhoTheTokenAuthenticatesAs(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jsonServer(t, `{"name": "jdoe", "displayName": "J Doe"}`)
	settings := config.Jira{BaseURL: server.URL, Token: config.Secret("t")}

	var out bytes.Buffer

	// Act
	err := checkJira(t.Context(), &out, server.Client().Do, settings)

	// Assert
	if err != nil || !strings.Contains(out.String(), "J Doe") {
		t.Errorf("checkJira = %v, out %q; want it to report the authenticated user", err, out.String())
	}
}

func TestCheckSlackReportsTheWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jsonServer(t, `{"ok": true, "user": "botuser", "team": "acme"}`)
	creds := config.Slack{Token: config.Secret("xoxb-token")}

	var out bytes.Buffer

	// Act
	err := checkSlack(t.Context(), &out, server.Client().Do, server.URL, creds)

	// Assert
	if err != nil || !strings.Contains(out.String(), "acme") {
		t.Errorf("checkSlack = %v, out %q; want it to report the workspace", err, out.String())
	}
}
