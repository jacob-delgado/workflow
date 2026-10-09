// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow doctor --online` asks Slack, here a fake on this machine
// (fakeSlack), whose a Slack user token is.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestDoctorOnlineSaysWhoseAnAcceptedSlackUserTokenIs(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeSlack(t, nil)

	dir := t.TempDir()
	writeFile(t, dir, slackLoggedInConfig())

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	row := credentialRow(output, "slack")
	if !strings.HasPrefix(row, "ana in Acme (user token, kept in the configuration file ") ||
		!strings.HasSuffix(row, config.FileName+", expires in 1h0m0s)") {
		t.Errorf("doctor's slack row = %q, want ana in Acme, where the token is kept and when it expires:\n%s",
			row, output)
	}
}

// credentialRow is what doctor's credential check prints for service.
func credentialRow(output, service string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if row, found := strings.CutPrefix(strings.TrimSpace(line), service+" "); found {
			return strings.TrimSpace(row)
		}
	}

	return ""
}

func TestDoctorOnlineNamesWhereASlackTokenIsKeptWhenItCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file stops parsing once Slack has answered for the token it held,
	// so how long the token has left cannot be read.
	dir := t.TempDir()
	path := writeFile(t, dir, slackLoggedInConfig())
	broken := breakFile(t, path)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		broken()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"team":"Acme","user":"ana","team_id":"` + slackWorkspace + `"}`))
	}))
	t.Cleanup(server.Close)
	setVariable(t, wiring.SlackAPIVariable, server.URL)

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	if row := credentialRow(output, "slack"); row != "ana in Acme (user token, kept in the configuration file "+path+")" {
		t.Errorf("doctor's slack row = %q, want ana in Acme and where the token is kept, without its expiry:\n%s",
			row, output)
	}
}
