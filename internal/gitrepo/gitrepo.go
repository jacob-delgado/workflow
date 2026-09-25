// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package gitrepo reads the git repository a session is running in.
//
// Every git invocation goes through a Runner the caller supplies, which is what
// lets the parsing here be tested against fixture bytes rather than against a
// repository someone has to build first.
package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrNotARepository reports a directory outside any git work tree.
var ErrNotARepository = errors.New("not a git repository")

// Runner runs a program and returns its standard output. proc.Run satisfies it;
// tests supply a function returning canned bytes.
//
// This is a function type rather than an interface because it has exactly one
// method's worth of surface, and a one-method seam is a func var.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Repository is a git work tree a caller reads and changes: the directory it
// works in and the Runner every git command goes through, bundled so the two
// stop traveling as a pair through every function's signature.
type Repository struct {
	run Runner
	dir string
}

// At is a Repository working in dir, running git through run. For Status and
// Stage, dir must be the work-tree root — git reports and stages paths relative
// to it — so pass the Root that Describe found; for the questions that name
// their own path, such as CheckIgnored, dir is only where git is run from.
func At(run Runner, dir string) Repository {
	return Repository{run: run, dir: dir}
}

// Repo describes the repository a session is running in.
type Repo struct {
	// Root is the absolute path of the work tree.
	Root string
	// Branch is the checked-out branch, empty when Detached is true.
	Branch string
	// Detached reports a HEAD that is not on a branch. It is not an error — the
	// repository still reads — but it is why a caller cannot branch from here.
	Detached bool
	// Remote is origin's URL, or empty when the repository has no origin.
	Remote string
}

// readFailure names a git read that failed: outside a work tree it is
// ErrNotARepository, whatever git wrote, and without git it is the missing
// program; inside one it is git's own error, so a real fault still reads in
// full. A read that timed out says so as it is, since asking git where the
// work tree is would only wait out the same bound again.
func readFailure(ctx context.Context, run Runner, dir, what string, err error) error {
	if errors.Is(err, proc.ErrTimedOut) {
		return fmt.Errorf("%s: %w", what, err)
	}

	outside := requireWorkTree(ctx, run, dir)
	if outside != nil {
		return outside
	}

	return fmt.Errorf("%s: %w", what, err)
}

// requireWorkTree answers nil for a directory inside a git work tree, by the
// same probe Describe uses to find the root, and why not otherwise.
func requireWorkTree(ctx context.Context, run Runner, dir string) error {
	_, err := run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return notInWorkTree(dir, err)
	}

	return nil
}

// notInWorkTree is what a failed `rev-parse --show-toplevel` in dir says: that
// it timed out, or that git is not on PATH, when either is so, since whether dir
// is a repository is then unknown; ErrNotARepository otherwise, whatever git
// wrote. A missing git is the runner's own error, unchanged, so it reads and
// exits as any other missing program does.
func notInWorkTree(dir string, err error) error {
	switch {
	case errors.Is(err, proc.ErrTimedOut):
		return fmt.Errorf("finding the work tree of %s: %w", dir, err)
	case errors.Is(err, proc.ErrNotFound):
		return err
	default:
		return fmt.Errorf("%w: %s", ErrNotARepository, dir)
	}
}

// CheckIgnored reports whether git ignores path. Outside a work tree it returns
// ErrNotARepository, because the question has no answer there — a caller warning
// about a file that is not ignored simply stays quiet — and without git, the
// missing program.
func (r Repository) CheckIgnored(ctx context.Context, path string) (bool, error) {
	err := requireWorkTree(ctx, r.run, r.dir)
	if err != nil {
		return false, err
	}

	// check-ignore exits zero when the path is ignored and non-zero when it is
	// not, so a non-zero exit here is the answer "no", not a failure to answer.
	_, err = r.run(ctx, "git", "-C", r.dir, "check-ignore", path)

	return err == nil, nil
}

// Describe reads the repository this one works in.
func (r Repository) Describe(ctx context.Context) (Repo, error) {
	root, err := r.run(ctx, "git", "-C", r.dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, notInWorkTree(r.dir, err)
	}

	// `branch --show-current` rather than `rev-parse --abbrev-ref HEAD`: it prints
	// the name of an UNBORN branch on a repository with no commits, where
	// rev-parse fails outright, and it prints nothing for a detached HEAD instead
	// of the literal string "HEAD" that a branch could legitimately be called.
	branch, err := r.run(ctx, "git", "-C", r.dir, "branch", "--show-current")
	if err != nil {
		return Repo{}, fmt.Errorf("reading the current branch of %s: %w", r.dir, err)
	}

	name := text(branch)

	return Repo{
		Root:     text(root),
		Branch:   name,
		Detached: name == "",
		Remote:   originURL(ctx, r.run, r.dir),
	}, nil
}

// originURL reads origin's URL. A repository without an origin is unusual but
// not broken — everything except opening a pull request still works — so the
// failure reads as "no remote" rather than becoming Describe's error.
func originURL(ctx context.Context, run Runner, dir string) string {
	out, err := run(ctx, "git", "-C", dir, "remote", "get-url", DefaultRemote)
	if err != nil {
		return ""
	}

	return text(out)
}

// text trims the newline git ends every one-line answer with.
func text(out []byte) string {
	return strings.TrimSpace(string(out))
}
