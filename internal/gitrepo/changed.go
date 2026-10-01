// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrOptionLikeRef refuses a ref starting with a dash, which git would read as
// an option rather than a revision.
var ErrOptionLikeRef = errors.New("a ref cannot start with a dash")

// ChangedPaths lists the paths the checked-out branch changes since it left
// base: `git diff base...HEAD`, renames read as a delete and an add so both
// paths are listed. base is a branch name, read as origin's copy when origin
// has one, since that is what a pull request merges into. The paths are for
// matching, not for showing, so one that would draw differently — a joined
// emoji, a tab — is kept; only one holding a control character other than a
// tab, which no one names a file with to own it, is left out.
func (r Repository) ChangedPaths(ctx context.Context, base string) ([]string, error) {
	ref, err := r.baseRef(ctx, base)
	if err != nil {
		return nil, err
	}

	out, err := r.run(ctx, gitProgram, "-C", r.dir, "diff", "--name-only", "-z", "--no-renames", ref+"...HEAD")
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "listing the paths changed since "+base, err)
	}

	var paths []string

	for path := range strings.SplitSeq(string(out), "\x00") {
		if path != "" && !strings.ContainsFunc(path, isControlNotTab) {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

// isControlNotTab reports a control character other than a tab.
func isControlNotTab(character rune) bool {
	return character != '\t' && unicode.IsControl(character)
}

// baseRef is the ref a base branch is read at: origin's copy of it when one
// has been fetched, and the name as given otherwise.
func (r Repository) baseRef(ctx context.Context, base string) (string, error) {
	err := notAnOption(base)
	if err != nil {
		return "", err
	}

	remote := remoteBranch(base)
	if optional(ctx, r.run, "-C", r.dir, "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote) != "" {
		return remote, nil
	}

	return base, nil
}

// notAnOption refuses a ref git would read as an option.
func notAnOption(ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("%w: %q", ErrOptionLikeRef, ref)
	}

	return nil
}

// FileAt reads the first of paths present at ref, reporting false when none
// is. A ref the repository does not have — a base never fetched — is read as
// HEAD instead: the branch's own copy of a file is usually the base's, and a
// guess beats nothing for a suggestion. Only a git that timed out is an error;
// any other failure to show a path means it is not there.
func (r Repository) FileAt(ctx context.Context, ref string, paths []string) ([]byte, bool, error) {
	err := notAnOption(ref)
	if err != nil {
		return nil, false, err
	}

	ref, err = r.presentOrHead(ctx, ref)
	if err != nil {
		return nil, false, err
	}

	for _, path := range paths {
		content, err := r.run(ctx, gitProgram, "-C", r.dir, "show", ref+":"+path)
		if errors.Is(err, proc.ErrTimedOut) {
			return nil, false, fmt.Errorf("reading %s at %s: %w", path, ref, err)
		}

		if err == nil {
			return content, true, nil
		}
	}

	return nil, false, nil
}

// presentOrHead is ref when the repository has that commit, and HEAD when not.
func (r Repository) presentOrHead(ctx context.Context, ref string) (string, error) {
	_, err := r.run(ctx, gitProgram, "-C", r.dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")

	switch {
	case err == nil:
		return ref, nil
	case errors.Is(err, proc.ErrTimedOut):
		return "", fmt.Errorf("finding %s: %w", ref, err)
	default:
		return headRef, nil
	}
}
