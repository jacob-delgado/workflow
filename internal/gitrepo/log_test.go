// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
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
			commits, err := gitrepo.RecentCommits(t.Context(), run, workDir, "1 day ago")

			// Assert
			if err != nil || len(commits) != tt.want || commits[0].Subject != "Fix the thing" {
				t.Errorf("RecentCommits = %+v, %v, want %d commits", commits, err, tt.want)
			}
		})
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
	_, err := gitrepo.RecentCommits(t.Context(), run, workDir, "1 day ago")

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("RecentCommits returned %v, want ErrNotARepository", err)
	}
}
