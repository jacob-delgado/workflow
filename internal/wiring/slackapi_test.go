// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// WORKFLOW_SLACK_API points every Slack Web API request at another address —
// a fake under test, or a proxy — and since each of those requests carries the
// Slack user token, it takes only an address that keeps the token off the
// wire in clear text: https://, or http:// to this machine.

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// loggedInToken is the Slack user token loggedIn keeps.
const loggedInToken = "xoxe.xoxp-1-logged-in"

// loggedIn is a configuration file keeping a Slack user token good for an
// hour, so a post sends it without refreshing it first.
func loggedIn(t *testing.T) config.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	contents := `{"messaging": {"kind": "slack", "client_id": "1234.5678", "client_secret": "client-secret-9999",` +
		` "refresh_token": "xoxe-1-refresh", "access_token": "` + loggedInToken + `",` +
		` "expires_at": "` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `", "channel": "#dev"}}`

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

// slackSeen is a fake Slack that accepts every post and keeps the
// Authorization header of each request it is sent.
type slackSeen struct {
	lock    sync.Mutex
	bearers []string
}

// start serves the fake until the test ends, and is its port.
func (s *slackSeen) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		s.lock.Lock()
		s.bearers = append(s.bearers, request.Header.Get("Authorization"))
		s.lock.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("reading the fake's port: %v", err)
	}

	return port
}

// seen is every Authorization header the fake was sent.
func (s *slackSeen) seen() []string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return append([]string(nil), s.bearers...)
}

func TestASlackAddressOnThisMachineIsAskedInsteadOfSlack(t *testing.T) {
	for name, host := range map[string]string{"an IP address": "127.0.0.1", "its name": "localhost"} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var slack slackSeen

			port := slack.start(t)
			t.Setenv(wiring.SlackAPIVariable, "http://"+net.JoinHostPort(host, port))
			seams := wired(t, loggedIn(t), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

			// Act
			err := seams.Post("", "hi")
			// Assert
			if err != nil {
				t.Fatalf("Post = %v, want it accepted by the fake", err)
			}

			if seen := slack.seen(); len(seen) != 1 || seen[0] != "Bearer "+loggedInToken {
				t.Errorf("the fake was sent %q, want the post with the user token", seen)
			}
		})
	}
}

func TestAPostFromARepositoryFindsTheUserTokenTheHomeFileKeeps(t *testing.T) {
	// Arrange
	var slack slackSeen

	t.Setenv(wiring.SlackAPIVariable, "http://"+net.JoinHostPort("127.0.0.1", slack.start(t)))

	home := loggedIn(t).Path
	repo := filepath.Join(t.TempDir(), config.FileName)

	err := os.WriteFile(repo, []byte(`{"jira": {"project": "OSS"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the repository's file: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: home, Repo: repo})
	if err != nil {
		t.Fatalf("loading the layers: %v", err)
	}

	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	err = seams.Post("", "hi")

	// Assert
	if seen := slack.seen(); err != nil || len(seen) != 1 || seen[0] != "Bearer "+loggedInToken {
		t.Errorf("Post = %v, the fake was sent %q; want the post with the home file's user token", err, seen)
	}
}

func TestASlackAddressInClearTextOffThisMachineSendsNothing(t *testing.T) {
	for name, address := range map[string]func(port string) string{
		// 0.0.0.0 reaches this machine's listeners on Linux and macOS, so
		// anything sent would arrive at the fake; it is no loopback address.
		"http to an address that is not loopback": func(port string) string { return "http://0.0.0.0:" + port },
		"http to a name that is not this machine's": func(port string) string {
			return "http://slack.example.invalid:" + port
		},
		"no scheme at all": func(port string) string { return "127.0.0.1:" + port },
		"another scheme":   func(port string) string { return "ftp://127.0.0.1:" + port },
	} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var slack slackSeen

			t.Setenv(wiring.SlackAPIVariable, address(slack.start(t)))
			seams := wired(t, loggedIn(t), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

			// Act
			err := seams.Post("", "hi")

			// Assert
			if !errors.Is(err, wiring.ErrSlackAPIRefused) || !strings.Contains(err.Error(), wiring.SlackAPIVariable) {
				t.Errorf("Post = %v, want the address refused, naming %s", err, wiring.SlackAPIVariable)
			}

			if seen := slack.seen(); len(seen) != 0 {
				t.Errorf("the fake was sent %q, want nothing sent and the token kept", seen)
			}
		})
	}
}

func TestAnHTTPSSlackAddressAnywhereIsAsked(t *testing.T) {
	// Arrange
	// The fake's certificate is one no system trusts, so the handshake fails
	// and nothing is sent; that a connection arrives is what shows the
	// address was taken.
	var connections atomic.Int32

	server := httptest.NewUnstartedServer(http.NotFoundHandler())
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	t.Setenv(wiring.SlackAPIVariable, server.URL)
	seams := wired(t, loggedIn(t), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	err := seams.Post("", "hi")

	// Assert
	if errors.Is(err, wiring.ErrSlackAPIRefused) || connections.Load() == 0 {
		t.Errorf("Post = %v after %d connections, want the https address asked", err, connections.Load())
	}
}
