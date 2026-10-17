// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// A Slack request that goes on when its asker leaves (the directory's read,
// which others may share, and a token's refresh, which must not stop between
// Slack's answer and keeping it) still ends with the command: SIGINT and
// SIGTERM cancel what every command runs.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// patientTiming lets each request run two minutes, so a request the command's
// end did not stop is still running when the test gives up on it.
const patientTiming = `"timing":{"request_timeout":"2m"}`

// slackExpiredConfig is slackLoggedInConfig with its access token run out, so
// the first Slack request refreshes it.
func slackExpiredConfig() string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` + patientTiming + `,` +
		`"messaging":{"client_id":"1234.5678","client_secret":"client-secret-9999",` +
		`"refresh_token":"xoxe-1-refresh","access_token":"xoxe.xoxp-1-held",` +
		`"expires_at":"` + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `","channel":"#dev"}}`
}

// slackPatientConfig is slackLoggedInConfig with patientTiming.
func slackPatientConfig() string {
	return strings.Replace(slackLoggedInConfig(), `{`, `{`+patientTiming+`,`, 1)
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
			_, _ = writer.Write([]byte(`{"ok":true,"team_id":"` + slackWorkspace + `"}`))

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

func TestACtrlCStopsASlackRequestThatOutlastsItsAsker(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		held   string
		config func() string
	}{
		"the directory's read of the workspace": {held: slackAuthTest, config: slackPatientConfig},
		"the refresh of a token run out":        {held: slackRefresh, config: slackExpiredConfig},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fakeGh(t, ghResponses{pulls: openPull("Add login")})

			command, interrupt := context.WithCancel(t.Context())
			defer interrupt()

			slack := interruptingSlack(t, testCase.held, interrupt)
			repo := ownedRepo(t)
			clitest.WriteFile(t, repo, testCase.config())
			root := cli.NewRootCmdOver(
				clitest.UnusedPrompt(t), nil, nil, clitest.EnvironmentFor(t, clitest.Place{Dir: repo, Home: t.TempDir()}),
			)
			root.SetArgs([]string{"announce", "--yes"})
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)

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
				t.Errorf("Slack's %s ran on %v after the command was interrupted, want it stopped",
					testCase.held, signalPatience)
			}
		})
	}
}
