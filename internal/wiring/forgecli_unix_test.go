// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wiring_test

// A forge request through gh or glab runs under the request's own context, so
// a request that ends stops the program carrying it. The stand-in gh says it
// has begun through a named pipe, which only Unix makes, and then never
// answers.

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// stopWait is how long a test waits for gh to be stopped before failing: a
// failsafe, never the pace of the test.
const stopWait = 10 * time.Second

// installSilentGH puts on PATH a gh that writes to begun once it runs, then
// never answers, and returns that named pipe.
func installSilentGH(t *testing.T) string {
	t.Helper()

	return installGHSilentAfter(t, "")
}

// installGHSilentAfter puts on PATH a gh that answers its first run with
// reply, as the forge's HTTP response, unless reply is empty, and every run
// after that writes to begun, then never answers; it returns that named pipe.
// The reply holds no single quote.
func installGHSilentAfter(t *testing.T, reply string) string {
	t.Helper()

	dir := t.TempDir()
	begun, answered := filepath.Join(dir, "begun"), filepath.Join(dir, "answered")

	err := syscall.Mkfifo(begun, 0o600)
	if err != nil {
		t.Fatalf("making the pipe gh says it has begun through: %v", err)
	}

	answerOnce := ""
	if reply != "" {
		answerOnce = "if [ ! -e '" + answered + "' ]; then\n" +
			"  : > '" + answered + "'\n" +
			"  printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n%s' '" + reply + "'\n" +
			"  exit 0\n" +
			"fi\n"
	}

	write(t, filepath.Join(dir, "gh"), "#!/bin/sh\n"+answerOnce+"printf x > '"+begun+"'\nexec sleep 600\n", 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return begun
}

// awaitBegun returns once gh has written to the pipe begun, or fails the test.
func awaitBegun(t *testing.T, begun string) {
	t.Helper()

	opened := make(chan error, 1)

	go func() {
		_, err := os.ReadFile(begun)
		opened <- err
	}()

	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("reading that gh has begun: %v", err)
		}
	case <-time.After(stopWait):
		t.Fatal("gh never began")
	}
}

func TestAForgeCLIRequestThatEndsStopsTheProgramCarryingIt(t *testing.T) {
	// Arrange
	// The connection's context stays live throughout: only the request's ends.
	begun := installSilentGH(t)
	cfg, _ := githubCLIWorkspace(t)
	process := processEnvironment()

	access, err := process.ReachForge(t.Context(), cfg.Forge, githubOwnerRepo(), githubAPI, overHTTP())
	if err != nil {
		t.Fatalf("ReachForge through gh: %v", err)
	}

	asking, leave := context.WithCancel(t.Context())
	t.Cleanup(leave)

	request, err := http.NewRequestWithContext(asking, http.MethodGet, branchPulls, nil)
	if err != nil {
		t.Fatal(err)
	}

	answered := make(chan error, 1)

	go func() {
		response, err := access.Doer(request)
		if response != nil {
			_ = response.Body.Close()
		}

		answered <- err
	}()

	awaitBegun(t, begun)

	// Act
	leave()

	// Assert
	select {
	case err := <-answered:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Doer = %v, want the request's own cancellation", err)
		}
	case <-time.After(stopWait):
		t.Fatal("gh ran on after its request ended")
	}
}

// timing.request_timeout bounds every request the interface and the web
// server make, one gh carries as well: a gh that never answers must not leave
// a pane loading for as long as its asker stays.
func TestAForgeCLIRequestIsBoundedByTheRequestTimeout(t *testing.T) {
	// Arrange
	begun := installSilentGH(t)
	cfg, where := githubCLIWorkspace(t)
	cfg.Timing.RequestTimeout = "200ms"
	forgeSeams := wired(t, cfg, where, nil).Forge
	answered := make(chan error, 1)

	// Act
	go func() {
		_, _, err := forgeSeams.FindPullRequest(t.Context(), "feat/x")
		answered <- err
	}()

	// Assert
	awaitBegun(t, begun)

	select {
	case err := <-answered:
		if !errors.Is(err, proc.ErrTimedOut) {
			t.Errorf("FindPullRequest = %v, want gh stopped at the request timeout, %v", err, proc.ErrTimedOut)
		}
	case <-time.After(stopWait):
		t.Fatal("gh ran on past the request timeout")
	}
}
