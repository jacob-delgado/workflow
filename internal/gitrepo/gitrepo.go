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
)

// ErrNotARepository reports a directory outside any git work tree.
var ErrNotARepository = errors.New("not a git repository")

// Runner runs a program and returns its standard output. proc.Run satisfies it;
// tests supply a function returning canned bytes.
//
// This is a function type rather than an interface because it has exactly one
// method's worth of surface, and a one-method seam is a func var.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

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
// ErrNotARepository, whatever git wrote; inside one it is git's own error, so a
// real fault still reads in full.
func readFailure(ctx context.Context, run Runner, dir, what string, err error) error {
	if !withinWorkTree(ctx, run, dir) {
		return fmt.Errorf("%w: %s", ErrNotARepository, dir)
	}

	return fmt.Errorf("%s: %w", what, err)
}

// withinWorkTree reports a directory inside a git work tree, by the same probe
// Describe uses to find the root.
func withinWorkTree(ctx context.Context, run Runner, dir string) bool {
	_, err := run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")

	return err == nil
}

// CheckIgnored reports whether git ignores path within dir. Outside a work tree
// it returns ErrNotARepository, because the question has no answer there — a
// caller warning about a file that is not ignored simply stays quiet.
func CheckIgnored(ctx context.Context, run Runner, dir, path string) (bool, error) {
	if !withinWorkTree(ctx, run, dir) {
		return false, fmt.Errorf("%w: %s", ErrNotARepository, dir)
	}

	// check-ignore exits zero when the path is ignored and non-zero when it is
	// not, so a non-zero exit here is the answer "no", not a failure to answer.
	_, err := run(ctx, "git", "-C", dir, "check-ignore", path)

	return err == nil, nil
}

// Describe reads the repository containing dir.
func Describe(ctx context.Context, run Runner, dir string) (Repo, error) {
	root, err := run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, fmt.Errorf("%w: %s", ErrNotARepository, dir)
	}

	// `branch --show-current` rather than `rev-parse --abbrev-ref HEAD`: it prints
	// the name of an UNBORN branch on a repository with no commits, where
	// rev-parse fails outright, and it prints nothing for a detached HEAD instead
	// of the literal string "HEAD" that a branch could legitimately be called.
	branch, err := run(ctx, "git", "-C", dir, "branch", "--show-current")
	if err != nil {
		return Repo{}, fmt.Errorf("reading the current branch of %s: %w", dir, err)
	}

	name := text(branch)

	return Repo{
		Root:     text(root),
		Branch:   name,
		Detached: name == "",
		Remote:   originURL(ctx, run, dir),
	}, nil
}

// originURL reads origin's URL. A repository without an origin is unusual but
// not broken — everything except opening a pull request still works — so the
// failure reads as "no remote" rather than becoming Describe's error.
func originURL(ctx context.Context, run Runner, dir string) string {
	out, err := run(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}

	return text(out)
}

// text trims the newline git ends every one-line answer with.
func text(out []byte) string {
	return strings.TrimSpace(string(out))
}
