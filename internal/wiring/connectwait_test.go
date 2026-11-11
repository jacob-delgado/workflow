// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// A client found on first use is found once, however many ask for it at a
// time, so a second asker waits on the first's connect. Each waits only as
// long as its own context lasts: a page that left, or a command interrupted,
// stops waiting at once, rather than when a token command another asker began
// gives up.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// heldProgram is a stand-in program that, run, marks that it was asked and
// then waits until the test releases it, then fails, so nothing is found and
// nothing goes on to reach a service.
type heldProgram struct {
	path    string
	asked   string
	release string
}

// newHeldProgram writes a held stand-in named name.
func newHeldProgram(t *testing.T, name string) heldProgram {
	t.Helper()

	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatalf("these tests need sleep: %v", err)
	}

	dir := t.TempDir()
	held := heldProgram{
		path: filepath.Join(dir, name), asked: filepath.Join(dir, "asked"), release: filepath.Join(dir, "release"),
	}
	write(t, held.path, "#!/bin/sh\n: >'"+held.asked+"'\n"+
		"while [ ! -f '"+held.release+"' ]; do '"+sleep+"' 0.05; done\nexit 1\n", 0o700)
	t.Cleanup(held.let)

	return held
}

// waitAsked waits until the stand-in has been run.
func (h heldProgram) waitAsked(t *testing.T) {
	t.Helper()

	for deadline := time.Now().Add(connectPatience); time.Now().Before(deadline); {
		_, err := os.Stat(h.asked)
		if err == nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("%s was never run", h.path)
}

// let lets the stand-in end.
func (h heldProgram) let() {
	_ = os.WriteFile(h.release, nil, 0o600)
}

// connectPatience is how long a test waits on an asker that should answer at
// once, before calling it stuck.
const connectPatience = 10 * time.Second

// leftAsker is a context already ended, as a page that left has.
func leftAsker(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return ctx
}

// answeredInTime is what ask answered, or a failure when it did not answer
// within connectPatience, while another asker's connect still held.
func answeredInTime(t *testing.T, ask func() error) error {
	t.Helper()

	answered := make(chan error, 1)

	go func() { answered <- ask() }()

	select {
	case err := <-answered:
		return err
	case <-time.After(connectPatience):
		t.Fatalf("a call whose context had ended still waited, after %s, on a connect another call began",
			connectPatience)

		return nil
	}
}

// begun runs ask on its own, as the first asker, and returns a channel closed
// once it has answered.
func begun(ask func()) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ask()
	}()

	return done
}

func TestAForgeCallStopsWaitingWhenItsContextEnds(t *testing.T) {
	// Arrange
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	tokenLookup := newHeldProgram(t, "gh")
	t.Setenv("PATH", filepath.Dir(tokenLookup.path)+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := config.Config{Forge: config.Forge{Kind: githubKind, Host: hostGitHub}}
	forgeSide := wired(t, cfg, onOrigin(t.TempDir(), remoteGitHub), nil).Forge

	first := begun(func() { _, _ = forgeSide.ReviewRequests(t.Context()) })

	tokenLookup.waitAsked(t)

	// Act
	err := answeredInTime(t, func() error {
		_, err := forgeSide.ReviewRequests(leftAsker(t))

		return err
	})

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ReviewRequests = %v, want its own context's end while another's token lookup runs", err)
	}

	tokenLookup.let()
	<-first
}

func TestATaskwarriorCallStopsWaitingWhenItsContextEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	task := newHeldProgram(t, "task")
	tasks := wired(t, config.Config{Taskwarrior: config.Taskwarrior{Program: task.path}},
		onOrigin(t.TempDir(), ""), nil).Tasks

	first := begun(func() { _, _ = tasks.Pending(t.Context()) })

	task.waitAsked(t)

	// Act
	err := answeredInTime(t, func() error {
		_, err := tasks.Pending(leftAsker(t))

		return err
	})

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Pending = %v, want its own context's end while another finds Taskwarrior", err)
	}

	task.let()
	<-first
}

func TestAJiraCallStopsWaitingWhenItsContextEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	tokenCommand := newHeldProgram(t, "token")
	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: "https://jira.invalid", TokenCommand: tokenCommand.path}
	tracker := wired(t, cfg, onOrigin(t.TempDir(), ""), nil).Jira

	first := begun(func() { _, _ = tracker.Search(t.Context(), "assignee = currentUser()", 0) })

	tokenCommand.waitAsked(t)

	// Act
	err := answeredInTime(t, func() error {
		_, err := tracker.Search(leftAsker(t), "assignee = currentUser()", 0)

		return err
	})

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Search = %v, want its own context's end while another's token command runs", err)
	}

	tokenCommand.let()
	<-first
}

// A call whose context has ended, finding no other call connecting, is not
// told it was waiting on one: whatever it answers, it says why truly.
func TestACallWhoseContextEndedIsNotToldItWaitedWhenNoneWasConnecting(t *testing.T) {
	t.Parallel()

	// Arrange
	task := newHeldProgram(t, "task")
	task.let()

	tasks := wired(t, config.Config{Taskwarrior: config.Taskwarrior{Program: task.path}},
		onOrigin(t.TempDir(), ""), nil).Tasks

	// Act
	answers := make([]error, 0, unheldAsks)
	for range unheldAsks {
		_, err := tasks.Pending(leftAsker(t))
		answers = append(answers, err)
	}

	// Assert
	for _, err := range answers {
		if err != nil && strings.Contains(err.Error(), "another call") {
			t.Fatalf("Pending = %v, with no other call connecting", err)
		}
	}
}

// unheldAsks is how many times a call asks with no other connecting: the
// turn free and its context ended both ready, a choice made at random would
// wrongly say it waited at least once in so many all but certainly.
const unheldAsks = 40
