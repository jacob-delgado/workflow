// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// endedRunner answers every command as the process runner does under an
// ended context, and counts the commands it was handed.
func endedRunner() (gitrepo.Runner, *int) {
	calls := 0

	return func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		calls++

		return nil, fmt.Errorf("git: %w", context.Cause(ctx))
	}, &calls
}

// ended is a context its asker has already stopped waiting on.
func ended(t *testing.T) context.Context {
	t.Helper()

	ctx, end := context.WithCancel(t.Context())
	end()

	return ctx
}

// canceledNotMissing reports an error that says the read was canceled, and
// does not say the directory is outside a repository, which a read its asker
// stopped cannot tell.
func canceledNotMissing(err error) bool {
	return errors.Is(err, context.Canceled) && !errors.Is(err, gitrepo.ErrNotARepository)
}

func TestAStatusItsAskerStoppedSaysSoWithoutAskingGitAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	run, calls := endedRunner()

	// Act
	_, err := gitrepo.At(run, workDir).Status(ended(t))

	// Assert
	if !canceledNotMissing(err) || *calls != 1 {
		t.Errorf("Status = %v after %d git runs; want it canceled after one", err, *calls)
	}
}

func TestADescribeItsAskerStoppedSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run, _ := endedRunner()

	// Act
	_, err := gitrepo.At(run, workDir).Describe(ended(t))

	// Assert
	if !canceledNotMissing(err) {
		t.Errorf("Describe = %v; want it canceled, not ErrNotARepository", err)
	}
}
