// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow slack login` refreshes and keeps under the refresh lock every
// workflow on the machine takes, so a post refreshing the pair it replaces
// never keeps its own over the login's.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// heldRefreshLock holds the Slack refresh lock in the store directory a run
// at where keeps, as another workflow refreshing would, until the test ends.
func heldRefreshLock(t *testing.T, where clitest.Place) {
	t.Helper()

	dir, err := clitest.EnvironmentFor(t, where).Process.StateDir()
	if err != nil {
		t.Fatalf("finding the store directory: %v", err)
	}

	unlock, err := slackauth.FileLock(filepath.Join(dir, "slack-refresh.lock"))(t.Context())
	if err != nil {
		t.Fatalf("holding the lock: %v", err)
	}

	t.Cleanup(unlock)
}

func TestSlackLoginWaitsForARefreshAnotherWorkflowIsMaking(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeSlack(t, map[string]clitest.SlackAnswer{slackRefresh: {Status: http.StatusOK, Body: slackRenewed}})

	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}
	// The file already keeps the app's secret, so the token is kept there on
	// every system, never in the keychain of the machine the test runs on.
	contents := `{"messaging": {"kind": "slack", "client_id": "` + clitest.SlackClientID + `",` +
		` "client_secret": "client-secret-old", "channel": "#dev"}}`
	path := clitest.WriteFile(t, where.Dir, contents)
	heldRefreshLock(t, where)

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	root := cli.NewRootCmdOver(clitest.Scripted([]string{clitest.SlackClientID}, typedLogin()), nil, nil,
		clitest.EnvironmentFor(t, where))
	root.SetArgs([]string{"slack", "login"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	// Act
	err := root.ExecuteContext(ctx)

	// Assert
	held, readErr := os.ReadFile(path)
	if !errors.Is(err, slackauth.ErrLocked) || readErr != nil || string(held) != contents {
		t.Errorf("slack login = %v, and %s holds %q (%v); want it to wait on the lock and keep nothing "+
			"while another workflow refreshes", err, path, held, readErr)
	}
}
