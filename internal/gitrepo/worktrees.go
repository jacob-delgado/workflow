// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"strings"
)

// Worktree is one working tree of a repository: where it is, which branch it
// has checked out — none when its HEAD is detached — and whether git keeps it
// from being removed (Locked) or finds its directory gone (Missing).
type Worktree struct {
	Dir      string
	Branch   string
	Head     string
	Detached bool
	Locked   bool
	Missing  bool
}

// Worktrees is every working tree of the repository, the main one first, as
// git lists them. A bare repository's entry has no working tree and is left
// out.
func (r Repository) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, notInWorkTree(r.dir, err)
	}

	var worktrees []Worktree

	for entry := range strings.SplitSeq(string(out), "\x00\x00") {
		worktree, isWorktree := parseWorktree(entry)
		if isWorktree {
			worktrees = append(worktrees, worktree)
		}
	}

	return worktrees, nil
}

// parseWorktree reads one worktree's NUL-separated attributes, and reports
// whether they name a working tree: neither an empty entry nor a bare one.
func parseWorktree(entry string) (Worktree, bool) {
	var worktree Worktree

	bare := false

	for attribute := range strings.SplitSeq(entry, "\x00") {
		label, value, _ := strings.Cut(attribute, " ")

		switch label {
		case "worktree":
			worktree.Dir = value
		case "HEAD":
			worktree.Head = value
		case "branch":
			worktree.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			worktree.Detached = true
		case "locked":
			worktree.Locked = true
		case "prunable":
			worktree.Missing = true
		case "bare":
			bare = true
		}
	}

	return worktree, worktree.Dir != "" && !bare
}
