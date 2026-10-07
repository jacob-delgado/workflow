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

// CanFinish reports a merged pull request's branch that can be finished: it is
// checked out with a base to return to, and holds no commit origin lacks,
// which the force delete would lose.
func CanFinish(pull forge.PullRequest, branch gitrepo.Branch) bool {
	return pull.State == forge.StateMerged && OnFeatureBranch(branch) && branch.Base != "" &&
		!branch.HasUnpushedWork()
}

// CanRerun reports failed CI on an open pull request. A merged one is left
// alone even when a stale read still says failed: nothing needs re-running
// once it is in.
func CanRerun(pull forge.PullRequest, checks forge.CI) bool {
	return pull.State == forge.StateOpen && checks.State == forge.CIFailed
}
