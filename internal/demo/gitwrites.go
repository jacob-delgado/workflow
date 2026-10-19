// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// What the demo's git refuses, as git itself does.
var (
	errBranchExists  = errors.New("a branch of that name already exists")
	errNothingStaged = errors.New("nothing added to commit")
	errNoSuchBranch  = errors.New("src refspec does not match any branch")
	errNoSuchChange  = errors.New("pathspec did not match any changed file")
)

// firstHash is the hash the demo's first commit takes; each one after takes
// the next of hashStep's steps, so each is its own and the same every run.
const (
	firstHash = 0x4f2c9e1
	hashStep  = 0x1d3b7
)

// finished is a run that has already ended: every line it printed, and how
// it ended. Hooks run inside a commit and a push, so both stream their
// output; the demo's arrives at once.
func finished(lines []string, err error) proc.Output {
	stream := make(chan string, len(lines))
	for _, line := range lines {
		stream <- line
	}

	close(stream)

	return proc.Output{Lines: stream, Wait: func() error { return err }, Stop: func() {}}
}

// remoteBranches is the branches origin holds, main and each branch pushed,
// named without the remote, as gitrepo.RemoteBranches names them.
func (w *World) remoteBranches(context.Context) ([]string, error) {
	return locked(w, func() []string {
		var names []string

		for _, name := range w.repo.order {
			if w.repo.branches[name].upstream != "" {
				names = append(names, name)
			}
		}

		return names
	}), nil
}

// createBranch creates a branch where main stands and switches to it,
// carrying the work tree's changes with it, as git switch -c does. It joins
// the branches still at main's commit, by name, after those committed to.
func (w *World) createBranch(_ context.Context, name, _ string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.repo.branches[name]; exists {
		return fmt.Errorf("%s: %w", name, errBranchExists)
	}

	place, _ := slices.BinarySearchFunc(w.repo.order, name, func(listed, target string) int {
		if len(w.repo.branches[listed].commits) > 0 {
			return -1
		}

		return strings.Compare(listed, target)
	})
	w.repo.order = slices.Insert(w.repo.order, place, name)
	w.repo.branches[name] = localBranch{}
	w.repo.current = name

	return nil
}

// stage moves a change into the index.
func (w *World) stage(_ context.Context, change gitrepo.Change) error {
	return w.setStaged(change.Path, true)
}

// unstage moves a change back out of the index.
func (w *World) unstage(_ context.Context, change gitrepo.Change) error {
	return w.setStaged(change.Path, false)
}

// setStaged stages or unstages the change at path, refusing a path the work
// tree has no change at, as git add does.
func (w *World) setStaged(path string, staged bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	at := slices.IndexFunc(w.repo.changes, func(change fileChange) bool { return change.as.Path == path })
	if at < 0 {
		return fmt.Errorf("%s: %w", path, errNoSuchChange)
	}

	w.repo.changes[at].staged = staged

	return nil
}

// commit records what is staged as a commit on the branch checked out, and
// leaves what is not staged in the work tree. With nothing staged it fails,
// as git does.
func (w *World) commit(_ context.Context, message string) (proc.Output, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	committing := slices.ContainsFunc(w.repo.changes, func(change fileChange) bool { return change.staged })
	if !committing {
		return finished([]string{"no changes added to commit"}, errNothingStaged), nil
	}

	subject, _, _ := strings.Cut(message, "\n")
	hash := fmt.Sprintf("%07x", firstHash+hashStep*len(w.repo.written))
	local := w.repo.branches[w.repo.current]
	local.commits = append(local.commits, gitrepo.Commit{Hash: hash, Subject: subject})
	w.repo.branches[w.repo.current] = local
	// git lists the branch just committed to first.
	others := slices.DeleteFunc(w.repo.order, func(listed string) bool { return listed == w.repo.current })
	w.repo.order = slices.Insert(others, 0, w.repo.current)
	w.repo.changes = slices.DeleteFunc(w.repo.changes, func(change fileChange) bool { return change.staged })
	w.repo.written = append(w.repo.written,
		gitrepo.DatedCommit{Hash: hash, Short: hash, Subject: subject, Authored: w.now()})

	return finished([]string{
		"lefthook  pre-commit: gofmt, golangci-lint, typos",
		"✔ gofmt", "✔ golangci-lint", "✔ typos",
		fmt.Sprintf("[%s %s] %s", w.repo.current, hash, subject),
	}, nil), nil
}

// push sends a branch's commits to origin and sets its upstream there.
func (w *World) push(_ context.Context, name string) (proc.Output, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	local, exists := w.repo.branches[name]
	if !exists {
		return finished([]string{"error: src refspec " + name + " does not match any"}, errNoSuchBranch), nil
	}

	since := headOf(localBranch{commits: local.commits[:local.pushed]})
	sent := "   " + since + ".." + headOf(local) + "  " + name + " -> " + name

	if local.upstream == "" {
		sent = " * [new branch]      " + name + " -> " + name
	}

	local.upstream, local.pushed = remote+"/"+name, len(local.commits)
	w.repo.branches[name] = local

	return finished([]string{
		"lefthook  pre-push: test", "✔ test", "To " + githubHost + ":" + forgeRepository + ".git", sent,
	}, nil), nil
}

// headOf is the commit a branch stands at: its last, or main's head before
// it has one of its own.
func headOf(local localBranch) string {
	if len(local.commits) == 0 {
		return mainHead
	}

	return local.commits[len(local.commits)-1].Hash
}
