// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"path/filepath"
	"testing"
)

// bareRemote is a new bare repository cloned from root and added to it as
// remote name, fetched, so the remote's branches are known to root.
func bareRemote(t *testing.T, root, name string) string {
	t.Helper()

	bare := filepath.Join(t.TempDir(), name+".git")
	git(t, root, "clone", "--quiet", "--bare", root, bare)
	git(t, root, "remote", "add", name, bare)
	git(t, root, "fetch", "--quiet", name)

	return bare
}

func TestTheGitSeamsPushToTheBranchPushRemote(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	origin := bareRemote(t, root, "origin")
	fork := bareRemote(t, root, "fork")

	err := repo.CreateBranch(t.Context(), featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	git(t, root, "config", "branch."+featureBranch+".pushRemote", "fork")

	// Act
	push, err := repo.Push(t.Context(), featureBranch)
	if err != nil {
		t.Fatalf("Push did not start: %v", err)
	}

	lines, err := drained(push)
	// Assert
	if err != nil {
		t.Fatalf("Push ended %v saying %q", err, lines)
	}

	if got := git(t, fork, "branch", "--list", featureBranch); got == "" {
		t.Error("the fork does not hold the branch its pushRemote names")
	}

	if got := git(t, origin, "branch", "--list", featureBranch); got != "" {
		t.Errorf("origin holds the branch, %q, though its pushRemote is the fork", got)
	}
}

// git push -u fork records the fork as the remote the branch tracks, and a
// plain git push goes there after; so does workflow's.
func TestABranchPushedWithUpstreamToAForkPushesThereAfter(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	bareRemote(t, root, "origin")
	bareRemote(t, root, "fork")

	err := repo.CreateBranch(t.Context(), featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	git(t, root, "push", "--quiet", "-u", "fork", featureBranch)

	// Act
	branch, err := repo.Branch(t.Context())

	// Assert
	if err != nil || branch.PushRemote != "fork" || !branch.Pushed() {
		t.Errorf("Branch = %+v, %v; want it pushed, and pushing, to fork", branch, err)
	}
}
