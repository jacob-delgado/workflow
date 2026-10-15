// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

// `workflow doctor --online` asks Slack, here a fake on this machine
// (clitest.FakeSlack), who holds a Slack user token: which user, in which
// workspace.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestDoctorOnlineSaysWhoseAnAcceptedSlackUserTokenIs(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeSlack(t, nil)

	dir := t.TempDir()
	clitest.WriteFile(t, dir, clitest.SlackLoggedInConfig())

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online")

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
	path := clitest.WriteFile(t, dir, clitest.SlackLoggedInConfig())
	broken := clitest.BreakFile(t, path)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		broken()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"team":"Acme","user":"ana","team_id":"` + clitest.SlackWorkspace + `"}`))
	}))
	t.Cleanup(server.Close)
	clitest.SetVariable(t, wiring.SlackAPIVariable, server.URL)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if row := credentialRow(output, "slack"); row != "ana in Acme (user token, kept in the configuration file "+path+")" {
		t.Errorf("doctor's slack row = %q, want ana in Acme and where the token is kept, without its expiry:\n%s",
			row, output)
	}
}

func TestDoctorUnderDryRunHoldsBackARefreshThatWouldWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",`+
		`"refresh_token":"slack-refresh-8888","access_token":"xoxe.xoxp-old","expires_at":"2020-01-01T00:00:00Z",`+
		`"channel":"#dev"}}`)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online", "--dry-run")

	// Assert
	if !strings.Contains(output, "--dry-run") {
		t.Errorf("doctor does not say the refresh was held back for --dry-run:\n%s", output)
	}

	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Errorf("the configuration file changed under --dry-run (%v)", err)
	}
}

func TestDoctorOnlineRefreshesASlackTokenThatIsDue(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only a token due a refresh tells a run without --dry-run from one with
	// it: a token with time left is handed back unchanged either way.
	slack := clitest.FakeSlack(t, map[string]clitest.SlackAnswer{"/oauth.v2.access": {
		Status: http.StatusOK,
		Body: `{"ok":true,"token_type":"user","access_token":"xoxe.xoxp-1-new",` +
			`"refresh_token":"xoxe-1-next","expires_in":43200}`,
	}})

	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",`+
		`"refresh_token":"slack-refresh-8888","access_token":"xoxe.xoxp-old","expires_at":"2020-01-01T00:00:00Z",`+
		`"channel":"#dev"}}`)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if row := credentialRow(output, "slack"); !strings.HasPrefix(row, "ana in Acme (user token, kept in ") {
		t.Errorf("doctor's slack row = %q, want ana in Acme once the token is refreshed:\n%s", row, output)
	}

	if asked := slack.Count("/oauth.v2.access"); asked != 1 {
		t.Errorf("doctor asked Slack for a refresh %d times, want once", asked)
	}

	after, err := os.ReadFile(path)

	kept := string(after)
	if err != nil || !strings.Contains(kept, "xoxe.xoxp-1-new") || !strings.Contains(kept, "xoxe-1-next") {
		t.Errorf("the configuration file does not keep the refreshed tokens (%v)", err)
	}
}

func TestDoctorUnderDryRunCallsASlackTokenNeverLoggedInMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file keeps the app's secret but no refresh token: the login was
	// never finished.
	dir := t.TempDir()
	clitest.WriteFile(
		t, dir, `{"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999","channel":"#dev"}}`,
	)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json", "--online", "--dry-run")

	// Assert
	if status := credentialStatusIn(decodeReport(t, output), messagingService); status != missingStatus {
		t.Errorf("the Slack check under --dry-run = %q, want missing:\n%s", status, output)
	}
}

func TestDoctorUnderDryRunAsksSlackAboutATokenStillGood(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token held has an hour left, so asking Slack about it writes nothing.
	clitest.FakeSlack(t, nil)

	dir := t.TempDir()
	clitest.WriteFile(t, dir, clitest.SlackLoggedInConfig())

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online", "--dry-run")

	// Assert
	if row := credentialRow(output, "slack"); !strings.HasPrefix(row, "ana in Acme (user token, kept in ") {
		t.Errorf("doctor's slack row under --dry-run = %q, want ana in Acme:\n%s", row, output)
	}
}

func TestDoctorUnderDryRunHoldsBackARefreshForATokenNeverHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	// The login kept a refresh token but no access token yet, so the first
	// post would refresh, which writes.
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",`+
		`"refresh_token":"slack-refresh-8888","expires_at":"2999-01-01T00:00:00Z","channel":"#dev"}}`)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json", "--online", "--dry-run")

	// Assert
	if status := credentialStatusIn(decodeReport(t, output), messagingService); status != uncheckedStatus {
		t.Errorf("the Slack check under --dry-run = %q, want unchecked:\n%s", status, output)
	}
}
