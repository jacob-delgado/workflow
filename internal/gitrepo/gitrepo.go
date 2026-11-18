// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package gitrepo reads and changes the git repository a session is running in.
//
// Every git invocation goes through a Runner the caller supplies, which is what
// lets the parsing here be tested against fixture bytes rather than against a
// repository someone has to build first.
package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
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
	// Origin is origin's URL, or empty when the repository has no origin.
	Origin string
	// Base is the base remote: where pull requests merge into, as BaseRemote
	// reads it — origin itself outside a fork's clone.
	Base Remote
	// Operation is the git operation stopped part way in the work tree, if any.
	Operation Operation
	// Rebasing is the branch a stopped rebase replays: empty when it replays a
	// detached HEAD, and for any other operation.
	Rebasing string
}

// Operation is a git operation stopped part way in the work tree, which git
// finishes with --continue or ends with --abort.
type Operation int

// The operations git can leave stopped part way; OperationNone when none is.
const (
	OperationNone Operation = iota
	OperationRebase
	OperationMerge
	OperationCherryPick
	OperationRevert
)

// Command is the git command that continues or aborts the operation:
// "rebase", "merge", "cherry-pick" or "revert"; OperationNone has none, "".
// A map, not a switch, so there is no last arm gobco can never see false.
func (o Operation) Command() string {
	return map[Operation]string{
		OperationNone:       "",
		OperationRebase:     "rebase",
		OperationMerge:      "merge",
		OperationCherryPick: "cherry-pick",
		OperationRevert:     "revert",
	}[o]
}

// Remote is a git remote by name and URL; the URL is empty when git holds
// none for that name.
type Remote struct {
	Name, URL string
}

// readFailure names a git read that failed: outside a work tree it is
// ErrNotARepository, whatever git wrote, and without git it is the missing
// program; inside one it is git's own error, so a real fault still reads in
// full. A read that timed out, or that its asker stopped waiting on, says so
// as it is, since asking git where the work tree is would only wait out the
// same bound again, or be stopped the same way.
func readFailure(ctx context.Context, run Runner, dir, what string, err error) error {
	if errors.Is(err, proc.ErrTimedOut) || ctx.Err() != nil {
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
	_, err := run(ctx, gitProgram, "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return notInWorkTree(dir, err)
	}

	return nil
}

// notInWorkTree is what a failed `rev-parse --show-toplevel` in dir says: that
// it timed out, that its asker stopped waiting on it, or that git is not on
// PATH, when any is so, since whether dir is a repository is then unknown;
// ErrNotARepository otherwise, whatever git wrote. A missing git is the
// runner's own error, unchanged, so it reads and exits as any other missing
// program does.
func notInWorkTree(dir string, err error) error {
	switch {
	case errors.Is(err, proc.ErrTimedOut), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
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
	_, err = r.run(ctx, gitProgram, "-C", r.dir, "check-ignore", path)

	return err == nil, nil
}

// Describe reads the repository this one works in.
func (r Repository) Describe(ctx context.Context) (Repo, error) {
	root, err := r.run(ctx, gitProgram, "-C", r.dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, notInWorkTree(r.dir, err)
	}

	// `branch --show-current` rather than `rev-parse --abbrev-ref HEAD`: it prints
	// the name of an UNBORN branch on a repository with no commits, where
	// rev-parse fails outright, and it prints nothing for a detached HEAD instead
	// of the literal string "HEAD" that a branch could legitimately be called.
	branch, err := r.run(ctx, gitProgram, "-C", r.dir, "branch", "--show-current")
	if err != nil {
		return Repo{}, fmt.Errorf("reading the current branch of %s: %w", r.dir, err)
	}

	name := text(branch)
	origin := remoteURL(ctx, r.run, r.dir, DefaultRemote)

	base, baseURL := r.BaseRemote(ctx), origin
	if base != DefaultRemote {
		baseURL = remoteURL(ctx, r.run, r.dir, base)
	}

	operation, rebasing := r.stopped(ctx)

	return Repo{
		Root:      text(root),
		Branch:    name,
		Detached:  name == "",
		Origin:    origin,
		Base:      Remote{Name: base, URL: baseURL},
		Operation: operation,
		Rebasing:  rebasing,
	}, nil
}

// SharedDir is the git directory every worktree of the repository shares,
// which tells one repository from another where each worktree's root differs.
func (r Repository) SharedDir(ctx context.Context) (string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", notInWorkTree(r.dir, err)
	}

	return text(out), nil
}

// RemoteURL is the URL of the remote named name, or empty when the repository
// has no such remote.
func (r Repository) RemoteURL(ctx context.Context, name string) string {
	return remoteURL(ctx, r.run, r.dir, name)
}

// stopMarker is a file git leaves while an operation is stopped part way.
type stopMarker struct {
	path      string
	operation Operation
}

// stopMarkers are the files that say an operation is stopped, in the order
// they decide it: a rebase's head-name under either backend, then a merge's,
// a cherry-pick's and a revert's head. `git am` uses rebase-apply too but
// writes no head-name there, so a stopped am reads as none.
func stopMarkers() []stopMarker {
	return []stopMarker{
		{path: "rebase-merge/head-name", operation: OperationRebase},
		{path: "rebase-apply/head-name", operation: OperationRebase},
		{path: "MERGE_HEAD", operation: OperationMerge},
		{path: "CHERRY_PICK_HEAD", operation: OperationCherryPick},
		{path: "REVERT_HEAD", operation: OperationRevert},
	}
}

// stopped reads which operation is stopped part way and, for a rebase, the
// branch it replays. git is asked where its markers are, since a linked
// worktree keeps its own apart from the shared directory, and the first that
// exists decides. A read that fails reads as none, as every optional read
// here does: the branch still reads, only without the warning.
func (r Repository) stopped(ctx context.Context) (Operation, string) {
	markers := stopMarkers()

	args := []string{"-C", r.dir, "rev-parse", "--path-format=absolute"}
	for _, marker := range markers {
		args = slices.Concat(args, []string{"--git-path", marker.path})
	}

	paths := strings.Split(optional(ctx, r.run, args...), "\n")
	if len(paths) != len(markers) {
		return OperationNone, ""
	}

	for index, marker := range markers {
		content, err := os.ReadFile(paths[index])
		if err != nil {
			continue
		}

		if marker.operation != OperationRebase {
			return marker.operation, ""
		}

		return OperationRebase, rebasedBranch(content)
	}

	return OperationNone, ""
}

// rebasedBranch is the branch a rebase's head-name names. git writes
// "detached HEAD" there for a rebase of no branch, which names none. The name
// is only ever shown, never handed to git, so it is cleaned, not refused.
func rebasedBranch(headName []byte) string {
	name, onBranch := strings.CutPrefix(text(headName), "refs/heads/")
	if !onBranch {
		return ""
	}

	return sanitize.Line(name)
}

// remoteURL reads the URL of the remote named name. A repository without that
// remote is unusual but not broken — everything except opening a pull request
// still works — so the failure reads as "no remote" rather than as an error.
func remoteURL(ctx context.Context, run Runner, dir, name string) string {
	out, err := run(ctx, gitProgram, "-C", dir, "remote", "get-url", name)
	if err != nil {
		return ""
	}

	return text(out)
}

// text trims the newline git ends every one-line answer with.
func text(out []byte) string {
	return strings.TrimSpace(string(out))
}
