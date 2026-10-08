// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow slack login`: what it asks, what it refuses before asking
// anything, and what it keeps once Slack, here a fake on this machine
// (fakeSlack), accepts the refresh it makes.

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
)

// slackClientID is the Slack app's client ID the login is given.
const slackClientID = "1234.5678"

// slackUserTokenFile is a configuration set up to post to Slack with a user
// token not yet logged in, keeping its credentials in the file.
const slackUserTokenFile = `{"messaging": {"kind": "slack", "channel": "#dev"}}`

// asking is a prompt that answers lines from lines and secrets from secrets,
// keeping which questions were asked of each.
func asking(lines, secrets []string, askedLine, askedSecret *[]string) cli.Prompt {
	answer := func(answers *[]string, asked *[]string) func(string) (string, error) {
		return func(question string) (string, error) {
			*asked = append(*asked, question)

			if len(*answers) == 0 {
				return "", nil
			}

			next := (*answers)[0]
			*answers = (*answers)[1:]

			return next, nil
		}
	}

	return cli.Prompt{Line: answer(&lines, askedLine), Secret: answer(&secrets, askedSecret)}
}

func TestSlackLoginAsksForTheAppOnScreenAndItsSecretOffIt(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, slackUserTokenFile)

	var askedLine, askedSecret []string

	prompt := asking([]string{slackClientID}, nil, &askedLine, &askedSecret)

	// Act
	_, err := runGuided(t, dir, prompt, "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("slack login = %v, want it to stop at the empty client secret", err)
	}

	if len(askedLine) != 1 || !strings.Contains(askedLine[0], "client ID") ||
		len(askedSecret) != 1 || !strings.Contains(askedSecret[0], "client secret") {
		t.Errorf("asked %q on screen and %q off it, want the client ID on screen and its secret off it",
			askedLine, askedSecret)
	}

	unchanged(t, path, slackUserTokenFile)
}

func TestSlackLoginStopsAtAnEmptyClientID(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, slackUserTokenFile)

	var askedLine, askedSecret []string

	// Act
	_, err := runGuided(t, dir, asking(nil, nil, &askedLine, &askedSecret), "slack", "login")

	// Assert
	if err == nil || len(askedSecret) != 0 {
		t.Errorf("slack login = %v after %d secret questions, want it stopped before any", err, len(askedSecret))
	}

	unchanged(t, path, slackUserTokenFile)
}

func TestSlackLoginRefusesASlackPostingThroughAWebhook(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	contents := `{"messaging": {"kind": "slack", "webhook_url": "https://hooks.slack.example/services/not-real"}}`
	path := writeFile(t, dir, contents)

	// Act
	_, err := run(t, dir, "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "webhook_url") {
		t.Errorf("slack login = %v, want the webhook named as what to remove first", err)
	}

	unchanged(t, path, contents)
}

func TestSlackLoginUnderDryRunAsksAndWritesNothing(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, slackUserTokenFile)

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "slack", "login", "--dry-run")

	// Assert
	if err != nil || !strings.Contains(printed.stderr, "dry run") || printed.stdout != "" {
		t.Errorf("slack login --dry-run = %v, want it to say on stderr alone what it would do:\n%+v", err, printed)
	}

	unchanged(t, path, slackUserTokenFile)
}

func TestSlackLoginNeedsAConfigurationFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	_, err := run(t, dir, "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "config init") {
		t.Errorf("slack login = %v, want it to send the reader to config init first", err)
	}

	_, statErr := os.Stat(filepath.Join(dir, config.FileName))
	if statErr == nil {
		t.Error("slack login wrote a configuration file")
	}
}

func TestSlackLoginKeepsWhatSlackGivesBackAndSaysWhoseItIs(t *testing.T) {
	// Arrange
	fakeSlack(t, map[string]slackAnswer{"/oauth.v2.access": {http.StatusOK, `{"ok":true,"token_type":"user",` +
		`"access_token":"xoxe.xoxp-1-new","refresh_token":"xoxe-1-next","expires_in":43200}`}})

	dir := t.TempDir()
	// The file already keeps the app's secret, so the token is kept there on
	// every system, never in the keychain of the machine the test runs on.
	path := writeFile(t, dir, `{"messaging": {"kind": "slack", "client_id": "`+slackClientID+`",`+
		` "client_secret": "client-secret-old", "channel": "#dev"}}`)

	var askedLine, askedSecret []string

	prompt := asking([]string{slackClientID}, []string{"client-secret-9999", "xoxe-1-first"}, &askedLine, &askedSecret)

	// Act
	printed, err := runStreams(t, dir, prompt, "slack", "login")

	// Assert
	if err != nil || !strings.Contains(printed.stderr, "Logged in to Slack as ana in Acme.") || printed.stdout != "" {
		t.Fatalf("slack login = %v, want it to say on stderr alone whose the token is:\n%+v", err, printed)
	}

	held, err := os.ReadFile(path)
	kept := string(held)

	if err != nil || !strings.Contains(kept, "xoxe.xoxp-1-new") || !strings.Contains(kept, "xoxe-1-next") ||
		strings.Contains(kept, "xoxe-1-first") {
		t.Errorf("%s holds %s (%v), want the pair Slack gave back in place of the spent refresh token", path, kept, err)
	}
}

// unchanged fails the test when the file at path no longer holds contents.
func unchanged(t *testing.T, path, contents string) {
	t.Helper()

	held, err := os.ReadFile(path)
	if err != nil || string(held) != contents {
		t.Errorf("%s now holds %q (%v), want it left as it was", path, held, err)
	}
}

func TestSlackLoginRefusalsExitInTheirFamily(t *testing.T) {
	cases := map[string]struct {
		contents string
		prompt   func(t *testing.T) cli.Prompt
		want     int
	}{
		"no configuration file": {want: 3, prompt: unusedPrompt},
		"slack over a webhook": {
			contents: `{"messaging": {"kind": "slack", "webhook_url": "https://hooks.slack.example/services/not-real"}}`,
			want:     3, prompt: unusedPrompt,
		},
		"another service": {contents: `{"messaging": {"kind": "teams"}}`, want: 3, prompt: unusedPrompt},
		"a blank client ID": {
			contents: slackUserTokenFile, want: 2,
			prompt: func(*testing.T) cli.Prompt { return asking(nil, nil, new([]string), new([]string)) },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			if tt.contents != "" {
				writeFile(t, dir, tt.contents)
			}

			// Act
			_, err := runGuided(t, dir, tt.prompt(t), "slack", "login")

			// Assert
			wantExit(t, err, tt.want)
		})
	}
}

func TestSlackLoginWithNoTerminalSaysToRunItAtOne(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, slackUserTokenFile)

	closed := func(string) (string, error) { return "", io.EOF }

	// Act
	_, err := runGuided(t, dir, cli.Prompt{Line: closed, Secret: closed}, "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "at a terminal") {
		t.Errorf("slack login with stdin closed = %v, want it to say to run it at a terminal", err)
	}

	wantExit(t, err, 2)
}

func TestSlackLoginThatSlackRefusesLeavesTheFileAsItWas(t *testing.T) {
	// Arrange
	fakeSlack(t, map[string]slackAnswer{"/oauth.v2.access": {http.StatusOK, `{"ok":false,"error":"invalid_grant"}`}})

	dir := t.TempDir()
	// A working login's file, the app's secret kept in it, so nothing is read
	// from the keychain of the machine the test runs on.
	contents := `{"messaging": {"kind": "slack", "client_id": "` + slackClientID + `",` +
		` "client_secret": "client-secret-old", "channel": "#dev"}}`
	path := writeFile(t, dir, contents)

	prompt := asking([]string{"9999.0000"}, []string{"client-secret-9999", "xoxe-1-first"}, new([]string), new([]string))

	// Act
	_, err := runGuided(t, dir, prompt, "slack", "login")

	// Assert
	unchanged(t, path, contents)
	wantExit(t, err, 3)
}
