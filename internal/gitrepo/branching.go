// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// CreateBranch creates a branch at start and switches to it, or at HEAD when
// start is empty.
//
// --no-track matters when start is a remote branch such as origin/main: without
// it git makes origin/main the new branch's upstream, so the branch reads as
// ahead of main rather than as never pushed.
func (r Repository) CreateBranch(ctx context.Context, name, start string) error {
	args := []string{"-C", r.dir, "switch", "--create", name}
	if start != "" {
		args = append(args, "--no-track", start)
	}

	_, err := r.run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("creating branch %s: %w", name, err)
	}

	return nil
}

// LocalBranches lists the repository's local branches, most recently committed
// to first, so a switcher offers the ones most likely to be picked up again.
func (r Repository) LocalBranches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir,
		"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	var branches []string

	for name := range strings.SplitSeq(text(out), "\n") {
		if name != "" && sanitize.Line(name) == name {
			branches = append(branches, name)
		}
	}

	return branches, nil
}

// RemoteBranches lists the branches that exist on the remotes, by name and most
// recently committed to first, so the base field can complete to one. The remote
// prefix is dropped ("origin/main" becomes "main") to match the name a base
// carries, a remote's symbolic HEAD pointer is left out (git abbreviates it to
// the bare remote name, "origin", which has no branch part), and a branch on
// more than one remote is listed once.
func (r Repository) RemoteBranches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir,
		"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("listing remote branches: %w", err)
	}

	var branches []string

	seen := map[string]bool{}

	for ref := range strings.SplitSeq(text(out), "\n") {
		_, name, found := strings.Cut(ref, "/")
		if !found || name == "" || name == "HEAD" {
			continue
		}

		if seen[name] || sanitize.Line(name) != name {
			continue
		}

		seen[name] = true
		branches = append(branches, name)
	}

	return branches, nil
}

// Checkout switches to a branch. It carries nothing across: the interface
// refuses a dirty tree before calling this, leaving stashing to the person.
//
// The name can come straight from the web request body, so "--" ends git's
// option parsing: a name beginning with a dash is then read as a ref, never as
// an option such as --orphan.
func (r Repository) Checkout(ctx context.Context, name string) error {
	_, err := r.run(ctx, gitProgram, "-C", r.dir, "switch", "--", name)
	if err != nil {
		return fmt.Errorf("switching to %s: %w", name, err)
	}

	return nil
}

// FinishBranch finishes a merged branch: switch to base, fast-forward it through
// pull, and delete the branch. It is for after the branch's pull request has
// merged, so the local repository catches up and the branch is cleaned away. A
// step that fails stops the rest, so a base that did not catch up keeps the
// branch.
//
// The pull is the caller's to run — PullCommand is the command — because it
// reaches the network, and so streams without the Runner's quick-read bound.
//
// The delete is a force delete (-D) on purpose: a squash or rebase merge leaves
// the branch's own commits unreachable from base, so the safe -d would refuse a
// branch the forge has already merged. Its caller is what makes this safe: it
// offers the finish only for a merged branch with no unpushed commits (see
// Branch.HasUnpushedWork), so the commits -D discards are the ones origin and
// the merge already hold.
func (r Repository) FinishBranch(ctx context.Context, branch, base string, pull func() error) error {
	err := r.finishStep(ctx, "switch", base)
	if err != nil {
		return err
	}

	err = pull()
	if err != nil {
		return fmt.Errorf("git pull --ff-only: %w", err)
	}

	return r.finishStep(ctx, "branch", "-D", branch)
}

// finishStep runs one of the git commands a finish runs itself, naming it in
// its failure.
func (r Repository) finishStep(ctx context.Context, step ...string) error {
	_, err := r.run(ctx, gitProgram, append([]string{"-C", r.dir}, step...)...)
	if err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(step, " "), err)
	}

	return nil
}

// WorktreeAdd creates branch name in a new worktree beside the repository and
// returns where it put it, so two tasks can be open at once — one checkout per
// worktree — where switching branches in place cannot. It starts the branch from
// start, or from HEAD when start is empty.
func (r Repository) WorktreeAdd(ctx context.Context, name, start string) (string, error) {
	path := worktreePath(r.dir, name)

	args := []string{"-C", r.dir, "worktree", "add"}
	if start != "" {
		// --no-track for the reason CreateBranch uses it: a branch off
		// origin/main should read as never pushed, not as ahead of it.
		args = append(args, "--no-track")
	}

	args = append(args, "-b", name, path)
	if start != "" {
		args = append(args, start)
	}

	_, err := r.run(ctx, gitProgram, args...)
	if err != nil {
		return "", fmt.Errorf("creating a worktree for %s: %w", name, err)
	}

	return path, nil
}

// worktreePath is where a worktree for a branch lives: a sibling of the
// repository named for the branch, out of its working tree but beside it.
func worktreePath(dir, name string) string {
	return dir + "-" + strings.ReplaceAll(name, "/", "-")
}
