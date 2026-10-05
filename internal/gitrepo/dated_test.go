// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

const (
	readMyEmail = "git -C /work config user.email"
	myCommits   = "git -C /work log -z --format=%H%x00%h%x00%aI%x00%s --branches --remotes --tags" +
		" --no-merges --fixed-strings --since=2026-10-02T00:00:00Z --author=<me+git@example.com>"
)

func TestCommitsBetweenReadsYourCommitsByTheirAuthorDate(t *testing.T) {
	t.Parallel()

	// Arrange
	// git filters by the date a commit was made; a rebase makes it again, so
	// one written before the period can come back, and is left out by when it
	// was written.
	from := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	log := "1111111111111111111111111111111111111111\x001111111\x002026-10-02T09:15:00+02:00\x00Fix the leak\x00" +
		"2222222222222222222222222222222222222222\x002222222\x002026-09-30T17:00:00Z\x00Rebased in\x00" +
		"3333333333333333333333333333333333333333\x003333333\x002026-10-03T00:00:00Z\x00Just after\x00"
	run := fakeRunner(t, map[string]reply{
		readMyEmail: {out: []byte("me+git@example.com\n")},
		myCommits:   {out: []byte(log)},
	})

	// Act
	commits, err := gitrepo.At(run, workDir).CommitsBetween(t.Context(), from, from.Add(24*time.Hour))

	// Assert
	written := time.Date(2026, 10, 2, 7, 15, 0, 0, time.UTC)
	if err != nil || len(commits) != 1 || commits[0].Short != "1111111" || commits[0].Subject != "Fix the leak" ||
		!commits[0].Authored.Equal(written) {
		t.Errorf("CommitsBetween = %+v, %v; want only the commit written in the period", commits, err)
	}
}

func TestCommitsBetweenRefusesToGuessWhoYouAre(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no user.email there is no telling your commits from anyone's, and a
	// summary of everyone's would not be yours.
	run := fakeRunner(t, map[string]reply{readMyEmail: {err: errNotIgnored}})
	from := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	// Act
	_, err := gitrepo.At(run, workDir).CommitsBetween(t.Context(), from, from.Add(24*time.Hour))

	// Assert
	if !errors.Is(err, gitrepo.ErrNoIdentity) {
		t.Errorf("CommitsBetween error = %v, want ErrNoIdentity", err)
	}
}

func TestSharedDirIsWhereEveryWorktreeKeepsItsHistory(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{
		"git -C /work rev-parse --path-format=absolute --git-common-dir": {out: []byte("/src/api/.git\n")},
	})

	// Act
	shared, err := gitrepo.At(run, workDir).SharedDir(t.Context())

	// Assert
	if err != nil || shared != "/src/api/.git" {
		t.Errorf("SharedDir = %q, %v; want /src/api/.git", shared, err)
	}
}

func TestSharedDirOutsideARepositorySaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{
		"git -C /work rev-parse --path-format=absolute --git-common-dir": {err: errNotARepository},
	})

	// Act
	_, err := gitrepo.At(run, workDir).SharedDir(t.Context())

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("SharedDir outside a repository = %v, want ErrNotARepository", err)
	}
}
