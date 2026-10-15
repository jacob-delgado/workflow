// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// The feature branch the finish tests merge and finish, and the base it
// returns to.
const (
	finishedBranch = "fix/PROJ-2-thing"
	finishBase     = "main"
)

// finishRepo is a repository on finishedBranch, one commit ahead of main, with
// a GitHub origin whose pull request for it clitest.FakeGh answers with pulls, handed
// the commit the branch is at.
func finishRepo(t *testing.T, pulls func(head string) string) string {
	t.Helper()

	repo := clitest.GithubRepo(t, finishedBranch)
	clitest.FakeGh(t, clitest.GhResponses{Pulls: pulls(headOf(t, repo))})
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

// headOf is the commit repo has checked out.
func headOf(t *testing.T, repo string) string {
	t.Helper()

	return clitest.GitOutput(t, repo, "rev-parse", "HEAD")
}

// mergedAtHead is the branch's pull request merged at head, the commit the
// branch is at: nothing on it is anywhere but what merged.
func mergedAtHead(head string) string {
	return mergedPullAt("work", head)
}

// listing is a pull request listing that is pulls whatever the branch is at.
func listing(pulls string) func(string) string {
	return func(string) string { return pulls }
}

// mergedFinishRepo is finishRepo with the branch's pull request merged at the
// commit the branch is at.
func mergedFinishRepo(t *testing.T) string {
	t.Helper()

	return finishRepo(t, mergedAtHead)
}

// originOnDisk moves origin to a bare repository on disk holding main, at a
// file:// address on github.com: git reads such an address by its path alone,
// so the finish's pull catches main up from disk, while the forge is still
// found at github.com.
func originOnDisk(t *testing.T, repo string) {
	t.Helper()

	bare := t.TempDir()
	clitest.Git(t, bare, "init", "--bare", "--quiet")
	clitest.Git(t, repo, "remote", "set-url", "origin", "file://github.com"+filepath.ToSlash(bare))
	clitest.Git(t, repo, "push", "--quiet", "origin", "main")
	clitest.Git(t, repo, "fetch", "--quiet", "origin")
}

// divergedBase is originOnDisk with main and origin's main each holding a
// commit the other lacks, so pull --ff-only refuses to catch main up.
func divergedBase(t *testing.T, repo string) {
	t.Helper()

	originOnDisk(t, repo)
	clitest.Git(t, repo, "switch", "--quiet", "main")
	clitest.Commit(t, repo, "origin only")
	clitest.Git(t, repo, "push", "--quiet", "origin", "main")
	clitest.Git(t, repo, "reset", "--quiet", "--hard", "HEAD~1")
	clitest.Commit(t, repo, "local only")
	clitest.Git(t, repo, "switch", "--quiet", finishedBranch)
}

// committedSincePushed has the branch pushed, then committed to again, so
// its upstream lacks a commit a delete would lose.
func committedSincePushed(t *testing.T, repo string) {
	t.Helper()

	clitest.PretendPushed(t, repo)
	clitest.Commit(t, repo, "after the merge")
}

// committedSinceMerged has the branch committed to after what merged, with no
// upstream — never pushed under its own name, or one the forge deleted after
// the merge and a prune dropped — so the commit is on no remote at all.
func committedSinceMerged(t *testing.T, repo string) {
	t.Helper()

	clitest.Commit(t, repo, "after the merge")
}

// baseRenamed renames main to a name no base is looked for by, so the
// branch has no base to return to.
func baseRenamed(t *testing.T, repo string) {
	t.Helper()

	clitest.Git(t, repo, "branch", "--move", "main", "trunk")
}

// requireBranchKept fails the test unless repo still holds finishedBranch.
func requireBranchKept(t *testing.T, repo string) {
	t.Helper()

	if listed := clitest.GitOutput(t, repo, "branch", "--list", finishedBranch); listed == "" {
		t.Errorf("%s was deleted, want it kept", finishedBranch)
	}
}

func TestFinishDryRunPrintsTheThreeCommandsAndRunsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedFinishRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("finish --dry-run: %v (%+v)", err, printed)
	}

	for _, want := range []string{
		"Finish " + finishedBranch + ": #7 merged",
		"  git -c checkout.defaultRemote=origin switch -- main",
		"  git pull --ff-only -- origin main",
		"  git branch -D -- " + finishedBranch,
	} {
		if !strings.Contains(printed.Stdout, want) {
			t.Errorf("stdout = %q, want %q", printed.Stdout, want)
		}
	}

	dryRun := "dry run: would finish " + finishedBranch + " (switch to main, pull it from origin, delete it)"
	if !strings.Contains(printed.Stderr, dryRun) {
		t.Errorf("stderr = %q, want %q", printed.Stderr, dryRun)
	}

	if current := clitest.CurrentBranch(t, repo); current != finishedBranch {
		t.Errorf("checked out %s, want %s left checked out", current, finishedBranch)
	}
}

func TestFinishYesDeletesTheMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedFinishRepo(t)
	originOnDisk(t, repo)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("finish --yes: %v (%+v)", err, printed)
	}

	if current := clitest.CurrentBranch(t, repo); current != finishBase {
		t.Errorf("checked out %s, want %s", current, finishBase)
	}

	if listed := clitest.GitOutput(t, repo, "branch", "--list", finishedBranch); listed != "" {
		t.Errorf("branches = %q, want %s deleted", listed, finishedBranch)
	}

	if want := "deleted " + finishedBranch; !strings.Contains(printed.Stderr, want) {
		t.Errorf("stderr = %q, want it to say %q", printed.Stderr, want)
	}
}

func TestFinishAsksBeforeRunning(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedFinishRepo(t)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "finish")
	// Assert
	if err != nil {
		t.Fatalf("finish declined: %v (%+v)", err, printed)
	}

	want := "Finish " + finishedBranch + ": switch to main, pull it from origin, and delete it? [y/N]: "
	if len(asked) != 1 || asked[0] != want {
		t.Errorf("asked %q, want only %q", asked, want)
	}

	if !strings.Contains(printed.Stderr, "Not finished.") {
		t.Errorf("stderr = %q, want it to say it was not finished", printed.Stderr)
	}

	requireBranchKept(t, repo)
}

func TestFinishWithoutATerminalSaysToPassYes(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedFinishRepo(t)
	noTerminal := cmdkit.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := clitest.RunStreams(t, repo, noTerminal, "finish")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("finish with no terminal = %v, want it to say to pass --yes", err)
	}

	wantExit(t, err, 2)
	requireBranchKept(t, repo)
}

func TestFinishRefusesWhatCannotBeFinished(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pulls  func(head string) string
		shape  func(t *testing.T, repo string)
		reason error
	}{
		"an open pull request": {pulls: listing(clitest.OpenPull("work")), reason: loop.ErrNotMerged},
		"one closed unmerged":  {pulls: listing(clitest.ClosedPull("work")), reason: loop.ErrNotMerged},
		"a commit not pushed":  {pulls: mergedAtHead, reason: loop.ErrUnpushedWork, shape: committedSincePushed},
		"a commit since the merge, no upstream": {
			pulls: mergedAtHead, reason: loop.ErrUnpushedWork, shape: committedSinceMerged,
		},
		"merged at a commit the forge did not name": {
			pulls: listing(clitest.MergedPull("work")), reason: loop.ErrUnpushedWork,
		},
		"no base to return to": {pulls: mergedAtHead, reason: loop.ErrNoBase, shape: baseRenamed},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := finishRepo(t, tt.pulls)
			if tt.shape != nil {
				tt.shape(t, repo)
			}

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")

			// Assert
			if !errors.Is(err, tt.reason) || !strings.Contains(err.Error(), "cannot finish "+finishedBranch+": ") {
				t.Errorf("finish = %v, want %v, naming the branch", err, tt.reason)
			}

			wantExit(t, err, 4)
			requireBranchKept(t, repo)
		})
	}
}

func TestFinishOnTheBaseAsksNoForge(t *testing.T) {
	t.Parallel()

	// Arrange
	// No origin, so a forge asked would fail as configuration, exit 3: exit 4
	// is the refusal made before the forge is asked.
	repo := clitest.PRRepo(t, finishedBranch)
	clitest.Git(t, repo, "switch", "--quiet", "main")

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")

	// Assert
	if !errors.Is(err, loop.ErrNotOnFeatureBranch) || !strings.Contains(err.Error(), "cannot finish main") {
		t.Errorf("finish on main = %v, want it refused as no branch of its own", err)
	}

	wantExit(t, err, 4)
}

func TestFinishOutsideARepositoryRefuses(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "finish", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "reading the branch") {
		t.Errorf("finish outside a repository = %v, want the branch read refused", err)
	}

	wantExit(t, err, 4)
}

func TestFinishSaysWhyThePullRequestCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A feature branch with no origin: the forge has no repository to read.
	repo := clitest.PRRepo(t, finishedBranch)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "reading the pull request of "+finishedBranch) {
		t.Errorf("finish with no origin = %v, want the pull request's read to fail", err)
	}

	wantExit(t, err, 3)
	requireBranchKept(t, repo)
}

func TestFinishKeepsGitsWordsWhenAStepFails(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedFinishRepo(t)
	divergedBase(t, repo)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "finishing "+finishedBranch+": git pull --ff-only") ||
		!strings.Contains(err.Error(), "fast-forward") {
		t.Errorf("finish onto a diverged main = %v, want git's own words for the pull", err)
	}

	wantExit(t, err, 1)
	requireBranchKept(t, repo)
}

func TestFinishNamesAMergeRequestInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := clitest.GitlabRepo(t, finishedBranch)
	clitest.FakeGlabListing(t, `[{"iid":7,"web_url":"`+clitest.GitlabMergeRequest+`","title":"work","state":"merged",`+
		`"sha":"`+headOf(t, repo)+`"}]`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "finish", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("finish --dry-run on GitLab: %v (%+v)", err, printed)
	}

	requireGitLabWords(t, printed.Stdout, "Finish "+finishedBranch+": !7 merged")
}

// mergedPullAt is clitest.MergedPull with its branch at head on the forge, the commit
// the merge took, as GitHub reports it.
func mergedPullAt(title, head string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"closed","merged_at":"2026-01-01T00:00:00Z","head":{"sha":"` + head + `"}}]`
}
