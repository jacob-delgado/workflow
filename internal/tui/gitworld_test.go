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
	deps := tui.GitDeps{
		Branch: func() (gitrepo.Branch, error) {
			w.record("branch")

			return w.branch, nil
		},
		Changes: func() ([]gitrepo.Change, error) {
			w.record("changes")

			return slices.Clone(w.changes), nil
		},
		Diff: func(change gitrepo.Change) ([]string, error) {
			w.record("diff " + change.Path)

			return slices.Clone(w.diff), w.diffErr
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
		RemoteBranches: func() ([]string, error) {
			w.record("remote-branches")

			return slices.Clone(w.remoteBranches), w.remoteBranchesErr
		},
		CodeOwners: func() ([]string, error) {
			w.record("code-owners")

			return slices.Clone(w.codeOwners), w.codeOwnersErr
		},
		RecentSubjects: func() ([]string, error) {
			w.record("recent-subjects")

			return slices.Clone(w.recentSubjects), w.recentSubjectsErr
		},
		Checkout: func(name string) error {
			w.record("checkout " + name)

			return w.checkoutErr
		},
		Finish: func(branch, base string) error {
			w.record("finish " + branch + " onto " + base)

			return w.finishErr
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
		Amend: func() (proc.Output, error) {
			w.record("amend")

			return output(w.amendLines, w.amendErr), nil
		},
		Fixup: func(hash string) (proc.Output, error) {
			w.record("fixup " + hash)

			return output(w.fixupLines, w.fixupErr), nil
		},
		Rebase: func(base string) (proc.Output, error) {
			w.record("rebase " + base)

			return output(w.rebaseLines, w.rebaseErr), nil
		},
	}

	// A repository that cannot be read leaves the seam nil, the way wiring does
	// outside a repository.
	if w.noRemoteBranches {
		deps.RemoteBranches = nil
	}

	if w.noCodeOwners {
		deps.CodeOwners = nil
	}

	if w.noRecentSubjects {
		deps.RecentSubjects = nil
	}

	if w.noDiff {
		deps.Diff = nil
	}

	if w.noAmend {
		deps.Amend = nil
	}

	if w.noFixup {
		deps.Fixup = nil
	}

	return deps
}
