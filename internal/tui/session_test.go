// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The tests here hold the context the interface asks Taskwarrior under: its
// session's, which ends when the program does.

// pendingHeld is Taskwarrior with its pending read held until the read's
// context ends: it says when the read is entered, and how its context ended.
type pendingHeld struct {
	entered chan struct{}
	ended   chan error
}

// newPendingHeld is a Taskwarrior whose pending read nothing has entered yet.
func newPendingHeld() *pendingHeld {
	return &pendingHeld{entered: make(chan struct{}, 1), ended: make(chan error, 1)}
}

// seams is the held Taskwarrior as the interface asks it.
func (p *pendingHeld) seams() seams.Tasks {
	return seams.Tasks{
		Install: func(context.Context) (taskwarrior.Install, error) {
			return taskwarrior.Install{Version: taskwarrior.MinimumVersion}, nil
		},
		Pending: func(ctx context.Context) (taskwarrior.List, error) {
			p.entered <- struct{}{}

			<-ctx.Done()

			p.ended <- ctx.Err()

			return taskwarrior.List{}, ctx.Err()
		},
	}
}

// endedWith is how the held read's context ended, or a failure once patience
// runs out.
func (p *pendingHeld) endedWith(t *testing.T) error {
	t.Helper()

	select {
	case err := <-p.ended:
		return err
	case <-time.After(quitPatience):
		t.Fatal("the held read's context never ended")

		return nil
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
	held := newPendingHeld()
	session, end := context.WithCancel(t.Context())
	model := tui.New(config.Config{}, nil, tui.Deps{Tasks: held.seams()}).WithContext(session)

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
	held := newPendingHeld()
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	model := tui.New(config.Config{}, nil, tui.Deps{Tasks: held.seams()})

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
