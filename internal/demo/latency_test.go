// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// latency is how long a slow read takes in these tests' world.
const latency = 500 * time.Millisecond

// slowRead is a read the recording shows the interface waiting on, asked of
// a world whose pull request is open.
type slowRead func(ctx context.Context, deps tui.Deps, pull forge.PullRequest) error

// slowReads are the reads the world's latency slows.
func slowReads() map[string]slowRead {
	return map[string]slowRead{
		"the branches origin holds": func(ctx context.Context, deps tui.Deps, _ forge.PullRequest) error {
			_, err := deps.Git.RemoteBranches(ctx)

			return err
		},
		"the pull request's CI": func(ctx context.Context, deps tui.Deps, pull forge.PullRequest) error {
			_, err := deps.Forge.CheckStatus(ctx, pull, pull.HeadCommit)

			return err
		},
	}
}

// slowWorld is the demo's world with its reads slowed, the feature branch
// pushed and its pull request opened.
func slowWorld(t *testing.T) (tui.Deps, forge.PullRequest) {
	t.Helper()

	deps, _ := pushedFeature(t, demo.ReadLatency(latency))

	pull, err := deps.Forge.CreatePullRequest(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}

	return deps, pull
}

func TestASlowReadTakesTheWorldsLatency(t *testing.T) {
	t.Parallel()

	for name, read := range slowReads() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				deps, pull := slowWorld(t)
				began := time.Now()

				// Act
				err := read(t.Context(), deps, pull)

				// Assert
				if waited := time.Since(began); err != nil || waited != latency {
					t.Errorf("the read answered %v after %v, want an answer after %v", err, waited, latency)
				}
			})
		})
	}
}

func TestASlowReadEndsWithItsContext(t *testing.T) {
	t.Parallel()

	for name, read := range slowReads() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				deps, pull := slowWorld(t)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				began := time.Now()

				// Act
				err := read(ctx, deps, pull)

				// Assert
				if waited := time.Since(began); !errors.Is(err, context.Canceled) || waited != 0 {
					t.Errorf("the read answered %v after %v, want context.Canceled at once", err, waited)
				}
			})
		})
	}
}

// pushLatency is how long a push takes in these tests' world.
const pushLatency = 1200 * time.Millisecond

// slowPush is the feature branch's push begun, in a world whose pushes take
// pushLatency.
func slowPush(t *testing.T) proc.Output {
	t.Helper()

	deps, _ := committedFeature(t, demo.PushLatency(pushLatency))

	output, err := deps.Git.Push(t.Context(), feature)
	if err != nil {
		t.Fatal(err)
	}

	return output
}

// linesOf reads a run's lines to the end.
func linesOf(output proc.Output) []string {
	var lines []string
	for line := range output.Lines {
		lines = append(lines, line)
	}

	return lines
}

func TestASlowPushPrintsAtOnceAndEndsAfterTheLatency(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		output := slowPush(t)
		began := time.Now()

		// Act
		lines := linesOf(output)
		printed := time.Since(began)
		err := output.Wait()

		// Assert
		if len(lines) == 0 || printed != 0 {
			t.Errorf("the push printed %q after %v, want its lines at once", lines, printed)
		}

		if ended := time.Since(began); err != nil || ended != pushLatency {
			t.Errorf("the push ended with %v after %v, want it to end cleanly after %v", err, ended, pushLatency)
		}
	})
}

func TestStoppingASlowPushEndsItAtOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		output := slowPush(t)
		linesOf(output)

		began := time.Now()

		// Act
		output.Stop()

		// Assert
		err := output.Wait()
		if waited := time.Since(began); err == nil || waited != 0 {
			t.Errorf("the stopped push ended with %v after %v, want an error at once", err, waited)
		}
	})
}

func TestStoppingAPushThatHasEndedChangesNothing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		output := slowPush(t)
		linesOf(output)

		ended := output.Wait()
		if ended != nil {
			t.Fatal(ended)
		}

		// Act
		output.Stop()

		// Assert
		err := output.Wait()
		if err != nil {
			t.Errorf("the push, stopped once it had ended, ended with %v, want it to end cleanly", err)
		}
	})
}
