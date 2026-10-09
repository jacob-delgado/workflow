// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The messaging service posts to Slack with a rotating user token, which every
// post asks for anew, so a token that ran out between two posts is refreshed
// rather than sent stale. A post these tests make never reaches Slack: its
// token is refused before anything is sent. Each keeps the token's
// credentials in the configuration file, which every system can hold, so no
// test reads the keychain of the machine it runs on.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// slackChannel is the channel a user-token configuration posts to.
const slackChannel = "#dev"

// halfLoggedIn is a configuration file that keeps the user token's credentials
// itself, holding the app's client secret and nothing to post with yet.
func halfLoggedIn(t *testing.T) config.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	contents := `{"messaging": {"kind": "slack", "client_id": "1234.5678", "client_secret": "client-secret-9999",` +
		` "channel": "` + slackChannel + `"}}`

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("loading the configuration: %v", err)
	}

	return cfg
}

func TestAUserTokenNotSetUpIsReportedBeforeAnythingIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	err := seams.Post("", "hi")

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("Post = %v, want no credential, and the login named", err)
	}
}

func TestMessagingSettingsSavedWhileRunningReachTheNextPost(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)
	deps, controls := processEnvironment().Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	controls.UseMessagingSettings(config.Messaging{Kind: config.KindSlack, WebhookURL: "http://hooks.example.com/x"})

	err := deps.Messaging.Post("", "hi")

	// Assert
	if !errors.Is(err, messaging.ErrInsecureWebhook) {
		t.Errorf("Post = %v, want the webhook saved since used, and refused for its address", err)
	}
}

// runOut is a configuration file keeping a Slack user token that has run out,
// so the next ask for it refreshes it first.
func runOut(t *testing.T) config.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	contents := `{"messaging": {"kind": "slack", "client_id": "1234.5678", "client_secret": "client-secret-9999",` +
		` "refresh_token": "xoxe-1-refresh", "access_token": "xoxe.xoxp-1-run-out",` +
		` "expires_at": "` + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `", "channel": "#dev"}}`

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("loading the configuration: %v", err)
	}

	return cfg
}

func TestARefreshSlackDidNotJudgeIsNoTokenItRefused(t *testing.T) {
	// A refresh Slack answers with a wait, or with trouble of its own, says
	// nothing of the token, so the post must not be told to log in again.
	cases := map[string]struct {
		status int
		want   error
	}{
		"asked to wait": {status: http.StatusTooManyRequests, want: httpx.ErrRateLimited},
		"in trouble":    {status: http.StatusBadGateway, want: messaging.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv(wiring.SlackAPIVariable, "")

			answering := func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tt.status, Header: http.Header{"Retry-After": {"30"}},
					Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":"ratelimited"}`)),
				}, nil
			}
			source := processEnvironment().SlackToken(runOut(t), answering)

			// Act
			_, err := source(t.Context(), "")

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, messaging.ErrRejected) {
				t.Errorf("the token = %v, want %v and no refused credential", err, tt.want)
			}
		})
	}
}

// errSlackDown is a transport's failure to reach Slack at all.
var errSlackDown = errors.New("connection refused")

// errNoStateDir is a system that names no directory for the store.
var errNoStateDir = errors.New("no data directory")

// stateIn is the wiring's environment with the store's directory as stateDir
// answers it, reading no variable of the process's own, so Slack is asked at
// its own address, through the transport each test hands it.
func stateIn(stateDir func() (string, error)) wiring.Environment {
	return wiring.Environment{
		Home: "", Getenv: func(string) string { return "" }, StateDir: stateDir, LookPath: proc.LookPath,
	}
}

// stateDirAt is a store directory that is dir.
func stateDirAt(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

// slackDown is a transport that never reaches Slack.
func slackDown(*http.Request) (*http.Response, error) {
	return nil, errSlackDown
}

// rotatingSlack is a Slack that answers every refresh with a new pair.
func rotatingSlack(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"ok":true,"access_token":"xoxe.xoxp-1-new","token_type":"user",` +
			`"expires_in":43200,"refresh_token":"xoxe-1-new"}`)),
	}, nil
}

// slackLockFile is the file a Slack token refresh is made under, in the
// directory it is kept in.
const slackLockFile = "slack-refresh.lock"

func TestASlackNotReachedReadsAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	source := stateIn(stateDirAt(t.TempDir())).SlackToken(runOut(t), slackDown)

	// Act
	_, err := source(t.Context(), "")

	// Assert
	if !errors.Is(err, messaging.ErrUnreachable) || errors.Is(err, messaging.ErrRejected) {
		t.Errorf("the token = %v, want Slack unreachable and no refused credential", err)
	}
}

func TestANewPairThatCouldNotBeKeptAsksForALoginAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file holds the pair to refresh, and a key no configuration has, so
	// the new pair Slack gives cannot be saved into it.
	cfg := runOut(t)

	contents, err := os.ReadFile(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}

	write(t, cfg.Path, strings.TrimSuffix(strings.TrimSpace(string(contents)), "}")+`, "unknown": true}`, config.FileMode)

	source := stateIn(stateDirAt(t.TempDir())).SlackToken(cfg, rotatingSlack)

	// Act
	_, err = source(t.Context(), "")

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || !errors.Is(err, slackauth.ErrNotKept) {
		t.Errorf("the token = %v, want no credential, and the pair not kept named", err)
	}
}

func TestARefreshLockHeldElsewhereKeepsItsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	unlock, err := slackauth.FileLock(filepath.Join(dir, slackLockFile))(t.Context())
	if err != nil {
		t.Fatalf("holding the lock: %v", err)
	}
	defer unlock()

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	source := stateIn(stateDirAt(dir)).SlackToken(runOut(t), rotatingSlack)

	// Act
	_, err = source(ctx, "")

	// Assert
	if !errors.Is(err, slackauth.ErrLocked) || errors.Is(err, messaging.ErrNoCredential) ||
		errors.Is(err, messaging.ErrRejected) {
		t.Errorf("the token = %v, want the lock held elsewhere, in its own words", err)
	}
}

func TestARefreshIsLockedBesideTheConfigurationWhereNoStoreDirectoryIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := runOut(t)
	source := stateIn(func() (string, error) { return "", errNoStateDir }).SlackToken(cfg, slackDown)

	// Act
	_, err := source(t.Context(), "")

	// Assert
	_, statErr := os.Stat(filepath.Join(filepath.Dir(cfg.Path), slackLockFile))
	if !errors.Is(err, messaging.ErrUnreachable) || statErr != nil {
		t.Errorf("the token = %v, lock beside the configuration: %v; want the refresh locked there", err, statErr)
	}
}
