// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// hungRunner answers every command as a run the bound gave up on, the way git
// on a dead network mount ends, and counts the commands it was handed.
func hungRunner() (gitrepo.Runner, *int) {
	calls := 0

	return func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		calls++

		return nil, fmt.Errorf("git: %w after 30s", proc.ErrTimedOut)
	}, &calls
}

// timedOutNotMissing reports an error that says git timed out, and does not say
// the directory is outside a repository, which a timeout cannot tell.
func timedOutNotMissing(err error) bool {
	return errors.Is(err, proc.ErrTimedOut) && !errors.Is(err, gitrepo.ErrNotARepository)
}

func TestAStatusThatTimedOutSaysSoWithoutAskingGitAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	run, calls := hungRunner()

	// Act
	_, err := gitrepo.At(run, workDir).Status(t.Context())

	// Assert
	if !timedOutNotMissing(err) || *calls != 1 {
		t.Errorf("Status = %v after %d git runs; want the timeout after one", err, *calls)
	}
}

func TestADescribeThatTimedOutSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run, _ := hungRunner()

	// Act
	_, err := gitrepo.At(run, workDir).Describe(t.Context())

	// Assert
	if !timedOutNotMissing(err) {
		t.Errorf("Describe = %v; want the timeout, not ErrNotARepository", err)
	}
}

func TestAnIgnoreCheckWhoseProbeTimedOutSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run, _ := hungRunner()

	// Act
	_, err := gitrepo.At(run, workDir).CheckIgnored(t.Context(), ".workflow.json")

	// Assert
	if !timedOutNotMissing(err) {
		t.Errorf("CheckIgnored = %v; want the timeout, not ErrNotARepository", err)
	}
}
