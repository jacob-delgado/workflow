// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// errDryRun is what a write seam returns once dry run has held it back, so a
// call that forgot its own dry-run guard fails safe — it reaches no service and
// says why — rather than writing for real.
var errDryRun = errors.New("held back by dry run")

// heldBack replaces every write seam in deps with one that writes nothing and
// returns errDryRun, so holding writes back is a property of the seams and not
// only of the scattered guards at each call site. A nil seam stays nil: nil
// means the feature is unavailable, which dry run must not turn on.
func heldBack(deps Deps) Deps {
	deps.Jira = heldBackJira(deps.Jira)
	deps.Git = heldBackGit(deps.Git)
	// A dry-run interface opens no store, as the web opens none. A read-only
	// store's seams look like a live one's, whose reads make its directory, so
	// the store is dropped whoever the caller is.
	deps.Store = seams.Store{}

	return heldBackServices(deps)
}

// heldBackJira holds back the writes to Jira.
func heldBackJira(deps seams.Jira) seams.Jira {
	if deps.Transition != nil {
		deps.Transition = func(jira.Key, jira.Transition, []jira.FieldValue) error { return errDryRun }
	}

	if deps.Comment != nil {
		deps.Comment = func(jira.Key, string) (jira.Comment, error) { return jira.Comment{}, errDryRun }
	}

	if deps.Assign != nil {
		deps.Assign = func(jira.Key, string) error { return errDryRun }
	}

	if deps.AddWorklog != nil {
		deps.AddWorklog = func(jira.Key, string, string) (jira.Worklog, error) { return jira.Worklog{}, errDryRun }
	}

	if deps.LinkPullRequest != nil {
		deps.LinkPullRequest = func(jira.Key, string, string) error { return errDryRun }
	}

	return deps
}

// heldBackGit holds back the writes to the repository.
func heldBackGit(deps seams.Git) seams.Git {
	if deps.Stage != nil {
		deps.Stage = func(gitrepo.Change) error { return errDryRun }
	}

	if deps.Unstage != nil {
		deps.Unstage = func(gitrepo.Change) error { return errDryRun }
	}

	if deps.CreateBranch != nil {
		deps.CreateBranch = func(string, string) error { return errDryRun }
	}

	if deps.Checkout != nil {
		deps.Checkout = func(string) error { return errDryRun }
	}

	if deps.CreateWorktree != nil {
		deps.CreateWorktree = func(string, string) (string, error) { return "", errDryRun }
	}

	if deps.Finish != nil {
		deps.Finish = func(string, string) error { return errDryRun }
	}

	return heldBackStreams(deps)
}

// heldBackStreams holds back the repository writes that stream their output.
func heldBackStreams(deps seams.Git) seams.Git {
	if deps.Commit != nil {
		deps.Commit = func(string) (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	if deps.Push != nil {
		deps.Push = func(string) (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	if deps.Amend != nil {
		deps.Amend = func() (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	if deps.Fixup != nil {
		deps.Fixup = func(string) (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	if deps.Rebase != nil {
		deps.Rebase = func(string) (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	return deps
}

// heldBackServices holds back the writes to the forge, the messaging service and
// lefthook.
func heldBackServices(deps Deps) Deps {
	if deps.Forge.CreatePullRequest != nil {
		deps.Forge.CreatePullRequest = func(forge.NewPullRequest) (forge.PullRequest, error) {
			return forge.PullRequest{}, errDryRun
		}
	}

	if deps.Forge.EditPullRequest != nil {
		deps.Forge.EditPullRequest = func(forge.PullRequest, forge.PullRequestEdit) (forge.PullRequest, error) {
			return forge.PullRequest{}, errDryRun
		}
	}

	if deps.Forge.Rerun != nil {
		deps.Forge.Rerun = func(forge.PullRequest, string) (bool, error) { return false, errDryRun }
	}

	if deps.Forge.Merge != nil {
		deps.Forge.Merge = func(forge.PullRequest, forge.MergeMethod) error { return errDryRun }
	}

	if deps.Messaging.Post != nil {
		deps.Messaging.Post = func(string, string) error { return errDryRun }
	}

	if deps.Hooks.Run != nil {
		deps.Hooks.Run = func(string) (proc.Output, error) { return proc.Output{}, errDryRun }
	}

	if deps.Hooks.Write != nil {
		deps.Hooks.Write = func(hooks.Generated) error { return errDryRun }
	}

	return deps
}
