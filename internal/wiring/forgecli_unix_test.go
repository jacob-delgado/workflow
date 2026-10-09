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
)

// stopWait is how long a test waits for gh to be stopped before failing: a
// failsafe, never the pace of the test.
const stopWait = 10 * time.Second

// installSilentGH puts on PATH a gh that writes to begun once it runs, then
// never answers, and returns that named pipe.
func installSilentGH(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	begun := filepath.Join(dir, "begun")

	err := syscall.Mkfifo(begun, 0o600)
	if err != nil {
		t.Fatalf("making the pipe gh says it has begun through: %v", err)
	}

	write(t, filepath.Join(dir, "gh"), "#!/bin/sh\nprintf x > '"+begun+"'\nexec sleep 600\n", 0o755)
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

	access, err := process.ReachForge(t.Context(), cfg.Forge, githubOwnerRepo(), githubAPI, http.DefaultClient.Do)
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
