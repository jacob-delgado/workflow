// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The tests here hold the context the interface asks each system under: its
// session's, which ends when the program does.

// heldRead is a read held until its context ends: it says when it is entered,
// and how its context ended.
type heldRead struct {
	entered chan struct{}
	ended   chan error
}

// newHeldRead is a held read nothing has entered yet.
func newHeldRead() *heldRead {
	return &heldRead{entered: make(chan struct{}, 1), ended: make(chan error, 1)}
}

// wait is the read's body: it blocks until ctx ends, and answers why.
func (h *heldRead) wait(ctx context.Context) error {
	h.entered <- struct{}{}

	<-ctx.Done()

	h.ended <- ctx.Err()

	return fmt.Errorf("the held read: %w", ctx.Err())
}

// endedWith is how the held read's context ended, or a failure once patience
// runs out.
func (h *heldRead) endedWith(t *testing.T) error {
	t.Helper()

	select {
	case err := <-h.ended:
		return err
	case <-time.After(quitPatience):
		t.Fatal("the held read's context never ended")

		return nil
	}
}

// tasks is Taskwarrior with its pending read held.
func (h *heldRead) tasks() seams.Tasks {
	return seams.Tasks{
		Install: func(context.Context) (taskwarrior.Install, error) {
			return taskwarrior.Install{Version: taskwarrior.MinimumVersion}, nil
		},
		Pending: func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, h.wait(ctx) },
	}
}

// forge is a forge with its review requests' read held.
func (h *heldRead) forge() seams.Forge {
	return seams.Forge{
		ReviewRequests: func(ctx context.Context) ([]forge.ReviewRequest, error) { return nil, h.wait(ctx) },
	}
}

// git is a repository with its branch read held.
func (h *heldRead) git() seams.Git {
	return seams.Git{
		Branch: func(ctx context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, h.wait(ctx) },
	}
}

// hooks is lefthook with its read of the hooks git runs held: a lefthook
// that can write a configuration, so the interface reads them.
func (h *heldRead) hooks() seams.Hooks {
	return seams.Hooks{
		Existing: func(ctx context.Context) ([]hooks.GitHook, bool) {
			_ = h.wait(ctx)

			return nil, false
		},
		Write: func(context.Context, hooks.Generated) error { return nil },
	}
}

// tracker is a tracker with its search held.
func (h *heldRead) tracker() seams.Jira {
	return seams.Jira{
		Search: func(ctx context.Context, _ string, _ int) (jira.SearchResult, error) {
			return jira.SearchResult{}, h.wait(ctx)
		},
	}
}

// runEach runs cmd, and each command of a batch it answers on a goroutine of
// its own, as Bubble Tea does, dropping what they answer.
func runEach(cmd tea.Cmd) {
	if cmd == nil {
		return
	}

	if batch, isBatch := cmd().(tea.BatchMsg); isBatch {
		for _, each := range batch {
			go runEach(each)
		}
	}
}

func TestATaskReadTheInterfaceBeganStopsWhenItsSessionEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Tasks: held.tasks()}).WithContext(session)

	go runEach(model.Init())

	<-held.entered

	// Act
	end()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled with the session", err)
	}
}

func TestATaskReadInFlightStopsWhenTheInterfaceQuits(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	model := tui.New(config.Config{}, nil, tui.Deps{Tasks: held.tasks()})

	go func() { _, _ = tui.Run(t.Context(), model, keys, io.Discard) }()

	<-held.entered

	// Act
	go func() { _, _ = typing.Write([]byte("q")) }()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once the interface quit", err)
	}
}

func TestAnIssueSearchTheInterfaceBeganStopsWhenItsSessionEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Jira: held.tracker()}).WithContext(session)

	go runEach(model.Init())

	<-held.entered

	// Act
	end()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the search's context ended with %v, want it canceled with the session", err)
	}
}

func TestAForgeReadTheInterfaceBeganStopsWhenItsSessionEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Forge: held.forge()}).WithContext(session)

	go runEach(model.Init())

	<-held.entered

	// Act
	end()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled with the session", err)
	}
}

func TestAGitReadTheInterfaceBeganStopsWhenItsSessionEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Git: held.git()}).WithContext(session)

	go runEach(model.Init())

	<-held.entered

	// Act
	end()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled with the session", err)
	}
}

func TestAHooksReadTheInterfaceBeganStopsWhenItsSessionEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldRead()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Hooks: held.hooks()}).WithContext(session)

	go runEach(model.Init())

	<-held.entered

	// Act
	end()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled with the session", err)
	}
}
