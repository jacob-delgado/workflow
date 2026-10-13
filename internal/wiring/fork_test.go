// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
)

// mainBranch is the branch every repository here starts on, and
// upstreamRemote the remote a fork's clone names for its parent.
const (
	mainBranch     = "main"
	upstreamRemote = "upstream"
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

// forkClone is the git seams on a fork's clone: origin is the fork, upstream
// the repository it came from, and upstream's main a commit past origin's.
// It returns the seams and the clone.
func forkClone(t *testing.T) (seams.Git, string) {
	t.Helper()

	repo, root, _ := gitSeams(t)
	bareRemote(t, root, "origin")
	git(t, root, "branch", "--quiet", "--set-upstream-to=origin/main", "main")
	bareRemote(t, root, upstreamRemote)

	write(t, filepath.Join(root, "UPSTREAM.md"), "upstream moved\n", 0o600)
	git(t, root, "add", "UPSTREAM.md")
	git(t, root, "commit", "--quiet", "-m", "chore: upstream moves")
	git(t, root, "push", "--quiet", "upstream", "main")
	git(t, root, "reset", "--quiet", "--hard", "origin/main")
	git(t, root, "fetch", "--quiet", "upstream")

	return repo, root
}

func TestTheGitSeamsReadAForkCloneAgainstUpstream(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)

	err := repo.CreateBranch(t.Context(), featureBranch, "upstream/main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	write(t, filepath.Join(root, "OWN.md"), "the branch's own\n", 0o600)
	git(t, root, "add", "OWN.md")
	git(t, root, "commit", "--quiet", "-m", "docs: own")

	// Act
	branch, err := repo.Branch(t.Context())

	// Assert
	if err != nil || branch.Base != "upstream/main" || branch.BaseRemote != upstreamRemote {
		t.Fatalf("Branch = %+v, %v; want upstream/main read on upstream", branch, err)
	}

	paths, err := repo.ChangedPaths(t.Context(), "main")
	if err != nil || !slices.Equal(paths, []string{"OWN.md"}) {
		t.Errorf("ChangedPaths(main) = %q, %v; want only the branch's own file", paths, err)
	}
}

func TestAGhBaseOutranksUpstream(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)
	bareRemote(t, root, "parent")
	git(t, root, "config", "remote.parent.gh-resolved", "base")

	// Act
	branch, err := repo.Branch(t.Context())

	// Assert
	if err != nil || branch.BaseRemote != "parent" {
		t.Errorf("Branch = %+v, %v; want the base read on parent, as gh marked it", branch, err)
	}
}

func TestTheGitSeamsReadABranchOntoTheBaseChosen(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	bareRemote(t, root, "origin")

	err := repo.CreateBranch(t.Context(), "release", "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	write(t, filepath.Join(root, "RELEASE.md"), "release only\n", 0o600)
	git(t, root, "add", "RELEASE.md")
	git(t, root, "commit", "--quiet", "-m", "chore: release only")
	git(t, root, "push", "--quiet", "origin", "release")

	err = repo.CreateBranch(t.Context(), featureBranch, "origin/release")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	write(t, filepath.Join(root, "FIX.md"), "the fix\n", 0o600)
	git(t, root, "add", "FIX.md")
	git(t, root, "commit", "--quiet", "-m", "fix: the fix")

	// Act
	branch, err := repo.BranchOnto(t.Context(), "release")

	// Assert
	if err != nil || branch.Base != "origin/release" || len(branch.Commits) != 1 ||
		branch.Commits[0].Subject != "fix: the fix" {
		t.Errorf("BranchOnto(release) = %+v, %v; want only the fix past origin/release", branch, err)
	}
}

func TestTheGitSeamsFetchTheBaseRemote(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)
	originBefore := git(t, root, "rev-parse", "refs/remotes/origin/main")
	upstreamBefore := git(t, root, "rev-parse", "refs/remotes/upstream/main")

	write(t, filepath.Join(root, "LATER.md"), "later\n", 0o600)
	git(t, root, "add", "LATER.md")
	git(t, root, "commit", "--quiet", "-m", "chore: later")
	git(t, root, "push", "--quiet", "--force", "upstream", "HEAD:main")
	git(t, root, "push", "--quiet", "origin", "HEAD:main")
	git(t, root, "update-ref", "refs/remotes/upstream/main", upstreamBefore)
	git(t, root, "update-ref", "refs/remotes/origin/main", originBefore)

	// Act
	err := repo.Fetch(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if git(t, root, "rev-parse", "refs/remotes/upstream/main") == upstreamBefore {
		t.Error("upstream's main did not move, though upstream is the base remote")
	}

	if git(t, root, "rev-parse", "refs/remotes/origin/main") != originBefore {
		t.Error("origin's main moved, though only the base remote is fetched")
	}
}

// A fork's clone's main tracks the fork, whose main is not what the branch
// merged into: the finish catches main up from upstream instead.
func TestTheGitSeamsFinishAForkCloneFromUpstream(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)
	upstreamMain := git(t, root, "rev-parse", "refs/remotes/upstream/main")

	err := repo.CreateBranch(t.Context(), featureBranch, "upstream/main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	branch, err := repo.Branch(t.Context())
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}

	// Act
	err = repo.Finish(t.Context(), branch.Finishing())
	// Assert
	if err != nil {
		t.Fatalf("Finish(%+v): %v", branch.Finishing(), err)
	}

	if got := git(t, root, "rev-parse", "main"); got != upstreamMain {
		t.Errorf("main is at %s after the finish, want upstream's %s", got, upstreamMain)
	}

	if left := git(t, root, "branch", "--list", featureBranch); left != "" {
		t.Errorf("the finished branch is still there: %q", left)
	}
}

func TestTheGitSeamsSwitchToABranchBothRemotesHold(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)
	git(t, root, "push", "--quiet", "origin", "main:release")
	git(t, root, "push", "--quiet", "upstream", "main:release")
	git(t, root, "fetch", "--quiet", "--all")

	// Act
	err := repo.Checkout(t.Context(), "release")
	// Assert
	if err != nil {
		t.Fatalf("Checkout(release) with both remotes holding it: %v", err)
	}

	if tracked := git(t, root, "rev-parse", "--abbrev-ref", "release@{upstream}"); tracked != "upstream/release" {
		t.Errorf("release tracks %q, want upstream/release", tracked)
	}
}

// git push -u fork records the fork as the remote the branch tracks, and a
// plain git push goes there after; so does workflow's.
func TestABranchPushedWithUpstreamToAForkPushesThereAfter(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	bareRemote(t, root, "origin")
	bareRemote(t, root, "fork")

	err := repo.CreateBranch(t.Context(), featureBranch, mainBranch)
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

// With no local main, switching to it creates it from a remote's copy; both
// remotes of a fork's clone hold one, so the finish says which.
func TestTheGitSeamsFinishAForkCloneWithNoLocalBase(t *testing.T) {
	// Arrange
	repo, root := forkClone(t)
	upstreamMain := git(t, root, "rev-parse", "refs/remotes/upstream/main")

	err := repo.CreateBranch(t.Context(), featureBranch, "upstream/main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	git(t, root, "branch", "--quiet", "-D", mainBranch)

	branch, err := repo.Branch(t.Context())
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}

	// Act
	err = repo.Finish(t.Context(), branch.Finishing())
	// Assert
	if err != nil {
		t.Fatalf("Finish(%+v): %v", branch.Finishing(), err)
	}

	if got := git(t, root, "rev-parse", mainBranch); got != upstreamMain {
		t.Errorf("main is at %s after the finish, want upstream's %s", got, upstreamMain)
	}
}
