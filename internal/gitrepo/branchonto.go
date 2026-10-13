// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoSuchBase refuses a base to merge into that neither the base remote nor
// the repository holds a branch of.
var ErrNoSuchBase = errors.New("no branch of that name to merge into")

// ReadBranchOnto reads the checked-out branch as ReadBranch does, but against
// base, a branch a pull request is to merge into, rather than the base remote's
// default: its Base, and so its commits, are those past base. base is read as
// the base remote's copy when it has one, else the local branch; one neither
// holds is ErrNoSuchBase.
func (r Repository) ReadBranchOnto(ctx context.Context, base string) (Branch, error) {
	return r.readBranch(ctx, func(git func(...string) string, remote string) (string, error) {
		return chosenBase(git, remote, base)
	})
}

// chosenBase is the ref base is read at: the remote's copy when it holds one,
// else the local branch.
func chosenBase(git func(...string) string, remote, base string) (string, error) {
	err := notAnOption(base)
	if err != nil {
		return "", err
	}

	if git("rev-parse", "--verify", "--quiet", "refs/remotes/"+remoteBranch(remote, base)) != "" {
		return remoteBranch(remote, base), nil
	}

	if git("rev-parse", "--verify", "--quiet", "refs/heads/"+base) != "" {
		return base, nil
	}

	return "", fmt.Errorf("%w: %s", ErrNoSuchBase, base)
}
