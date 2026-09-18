// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// gitDeps fakes the repository.
func (w *world) gitDeps() tui.GitDeps {
	return tui.GitDeps{
		Branch: func() (gitrepo.Branch, error) {
			w.record("branch")

			return w.branch, nil
		},
		Changes: func() ([]gitrepo.Change, error) {
			w.record("changes")

			return slices.Clone(w.changes), nil
		},
		Stage: func(change gitrepo.Change) error {
			w.record("stage " + change.Path)

			return w.stageErr
		},
		Unstage: func(change gitrepo.Change) error {
			w.record("unstage " + change.Path)

			return w.stageErr
		},
		CreateBranch: func(name, start string) error {
			w.record("create " + name + " from " + start)

			return w.createErr
		},
		CreateWorktree: func(name, start string) (string, error) {
			w.record("worktree " + name + " from " + start)

			return "/work-" + name, w.worktreeErr
		},
		Branches: func() ([]string, error) {
			w.record("branches")

			return slices.Clone(w.branches), w.branchesErr
		},
		Checkout: func(name string) error {
			w.record("checkout " + name)

			return w.checkoutErr
		},
		Fetch: func() error {
			w.record("fetch")

			return w.fetchErr
		},
		Commit: func(message string) (proc.Output, error) {
			w.record("commit " + message)

			if w.commitStartErr != nil {
				return proc.Output{}, w.commitStartErr
			}

			return output(w.commitLines, w.commitErr), nil
		},
		Push: func(branch string) (proc.Output, error) {
			w.record("push " + branch)

			return output(w.pushLines, w.pushErr), nil
		},
		Rebase: func(base string) (proc.Output, error) {
			w.record("rebase " + base)

			return output(w.rebaseLines, w.rebaseErr), nil
		},
	}
}
