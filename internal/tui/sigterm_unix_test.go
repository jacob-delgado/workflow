// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package tui_test

// A SIGTERM ends the interface through the context it runs under, which the
// command line's signal.NotifyContext cancels, and the interface takes no
// signal of its own. The signal is a real one, raised on this process, which
// every run beside it would hear too, so this file's test runs alone, as
// .golangci.yml's paralleltest exclusion for it says.

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errTerminated is the cause a SIGTERM ends the command's context with.
var errTerminated = errors.New("terminated signal received")

// heldClock is the interface's clock, which, once armed, holds whoever reads
// it next until it is released: the interface's update loop, which reads it
// as a key switches panes.
type heldClock struct {
	armed    atomic.Bool
	read     chan struct{}
	reading  sync.Once
	released chan struct{}
	release  func()
}

// newHeldClock is a clock not yet armed, released when the test ends at the
// latest.
func newHeldClock(t *testing.T) *heldClock {
	t.Helper()

	clock := &heldClock{read: make(chan struct{}), released: make(chan struct{})}
	clock.release = sync.OnceFunc(func() { close(clock.released) })
	t.Cleanup(clock.release)

	return clock
}

// now is the time, once the clock is released when it is armed.
func (c *heldClock) now() time.Time {
	if c.armed.Load() {
		c.reading.Do(func() { close(c.read) })
		<-c.released
	}

	return testNow()
}

// raiseSIGTERM raises SIGTERM on this process and returns once every listener
// has been handed it: signal.Stop waits for a delivery in progress to finish.
func raiseSIGTERM(t *testing.T) {
	t.Helper()

	heard := make(chan os.Signal, 1)
	signal.Notify(heard, syscall.SIGTERM)

	err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
	if err != nil {
		t.Fatalf("raising SIGTERM: %v", err)
	}

	<-heard
	signal.Stop(heard)
}

func TestRunReturnsWhenSIGTERMEndsItsContext(t *testing.T) {
	// Arrange
	clock := newHeldClock(t)
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	command, terminate := context.WithCancelCause(t.Context())
	returned := make(chan error, 1)

	go func() {
		_, err := tui.Run(command, tui.New(config.Config{}, nil, tui.Deps{Clock: clock.now}), keys, io.Discard)
		returned <- err
	}()

	// The update loop is held reading the clock as 2 switches panes, so the
	// signal arrives while the interface is busy, as it may at any moment.
	time.Sleep(typingPause)
	clock.armed.Store(true)

	go func() { _, _ = typing.Write([]byte("2")) }()

	select {
	case <-clock.read:
	case <-time.After(quitPatience):
		t.Fatal("the interface never read its clock as 2 switched panes")
	}

	raiseSIGTERM(t)

	// Act
	// The signal ends the command's context, as Execute's signal.NotifyContext
	// does, and the update loop goes on.
	terminate(errTerminated)
	clock.release()

	// Assert
	select {
	case <-returned:
	case <-time.After(quitPatience):
		t.Fatal("Run had not returned once SIGTERM ended its context, want the interface to end with it")
	}
}
