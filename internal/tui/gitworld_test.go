// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// gitDeps fakes the repository.
func (w *world) gitDeps() seams.Git {
	deps := seams.Git{
		Branch: func() (gitrepo.Branch, error) {
			w.record("branch")

			return w.branch, nil
		},
		Changes: func() ([]gitrepo.Change, error) {
			w.record("changes")

			return slices.Clone(w.changes), w.changesErr
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
		Discard: func(change gitrepo.Change) error {
			w.record("discard " + change.Path)

			return w.discardErr
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
		ChangedPaths: func(base string) ([]string, error) {
			w.record("changed-paths " + base)

			return []string{"README.md"}, w.changedPathsErr
		},
		CodeOwnersAt: func(base string) (codeowners.File, bool, error) {
			w.record("code-owners " + base)

			// Every path is owned by every owner the world names.
			content := "* @" + strings.Join(w.codeOwners, " @")

			return codeowners.Parse(content, codeowners.GitHub), len(w.codeOwners) > 0, w.codeOwnersErr
		},
		RecentSubjects: func() ([]string, error) {
			w.record("recent-subjects")

			return slices.Clone(w.recentSubjects), w.recentSubjectsErr
		},
		Checkout: func(name string) error {
			w.record("checkout " + name)

			return w.checkoutErr
		},
		LinkIssue: func(branch, issueKey string) error {
			w.record("link-issue " + branch + " " + issueKey)
			w.branch.IssueLink = issueKey

			return nil
		},
		UnlinkIssue: func(branch string) error {
			w.record("unlink-issue " + branch)

			if w.unlinkErr != nil {
				return w.unlinkErr
			}

			w.branch.IssueLink = ""

			return nil
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
		deps.ChangedPaths, deps.CodeOwnersAt = nil, nil
	}

	if w.noRecentSubjects {
		deps.RecentSubjects = nil
	}

	if w.noChanges {
		deps.Changes = nil
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
