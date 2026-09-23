// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// output is a finished program's output: its lines, already written, and how it
// exited.
func output(lines []string, waitErr error) proc.Output {
	channel := make(chan string, len(lines))
	for _, line := range lines {
		channel <- line
	}

	close(channel)

	return proc.Output{Lines: channel, Wait: func() error { return waitErr }, Stop: func() {}}
}

// recordingPush is a push that succeeds and records each branch it was asked to
// push.
func recordingPush(pushed *[]string) func(string) (proc.Output, error) {
	return func(branch string) (proc.Output, error) {
		*pushed = append(*pushed, branch)

		return output([]string{"To origin"}, nil), nil
	}
}

func TestPushPublishesTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	var pushed []string

	// Act
	err := loop.Push(recordingPush(&pushed), branchName)

	// Assert
	if err != nil || !slices.Equal(pushed, []string{branchName}) {
		t.Errorf("Push = %v, pushed %q; want %q pushed once", err, pushed, branchName)
	}
}

func TestPushCarriesTheOutputOfAFailedPush(t *testing.T) {
	t.Parallel()

	// Arrange
	push := func(string) (proc.Output, error) {
		return output([]string{"! [rejected]", "hint: fetch first"}, errSeam), nil
	}

	// Act
	err := loop.Push(push, branchName)

	// Assert
	var failed loop.PushFailedError
	if !errors.Is(err, loop.ErrPushFailed) || !errors.As(err, &failed) ||
		!slices.Equal(failed.Output, []string{"! [rejected]", "hint: fetch first"}) {
		t.Fatalf("Push returned %v, want ErrPushFailed carrying the push's output", err)
	}

	if err.Error() != "the push failed:\n! [rejected]\nhint: fetch first" {
		t.Errorf("Push error = %q, want the failure followed by the output", err.Error())
	}
}

func TestPushReturnsAPushThatCannotStart(t *testing.T) {
	t.Parallel()

	// Arrange
	push := func(string) (proc.Output, error) { return proc.Output{}, errSeam }

	// Act
	err := loop.Push(push, branchName)

	// Assert
	if !errors.Is(err, errSeam) || errors.Is(err, loop.ErrPushFailed) {
		t.Errorf("Push returned %v, want the start's own error, not a failed push", err)
	}
}

func TestPushWithoutAPushSeamIsUnavailable(t *testing.T) {
	t.Parallel()

	// Act
	err := loop.Push(nil, branchName)

	// Assert
	if !errors.Is(err, loop.ErrPushUnavailable) {
		t.Errorf("Push returned %v, want ErrPushUnavailable", err)
	}
}

func TestEnsurePushedSkipsAPushedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	var pushed []string

	branch := openable(branchName)
	branch.Upstream = "origin/" + branchName

	// Act
	err := loop.EnsurePushed(recordingPush(&pushed), branch)

	// Assert
	if err != nil || len(pushed) != 0 {
		t.Errorf("EnsurePushed = %v, pushed %q; want nothing pushed", err, pushed)
	}
}

func TestEnsurePushedPushesAnUnpublishedBranch(t *testing.T) {
	t.Parallel()

	cases := map[string]gitrepo.Branch{
		"a branch with no upstream": openable(branchName),
		"a branch ahead of its upstream": {
			Name: branchName, Upstream: "origin/" + branchName, Ahead: 1,
		},
	}

	for name, branch := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var pushed []string

			// Act
			err := loop.EnsurePushed(recordingPush(&pushed), branch)

			// Assert
			if err != nil || !slices.Equal(pushed, []string{branchName}) {
				t.Errorf("EnsurePushed = %v, pushed %q; want %q pushed", err, pushed, branchName)
			}
		})
	}
}
