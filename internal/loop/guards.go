// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

var (
	// ErrDirtyTree refuses switching branches with uncommitted work in the tree,
	// which the switch would carry onto the other branch.
	ErrDirtyTree = errors.New("the working tree has uncommitted changes")
	// ErrNothingStaged refuses a commit with nothing in the index.
	ErrNothingStaged = errors.New("nothing is staged to commit")
)

// RefuseDirty refuses a working tree that holds any change — staged, unstaged
// or untracked — before a switch moves it onto another branch. Stashing is left
// to the person, so a surface's refusal says what to do rather than doing it.
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
