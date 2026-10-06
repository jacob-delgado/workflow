// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestRecentSubjectsReadsTheMostRecentSubjects(t *testing.T) {
	t.Parallel()

	// Arrange
	const readSubjects = "git -C /work log --format=%s -n 200"

	replies := map[string]reply{
		readSubjects: {out: []byte("feat(tui): add a pane\nfix: a typo\n\nchore(deps): bump\n")},
	}
	run := fakeRunner(t, replies)

	// Act
	subjects, err := gitrepo.At(run, workDir).RecentSubjects(t.Context())

	// Assert
	// The blank line between commits is dropped; the rest keep their order.
	want := []string{"feat(tui): add a pane", "fix: a typo", "chore(deps): bump"}
	if err != nil || !slices.Equal(subjects, want) {
		t.Errorf("RecentSubjects = %q, %v, want %q", subjects, err, want)
	}
}

func TestRecentSubjectsOutsideARepositoryReportsSo(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		"git -C /work log --format=%s -n 200":    {err: errNotARepository},
		"git -C /work rev-parse --show-toplevel": {err: errNotARepository},
	}
	run := fakeRunner(t, replies)

	// Act
	_, err := gitrepo.At(run, workDir).RecentSubjects(t.Context())

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("RecentSubjects returned %v, want ErrNotARepository", err)
	}
}
