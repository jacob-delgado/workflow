// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The context the root command hands the interface is the lifetime of the
// program and of every write it sends (tui.Run ends those writes with it): a
// signal ends it while an interface is open, but Execute's own end, once the
// last interface has quit, does not, or a quit would cut a write after all.

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errSignaled is a signal ending the command's context: signal.NotifyContext
// cancels it with a cause naming the signal.
var errSignaled = errors.New("interrupt signal received")

// signalPatience is how long a context a signal ended has to say so.
const signalPatience = 10 * time.Second

// endingPatience is how long a context that must go on is watched for an
// end: one canceled through context.AfterFunc ends in a goroutine of its
// own, so a read right after the cancel can come too early to see it.
const endingPatience = 250 * time.Millisecond

// interfaceRoot is the root command where the test chose, over run, its input
// empty and what it says discarded, to execute under a context the test makes.
func interfaceRoot(t *testing.T, where place, run cli.RunInterface) *cobra.Command {
	t.Helper()

	root := cli.NewRootCmdOver(unusedPrompt(t), run, nil, environmentFor(t, where))
	root.SetArgs([]string{})
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	return root
}

// executing runs root, under a context the function it returns signals,
// until it returns, which the test waits for before its directories go.
func executing(t *testing.T, root *cobra.Command) context.CancelCauseFunc {
	t.Helper()

	command, signaled := context.WithCancelCause(t.Context())
	ran := make(chan struct{})

	t.Cleanup(func() { <-ran })

	go func() {
		defer close(ran)

		_ = root.ExecuteContext(command)
	}()

	return signaled
}

// untilEnded blocks until ctx ends, or the test does: a fake interface whose
// context an assertion found going on returns once that failure ends the
// test, rather than holding executing's cleanup until go test times out.
func untilEnded(ctx context.Context, t *testing.T) {
	t.Helper()

	select {
	case <-ctx.Done():
	case <-t.Context().Done():
	}
}

// awaitEnd waits for the interface's context to be done, failing the test once
// patience runs out.
func awaitEnd(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(signalPatience):
		t.Fatal("the interface's context went on once the command was signaled, want it ended with the program")
	}
}

func TestTheInterfacesContextGoesOnPastExecutesEndAfterAQuit(t *testing.T) {
	t.Parallel()

	// Arrange
	handed := make(chan context.Context, 1)
	quitting := func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		handed <- ctx

		return tui.Next{}, nil
	}
	root := interfaceRoot(t, place{dir: t.TempDir(), home: t.TempDir()}, quitting)
	command, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)

	// Act
	err := root.ExecuteContext(command)

	stop() // Execute's deferred stop, once the command returns

	// Assert
	if err != nil {
		t.Fatalf("workflow = %v, want the interface opened and quit", err)
	}

	lifetime := <-handed
	select {
	case <-lifetime.Done():
		t.Errorf("the interface's context ended with %v once Execute's stop ran after the quit, "+
			"want it to go on, so a write the interface left running is not cut", context.Cause(lifetime))
	case <-time.After(endingPatience):
	}
}

func TestASignalEndsTheContextTheInterfaceRunsUnder(t *testing.T) {
	t.Parallel()

	// Arrange
	handed := make(chan context.Context, 1)
	runningUntilSignaled := func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		handed <- ctx

		untilEnded(ctx, t)

		return tui.Next{}, nil
	}
	root := interfaceRoot(t, place{dir: t.TempDir(), home: t.TempDir()}, runningUntilSignaled)
	signaled := executing(t, root)

	lifetime := <-handed

	// Act
	signaled(errSignaled)

	// Assert
	awaitEnd(t, lifetime.Done())

	cause := context.Cause(lifetime)
	if !errors.Is(cause, errSignaled) {
		t.Errorf("the interface's context ended with %v, want the signal, %v", cause, errSignaled)
	}
}

// A write an interface sent before a switch goes on in the directory left,
// and a signal while the next interface is open still ends it.
func TestASignalEndsTheContextAnInterfaceLeftBeforeASwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	start, other := twoDirectories(t)
	first, opened := make(chan context.Context, 1), make(chan struct{})
	opens := 0
	switchingOnce := func(ctx context.Context, _ tui.Model, _ io.Reader, _ io.Writer) (tui.Next, error) {
		opens++
		if opens == 1 {
			first <- ctx

			return tui.Next{Dir: other}, nil
		}

		close(opened)

		untilEnded(ctx, t)

		return tui.Next{}, nil
	}
	root := interfaceRoot(t, place{dir: start, home: t.TempDir()}, switchingOnce)
	signaled := executing(t, root)

	<-opened

	// Act
	signaled(errSignaled)

	// Assert
	left := <-first
	awaitEnd(t, left.Done())

	cause := context.Cause(left)
	if !errors.Is(cause, errSignaled) {
		t.Errorf("the first interface's context ended with %v, want the signal, %v", cause, errSignaled)
	}
}
