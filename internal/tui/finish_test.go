// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errFinishDiverged is how a finish fails when git cannot fast-forward the
// base: the step, then git's own words over more than one line, the way the
// runner folds its standard error into the error.
var errFinishDiverged = errors.New("git pull --ff-only: git: exit status 128: " +
	"hint: Diverging branches can't be fast-forwarded.\nfatal: Not possible to fast-forward, aborting")

// errBranchUnread is the branch failing to read again when a finish is
// confirmed.
var errBranchUnread = errors.New("fatal: not a git repository")

// mergedBranch is newWorld with the branch's pull request merged, so the branch
// can be finished.
func mergedBranch() *world {
	w := newWorld()
	w.pull.State = forge.StateMerged

	return w
}

func TestTheReviewPaneShowsAMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged", "F finishes the branch")
}

func TestAMergedBranchSaysWhyItCannotBeFinished(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit was made on the merged branch after it was pushed, so F is left
	// out, and the pane says why rather than leaving it out without a word.
	merged := mergedBranch()
	merged.branch.Ahead, merged.branch.Head = 1, pushedCommit

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged", "cannot be finished", "holds commits that neither merged")
	refuseScreen(t, view, "F finishes the branch")
}

func TestAMergedBranchDoesNotSitOnCheckingCI(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged")
	refuseScreen(t, view, "CI     reading")
}

func TestTheSpineReadsTheReviewOfAMergedBranchAsDone(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merged pull request has no CI to pass, so only its merge can finish the
	// review stage the rail already calls merged.
	merged := mergedBranch()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged")
	requireScreen(t, track(view), "● Commits ━ ● Review ━")
}

func TestFinishIsNotOfferedWithUnpushedCommits(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit was made on the merged branch after it was pushed, so a force
	// delete would lose it.
	merged := mergedBranch()
	merged.branch.Ahead, merged.branch.Head = 1, pushedCommit
	model := merged.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("offered to finish a branch with unpushed commits: %q", calls)
	}
}

func TestFinishIsNotOfferedWithoutABase(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's base could not be found, so there is nowhere to switch to.
	merged := mergedBranch()
	merged.branch.Base = ""
	model := merged.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("offered to finish a branch with no base: %q", calls)
	}
}

func TestFinishRunsTheThreeCommandsOnAMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	done := typing(t, model, "4", "F", keyEnter)

	// Assert
	requireScreen(t, done.View().Content, "● finished "+featureName)

	if calls := merged.asked("finish " + featureName); len(calls) != 1 {
		t.Errorf("finish calls = %q, want one onto the base", merged.asked("finish"))
	}
}

func TestTheFinishPreviewShowsTheThreeCommands(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	preview := typing(t, model, "4", "F").View().Content

	// Assert
	requireScreen(t, preview,
		gitrepo.Finishing{Branch: featureName, Base: baseName, Remote: gitrepo.DefaultRemote}.Commands()...)
}

func TestFinishIsNotOfferedOnAnOpenPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// newWorld's pull request is open.
	open := newWorld()
	model := open.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := open.asked("finish"); len(calls) != 0 {
		t.Errorf("finished an open pull request: %q", calls)
	}
}

func TestRefusedFinishStaysInItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	merged.finishErr = errFinishDiverged
	preview := typing(t, merged.live(t, 120, 40), "4", "F")

	// Act: finish, and git fails
	failed := typing(t, preview, keyEnter)

	// Assert: the preview is still open, git's reason pinned in it whole, ready
	// to try again
	requireScreen(t, failed.View().Content, "git branch -D -- "+featureName, "✗ git pull --ff-only",
		"fatal: Not possible to fast-forward", "enter finish")

	// Act: close it
	closed := typing(t, failed, keyEsc)

	// Assert: the preview is gone, and the failed finish was asked for once
	refuseScreen(t, closed.View().Content, "git branch -D", "fatal: Not possible to fast-forward")

	if calls := merged.asked("finish"); len(calls) != 1 {
		t.Errorf("finish calls = %q, want the one that failed", calls)
	}
}

func TestAFailedFinishCanBeTriedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	merged.finishErr = errFinishDiverged
	failed := typing(t, merged.live(t, 120, 40), "4", "F", keyEnter)
	merged.finishErr = nil

	// Act
	retried := typing(t, failed, keyEnter)

	// Assert
	requireScreen(t, retried.View().Content, "● finished "+featureName)
	refuseScreen(t, retried.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 2 {
		t.Errorf("finish calls = %q, want the failed finish and its retry", calls)
	}
}

func TestEscCancelsTheFinishPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	canceled := typing(t, model, "4", "F", "x", keyEsc)

	// Assert
	refuseScreen(t, canceled.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished after cancel: %q", calls)
	}
}

func TestADryRunFinishSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := mergedBranch()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	previewed := typing(t, model, "4", "F", keyEnter)

	// Assert
	requireScreen(t, previewed.View().Content, "dry run: would finish "+featureName)

	if calls := dry.asked("finish"); len(calls) != 0 {
		t.Errorf("a dry run finished: %q", calls)
	}
}

func TestFinishReadsTheBranchAgainBeforeDeletingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit is made on the branch from another terminal while the preview
	// is open, so the force delete the preview offered would now lose it.
	merged := mergedBranch()
	preview := typing(t, merged.live(t, 120, 40), "4", "F")
	merged.branch.Ahead, merged.branch.Head = 1, pushedCommit

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "holds commits that neither merged")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished a branch that gained a commit after the preview opened: %q", calls)
	}
}

func TestFinishLeavesABranchSwitchedToSinceThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	// Another branch is checked out from another terminal while the preview is
	// open, so finishing now would delete that one instead.
	merged := mergedBranch()
	preview := typing(t, merged.live(t, 120, 40), "4", "F")
	merged.branch.Name = "feat/PROJ-500-other-work"

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "another branch is checked out")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished a branch other than the one previewed: %q", calls)
	}
}

func TestFinishStopsWhenTheBranchCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	deps := merged.deps()
	readOnce := deps.Git.Branch
	reads := 0
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) {
		reads++
		if reads > 1 {
			return gitrepo.Branch{}, errBranchUnread
		}

		return readOnce(ctx)
	}
	model := sized(t, tui.New(merged.cfg, nil, deps), 120, 40)
	preview := typing(t, drain(t, model, model.Init()), "4", "F")

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "not a git repository")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished a branch that could not be read again: %q", calls)
	}
}

func TestFinishLeavesABranchWhoseBaseChangedSinceThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's upstream is reset from another terminal while the preview
	// is open, so finishing now would switch to and pull a base it never
	// showed.
	merged := mergedBranch()
	preview := typing(t, merged.live(t, 120, 40), "4", "F")
	merged.branch.Base = "upstream/release-2"

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "its base changed since")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished onto a base other than the one previewed: %q", calls)
	}
}

// madeSince is a commit made on the branch after its pull request merged.
func madeSince() gitrepo.AfterMerge {
	return gitrepo.AfterMerge{Latest: []gitrepo.Commit{{Hash: "c0ffee1", Subject: "feat: after the merge"}}, Own: 1}
}

func TestAMergedBranchCommittedToSinceItMergedCannotBeFinished(t *testing.T) {
	t.Parallel()

	// Arrange
	// P005: its name reused, or work gone on past the merge, and pushed.
	merged := mergedBranch()
	merged.madeAfter = madeSince()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4", "F").View().Content

	// Assert
	requireScreen(t, view, "cannot be finished", "commits made after it merged", "c0ffee1 feat: after the merge")
	refuseScreen(t, view, "F finishes the branch", "git branch -D")
}

func TestAMergedBranchWhoseCommitsSinceCannotBeReadCannotBeFinished(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	merged.madeAfterErr = errBranchUnread

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "cannot be finished", "reading the commits made after it merged")
	refuseScreen(t, view, "F finishes the branch")
}

func TestFinishLeavesABranchCommittedToSinceThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	preview := typing(t, merged.live(t, 120, 40), "4", "F")
	merged.madeAfter = madeSince()

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "commits made after it merged")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished a branch committed to since the preview: %q", calls)
	}
}

func TestFinishStopsWhenTheCommitsSinceCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	preview := typing(t, merged.live(t, 120, 40), "4", "F")
	merged.madeAfterErr = errBranchUnread

	// Act
	confirmed := typing(t, preview, keyEnter)

	// Assert
	requireScreen(t, confirmed.View().Content, "Finish the branch", "not a git repository")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished a branch whose commits since could not be read: %q", calls)
	}
}

func TestAMergedBranchWhoseMergedCommitCannotBeFetchedCannotBeFinished(t *testing.T) {
	t.Parallel()

	// Arrange
	// Q27: the commit it merged at was pushed from elsewhere, and the fetch
	// of the forge's ref for it failed.
	merged := mergedBranch()
	merged.madeAfterErr = fmt.Errorf("the commit it merged at, c0ffee2, %w: fetching refs/pull/7/head from origin",
		gitrepo.ErrNotFetched)

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "cannot be finished", "could not be fetched", "refs/pull/7/head")
	refuseScreen(t, view, "F finishes the branch")
}

func TestAMergedBranchSomethingElseRefusesIsNotLookedPast(t *testing.T) {
	t.Parallel()

	// Arrange
	// Committed to since it was pushed: refused as it stands, so nothing is
	// fetched to count what was made after the merge.
	merged := mergedBranch()
	merged.branch.Ahead, merged.branch.Head = 1, pushedCommit

	// Act
	typing(t, merged.live(t, 120, 40), "4")

	// Assert
	if calls := merged.asked("made-after"); len(calls) != 0 {
		t.Errorf("read what was made after the merge of a branch refused already: %q", calls)
	}
}
