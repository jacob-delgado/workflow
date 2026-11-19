// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// A Slack request that goes on when its asker leaves, the directory's read,
// which others may share, still ends with the command: SIGINT and SIGTERM
// cancel what every command runs. A token's refresh does not: it must not
// stop between Slack's answer and keeping the pair it gives, so the command
// waits for it, bounded by timing.request_timeout, and then ends.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// patientTiming lets each request run two minutes, so a request the command's
// end did not stop is still running when the test gives up on it.
const patientTiming = `"timing":{"request_timeout":"2m"}`

// slackExpiredConfig is clitest.SlackLoggedInConfig with its access token run out, so
// the first Slack request refreshes it.
func slackExpiredConfig() string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` + patientTiming + `,` +
		`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",` +
		`"refresh_token":"xoxe-1-refresh","access_token":"xoxe.xoxp-1-held",` +
		`"expires_at":"` + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `","channel":"#dev"}}`
}

// slackPatientConfig is clitest.SlackLoggedInConfig with patientTiming.
func slackPatientConfig() string {
	return strings.Replace(clitest.SlackLoggedInConfig(), `{`, `{`+patientTiming+`,`, 1)
}

// heldRequest is a request a Slack on this machine holds: asked closes once
// it first arrives, and ended once one has ended.
type heldRequest struct {
	asked, ended        chan struct{}
	arriving, finishing sync.Once
}

// interruptingSlack is a Slack on this machine that, asked on path, calls
// interrupt, as a Ctrl+C there would, and holds that request until it ends.
// Asked anything else, it answers that all is well.
func interruptingSlack(t *testing.T, path string, interrupt context.CancelFunc) *heldRequest {
	t.Helper()

	held := &heldRequest{asked: make(chan struct{}), ended: make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != path {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"team_id":"` + clitest.SlackWorkspace + `"}`))

			return
		}

		held.arriving.Do(func() { close(held.asked) })
		interrupt()

		// The server notices a request's end only once its body is read.
		_, _ = io.Copy(io.Discard, request.Body)

		select {
		case <-request.Context().Done():
			held.finishing.Do(func() { close(held.ended) })
		case <-time.After(2 * signalPatience):
		}
	}))
	t.Cleanup(server.Close)
	clitest.SetVariable(t, wiring.SlackAPIVariable, server.URL)

	return held
}

// awaitAsked waits for Slack to be asked the request it holds, failing the
// test once patience runs out.
func (h *heldRequest) awaitAsked(t *testing.T) {
	t.Helper()

	select {
	case <-h.asked:
	case <-time.After(signalPatience):
		t.Fatal("Slack was never asked the request it holds")
	}
}

// interruptingThenAnsweringSlack is interruptingSlack, but for the request it
// holds: once a command that let it go would have ended it, it answers with a
// pair Slack gives on a refresh, as a Slack that took the refresh token before
// the Ctrl+C would.
func interruptingThenAnsweringSlack(t *testing.T, path string, interrupt context.CancelFunc) *heldRequest {
	t.Helper()

	held := &heldRequest{asked: make(chan struct{}), ended: make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if request.URL.Path != path {
			_, _ = writer.Write([]byte(`{"ok":true,"team_id":"` + clitest.SlackWorkspace + `"}`))

			return
		}

		held.arriving.Do(func() { close(held.asked) })
		interrupt()

		// The server notices a request's end only once its body is read.
		_, _ = io.Copy(io.Discard, request.Body)

		select {
		case <-request.Context().Done():
			held.finishing.Do(func() { close(held.ended) })

			return
		case <-time.After(time.Second):
		}

		_, _ = writer.Write([]byte(slackRenewed))
	}))
	t.Cleanup(server.Close)
	clitest.SetVariable(t, wiring.SlackAPIVariable, server.URL)

	return held
}

// announceOver is `announce --yes` over a repository of its own with an open
// pull request, whose configuration file holds config, and that file's path.
func announceOver(t *testing.T, config string) (*cobra.Command, string) {
	t.Helper()

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})

	repo := clitest.OwnedRepo(t)
	path := clitest.WriteFile(t, repo, config)
	root := cli.NewRootCmdOver(
		clitest.UnusedPrompt(t), nil, nil, clitest.EnvironmentFor(t, clitest.Place{Dir: repo, Home: t.TempDir()}),
	)
	root.SetArgs([]string{"announce", "--yes"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	return root, path
}

func TestACtrlCStopsTheDirectoryReadOfTheWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	command, interrupt := context.WithCancel(t.Context())
	defer interrupt()

	slack := interruptingSlack(t, clitest.SlackAuthTest, interrupt)
	root, _ := announceOver(t, slackPatientConfig())

	returned := make(chan struct{})

	t.Cleanup(func() { <-returned })

	// Act
	go func() {
		defer close(returned)

		_ = root.ExecuteContext(command)
	}()

	// Assert
	slack.awaitAsked(t)

	select {
	case <-slack.ended:
	case <-time.After(signalPatience):
		t.Errorf("Slack's directory read ran on %v after the command was interrupted, want it stopped", signalPatience)
	}
}

func TestACtrlCDuringARenewalStillKeepsThePair(t *testing.T) {
	t.Parallel()

	// Arrange
	command, interrupt := context.WithCancel(t.Context())
	defer interrupt()

	slack := interruptingThenAnsweringSlack(t, slackRefresh, interrupt)
	root, path := announceOver(t, slackExpiredConfig())

	var err error

	returned := make(chan struct{})

	t.Cleanup(func() { <-returned })

	// Act
	go func() {
		defer close(returned)

		err = root.ExecuteContext(command)
	}()

	// Assert
	slack.awaitAsked(t)

	select {
	case <-returned:
	case <-time.After(signalPatience):
		t.Fatalf("announce ran on %v after it was interrupted, want it ended once the pair was kept", signalPatience)
	}

	wantExit(t, err, 130)
	holdsTheNewPair(t, path)
}

// holdsTheNewPair fails the test when the file at path does not hold the pair
// slackRenewed gives.
func holdsTheNewPair(t *testing.T, path string) {
	t.Helper()

	held, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(held), "xoxe.xoxp-1-new") || !strings.Contains(string(held), "xoxe-1-next") {
		t.Errorf("%s holds %q (%v), want the pair Slack gave back kept", path, held, err)
	}
}

func TestACtrlCStopsASlackLoginsRefresh(t *testing.T) {
	t.Parallel()

	// Arrange
	// A login's refresh and keep end with the command, as every renewal's
	// do: a Ctrl+C once Slack has taken the refresh token typed can lose the
	// pair it gives, rather than leave a hung Slack holding the Ctrl+C.
	command, interrupt := context.WithCancel(t.Context())
	defer interrupt()

	slack := interruptingSlack(t, slackRefresh, interrupt)
	dir := t.TempDir()
	contents := `{` + patientTiming + `,"messaging": {"kind": "slack", "client_id": "` + clitest.SlackClientID +
		`", "client_secret": "client-secret-old", "channel": "#dev"}}`
	path := clitest.WriteFile(t, dir, contents)
	env := clitest.EnvironmentFor(t, clitest.Place{Dir: dir, Home: t.TempDir()})
	root := cli.NewRootCmdOver(loginTyped(), nil, nil, env)
	root.SetArgs([]string{"slack", "login"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	var err error

	returned := make(chan struct{})

	t.Cleanup(func() { <-returned })

	// Act
	go func() {
		defer close(returned)

		err = root.ExecuteContext(command)
	}()

	// Assert
	slack.awaitAsked(t)

	select {
	case <-returned:
	case <-time.After(signalPatience):
		t.Fatalf("slack login ran on %v after it was interrupted, want it stopped", signalPatience)
	}

	wantExit(t, err, 130)
	unchanged(t, path, contents)
}
