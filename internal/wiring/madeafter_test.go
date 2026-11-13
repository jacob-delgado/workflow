// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// pullRef is where the forge keeps pull request 7's head, as GitHub does: a
// forge.kind left unset reads as GitHub.
const pullRef = "refs/pull/7/head"

// mergedElsewhere has the repository at root on featureBranch, pushed to an
// origin on disk, and its pull request 7 merged at a commit shape makes from
// another clone and pushes only to pushedTo there, which this clone never
// fetched. It answers that pull request.
func mergedElsewhere(
	t *testing.T, repo seams.Git, root string, shape func(other string), pushedTo string,
) forge.PullRequest {
	t.Helper()

	err := repo.CreateBranch(t.Context(), featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	git(t, root, "commit", "--quiet", "--allow-empty", "-m", "feat: the work")

	origin := filepath.Join(t.TempDir(), "origin.git")
	git(t, root, "clone", "--quiet", "--bare", root, origin)
	git(t, root, "remote", "add", "origin", origin)

	other := t.TempDir()
	git(t, other, "clone", "--quiet", "--branch", featureBranch, origin, ".")
	git(t, other, "config", "user.email", "test@example.com")
	git(t, other, "config", "user.name", "Test")
	shape(other)
	git(t, other, "push", "--quiet", "origin", "HEAD:"+pushedTo)

	return forge.PullRequest{Number: 7, State: forge.StateMerged, HeadCommit: git(t, other, "rev-parse", "HEAD")}
}

// onTop makes a commit on top of the branch in other, as a review's
// suggestion applied on the forge is.
func onTop(t *testing.T) func(other string) {
	t.Helper()

	return func(other string) {
		git(t, other, "commit", "--quiet", "--allow-empty", "-m", "fix: a suggestion applied on the forge")
	}
}

// inPlace replaces the branch's last commit in other with another, as a
// force-push from elsewhere does.
func inPlace(t *testing.T) func(other string) {
	t.Helper()

	return func(other string) {
		git(t, other, "reset", "--quiet", "--hard", "HEAD~1")
		git(t, other, "commit", "--quiet", "--allow-empty", "-m", "feat: the work, rewritten")
	}
}

func TestTheGitSeamsFetchTheMergedCommitAndFindNothingMadeOnABranchBehindIt(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	merged := mergedElsewhere(t, repo, root, onTop(t), pullRef)

	// Act
	made, err := repo.MadeAfter(t.Context(), merged)

	// Assert
	if err != nil || made.Any() {
		t.Errorf("MadeAfter = %v, %v; want none, the branch an ancestor of what merged", made, err)
	}

	if held := git(t, root, "cat-file", "-t", merged.HeadCommit); held != "commit" {
		t.Errorf("the merged commit reads as %q here, want it fetched", held)
	}
}

func TestTheGitSeamsFetchTheMergedCommitAndNameWhatCameAfterIt(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	merged := mergedElsewhere(t, repo, root, inPlace(t), pullRef)

	// Act
	made, err := repo.MadeAfter(t.Context(), merged)

	// Assert
	if err != nil || made.Own != 1 || made.Latest[0].Subject != "feat: the work" {
		t.Errorf("MadeAfter = %v, %v; want feat: the work, made past what merged", made, err)
	}
}

func TestTheGitSeamsSayWhenTheMergedCommitCannotBeFetched(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)
	merged := mergedElsewhere(t, repo, root, onTop(t), "refs/heads/elsewhere")

	// Act
	_, err := repo.MadeAfter(t.Context(), merged)

	// Assert
	if !errors.Is(err, gitrepo.ErrNotFetched) {
		t.Errorf("MadeAfter = %v, want gitrepo.ErrNotFetched", err)
	}
}

func TestTheGitSeamsNameOnlyWhatTheBaseLacksOnABranchReusedAfterASquashMerge(t *testing.T) {
	// Arrange
	repo, root, _ := gitSeams(t)

	err := repo.CreateBranch(t.Context(), featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	git(t, root, "commit", "--quiet", "--allow-empty", "-m", "feat: the work")
	merged := forge.PullRequest{
		Number: 7, State: forge.StateMerged, HeadCommit: git(t, root, "rev-parse", "HEAD"), Base: "main",
	}

	origin := filepath.Join(t.TempDir(), "origin.git")
	git(t, root, "init", "--quiet", "--bare", origin)
	git(t, root, "remote", "add", "origin", origin)
	git(t, root, "switch", "--quiet", "main")
	git(t, root, "commit", "--quiet", "--allow-empty", "-m", "feat: the work (#7)")
	git(t, root, "commit", "--quiet", "--allow-empty", "-m", "fix: another pull request")
	git(t, root, "push", "--quiet", "origin", "main")
	git(t, root, "switch", "--quiet", featureBranch)
	git(t, root, "reset", "--quiet", "--hard", "origin/main")
	git(t, root, "commit", "--quiet", "--allow-empty", "-m", "feat: my new work")

	// Act
	made, err := repo.MadeAfter(t.Context(), merged)

	// Assert
	if err != nil || len(made.Latest) != 1 || made.Latest[0].Subject != "feat: my new work" || made.Own != 1 ||
		made.OnBase != 2 || made.Base != "origin/main" {
		t.Errorf("MadeAfter = %+v, %v; want feat: my new work, beside 2 already on origin/main", made, err)
	}
}
