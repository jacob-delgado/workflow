// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

var (
	// ErrDirtyTree refuses switching branches with uncommitted work in the tree,
	// which the switch would carry onto the other branch. Stashing is left to the
	// person, so the one sentence every surface tells says what to do.
	ErrDirtyTree = errors.New("the working tree has uncommitted changes; commit or stash them before switching")
	// ErrNothingStaged refuses a commit with nothing in the index.
	ErrNothingStaged = errors.New("nothing is staged to commit")
)

// RefuseDirty refuses a working tree that holds any change — staged, unstaged
// or untracked — before a switch moves it onto another branch.
func RefuseDirty(changes []gitrepo.Change) error {
	if len(changes) > 0 {
		return ErrDirtyTree
	}

	return nil
}

// RefuseNothingStaged refuses a commit when no change is in the index, before it
// runs and fails with git's own less helpful message.
func RefuseNothingStaged(changes []gitrepo.Change) error {
	if slices.ContainsFunc(changes, gitrepo.Change.IsStaged) {
		return nil
	}

	return ErrNothingStaged
}

// OnFeatureBranch reports a branch of its own, not a detached HEAD and not the
// base itself: the branch a rebase, a finish or a pull request is about.
func OnFeatureBranch(branch gitrepo.Branch) bool {
	return branch.Name != "" && branch.Name != branch.BaseName()
}

// CanRebase reports a feature branch with a base to replay it onto.
func CanRebase(branch gitrepo.Branch) bool {
	return OnFeatureBranch(branch) && branch.Base != ""
}

// Foldable is the commits staged changes can be folded into — amended into
// the last, or fixed up into any — oldest first: the branch's commits not yet
// pushed, which alone are safe to rewrite, and none while nothing is staged.
func Foldable(changes []gitrepo.Change, branch gitrepo.Branch) []gitrepo.Commit {
	if !slices.ContainsFunc(changes, gitrepo.Change.IsStaged) || branch.Name == "" {
		return nil
	}

	return branch.Unpushed()
}

// CanMerge reports a pull request ready to merge: open, not a draft, free of
// conflicts, approved with no changes asked for, and its CI green.
func CanMerge(pull forge.PullRequest, checks forge.CI) bool {
	return pull.State == forge.StateOpen &&
		!pull.Draft &&
		pull.Mergeable == forge.MergeClean &&
		pull.Approvals > 0 &&
		!pull.ChangesRequested &&
		checks.State == forge.CIPassed
}

// Why a merged branch cannot be finished, each worded to follow "cannot
// finish BRANCH: " in any forge's words.
var (
	// ErrNotOnFeatureBranch refuses finishing the base itself or a detached
	// HEAD: there is no branch of its own to delete.
	ErrNotOnFeatureBranch = errors.New("no branch of its own is checked out")
	// ErrNotMerged refuses finishing a branch whose pull request has not
	// merged: none was found, it is still open, or it closed without merging.
	ErrNotMerged = errors.New("it has not merged")
	// ErrNoBase refuses finishing a branch whose base was not found, so there
	// is nowhere to switch to.
	ErrNoBase = errors.New("there is no base branch to return to")
	// ErrUnpushedWork refuses finishing a branch whose commits nothing else is
	// known to hold, which the force delete would lose: it is not at the
	// commit that merged, and it has no upstream or is ahead of the one it has.
	ErrUnpushedWork = errors.New(
		"it holds commits that neither merged nor are on its upstream, which deleting it would lose; push them first")
)

// RefuseFinish says why a branch cannot be finished, or nil when it can: it is
// checked out as a branch of its own, its pull request — found says whether
// there is one — has merged, it has a base to return to, and the force delete
// would lose nothing (see keptElsewhere).
func RefuseFinish(pull forge.PullRequest, found bool, branch gitrepo.Branch) error {
	switch {
	case !OnFeatureBranch(branch):
		return ErrNotOnFeatureBranch
	case !found || pull.State != forge.StateMerged:
		return ErrNotMerged
	case branch.Base == "":
		return ErrNoBase
	case !keptElsewhere(pull, branch):
		return ErrUnpushedWork
	default:
		return nil
	}
}

// keptElsewhere reports a merged branch whose every commit something else
// holds, so deleting it loses none: it is at the commit its pull request
// merged, which the forge keeps, or its upstream holds them all. With no
// upstream, a branch behind what merged is not told apart from one ahead of
// it — telling them apart takes git — so it reports false for both.
func keptElsewhere(pull forge.PullRequest, branch gitrepo.Branch) bool {
	atWhatMerged := pull.HeadCommit != "" && branch.Head == pull.HeadCommit

	return atWhatMerged || branch.UpstreamHoldsAll()
}

// CanRerun reports failed CI on an open pull request. A merged one is left
// alone even when a stale read still says failed: nothing needs re-running
// once it is in.
func CanRerun(pull forge.PullRequest, checks forge.CI) bool {
	return pull.State == forge.StateOpen && checks.State == forge.CIFailed
}
