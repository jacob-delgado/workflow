// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestRecentCommitsReadsYourCommitsSince(t *testing.T) {
	t.Parallel()

	const (
		readEmail = "git -C /work config user.email"
		withMine  = "git -C /work log -z --format=%h%x00%s --all --since=1 day ago --author=me@example.com"
		withAll   = "git -C /work log -z --format=%h%x00%s --all --since=1 day ago"
	)

	twoCommits := []byte("abc1234\x00Fix the thing\x00def5678\x00Add retries\x00")

	cases := map[string]struct {
		replies map[string]reply
		want    int
	}{
		"filtered to my email": {
			replies: map[string]reply{readEmail: {out: []byte("me@example.com\n")}, withMine: {out: twoCommits}},
			want:    2,
		},
		"no email set, everyone's": {
			replies: map[string]reply{readEmail: {err: errNotIgnored}, withAll: {out: twoCommits}},
			want:    2,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := fakeRunner(t, tt.replies)

			// Act
			commits, err := gitrepo.At(run, workDir).RecentCommits(t.Context(), "1 day ago")

			// Assert
			if err != nil || len(commits) != tt.want || commits[0].Subject != "Fix the thing" {
				t.Errorf("RecentCommits = %+v, %v, want %d commits", commits, err, tt.want)
			}
		})
	}
}

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

func TestRecentCommitsOutsideARepositoryReportsSo(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		"git -C /work config user.email": {out: []byte("me@example.com\n")},
		"git -C /work log -z --format=%h%x00%s --all --since=1 day ago --author=me@example.com": {err: errNotARepository},
		"git -C /work rev-parse --show-toplevel":                                                {err: errNotARepository},
	}
	run := fakeRunner(t, replies)

	// Act
	_, err := gitrepo.At(run, workDir).RecentCommits(t.Context(), "1 day ago")

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("RecentCommits returned %v, want ErrNotARepository", err)
	}
}
