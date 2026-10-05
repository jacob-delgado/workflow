// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// listWorktrees is how Worktrees asks git for them.
const listWorktrees = "git -C /work worktree list --porcelain -z"

// headHash is the commit every worktree in the cases has checked out.
const headHash = "300a7be68d29eb9302518fd80f1cf20267d6feba"

func TestWorktreesAreReadAsGitListsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	// As git 2.43 writes them under -z: each attribute ended by a NUL, each
	// worktree by one more.
	listing := "worktree /src/api\x00HEAD " + headHash + "\x00branch refs/heads/main\x00\x00" +
		"worktree /src/api-feat-x\x00HEAD " + headHash + "\x00branch refs/heads/feat/x\x00\x00" +
		"worktree /src/api-review\x00HEAD " + headHash + "\x00detached\x00locked\x00\x00" +
		"worktree /src/api-gone\x00HEAD " + headHash + "\x00branch refs/heads/gone\x00" +
		"prunable gitdir file points to non-existent location\x00\x00"
	run := fakeRunner(t, map[string]reply{listWorktrees: {out: []byte(listing)}})

	// Act
	worktrees, err := gitrepo.At(run, workDir).Worktrees(t.Context())

	// Assert
	want := []gitrepo.Worktree{
		{Dir: "/src/api", Branch: "main", Head: headHash},
		{Dir: "/src/api-feat-x", Branch: "feat/x", Head: headHash},
		{Dir: "/src/api-review", Head: headHash, Detached: true, Locked: true},
		{Dir: "/src/api-gone", Branch: "gone", Head: headHash, Missing: true},
	}
	if err != nil || !slices.Equal(worktrees, want) {
		t.Errorf("Worktrees = %+v, %v; want %+v", worktrees, err, want)
	}
}

func TestABareRepositoryIsNoWorktree(t *testing.T) {
	t.Parallel()

	// Arrange
	listing := "worktree /src/api.git\x00bare\x00\x00" +
		"worktree /src/api\x00HEAD " + headHash + "\x00branch refs/heads/main\x00\x00"
	run := fakeRunner(t, map[string]reply{listWorktrees: {out: []byte(listing)}})

	// Act
	worktrees, err := gitrepo.At(run, workDir).Worktrees(t.Context())

	// Assert
	if err != nil || len(worktrees) != 1 || worktrees[0].Dir != "/src/api" {
		t.Errorf("Worktrees = %+v, %v; want /src/api alone", worktrees, err)
	}
}

func TestWorktreesOutsideARepositorySayWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{listWorktrees: {err: errNotARepository}})

	// Act
	_, err := gitrepo.At(run, workDir).Worktrees(t.Context())

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("Worktrees outside a repository = %v, want ErrNotARepository", err)
	}
}
