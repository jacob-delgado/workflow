// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/store"
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
	deps.Tasks = heldBackTasks(deps.Tasks)
	deps.Store = keptReads(deps.Store)
	deps.Settings = heldBackSettings(deps.Settings)

	return heldBackServices(deps)
}

// heldBackSettings holds back saving the configuration and removing the local
// data; both are still read.
func heldBackSettings(deps seams.Settings) seams.Settings {
	if deps.Save != nil {
		deps.Save = func(config.Config, []config.Credential, config.Revision) (config.Config, config.Revision, error) {
			return config.Config{}, config.Revision{}, errDryRun
		}
	}

	if deps.RemoveLocalData != nil {
		deps.RemoveLocalData = func(store.CleanScope) error { return errDryRun }
	}

	deps.Setup = heldBackSetup(deps.Setup)

	return deps
}

// heldBackSetup holds back writing a first file, and offers no keychain,
// which would store the token; Jira's check is a read, and stays.
func heldBackSetup(deps seams.Setup) seams.Setup {
	if deps.Write != nil {
		deps.Write = func(setup.Request) (setup.Written, error) { return setup.Written{}, errDryRun }
	}

	if offer := deps.Offer; offer != nil {
		deps.Offer = func() setup.Offer {
			offered := offer()
			offered.Keychain = false

			return offered
		}
	}

	return deps
}

// keptReads is the store as a dry run uses it: the kept associations read,
// so the announcement preview says whom a post would tag, and the favorite
// directories, and nothing else.
// A dry-run interface opens no cache, as the web opens none: a read-only
// store's seams look like a live one's, whose cache reads make its file, so
// the cache is dropped whoever the caller is. The kept reads stay because
// under --dry-run the command line binds them to the read-only store, which
// reads a kept file as it is and never makes one.
func keptReads(store seams.Store) seams.Store {
	return seams.Store{
		OwnerLinks: store.OwnerLinks, RepoGroups: store.RepoGroups, LastGroups: store.LastGroups,
		Favorites: store.Favorites,
	}
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

	if deps.Discard != nil {
		deps.Discard = func(gitrepo.Change) error { return errDryRun }
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

	if deps.LinkIssue != nil {
		deps.LinkIssue = func(string, string) error { return errDryRun }
	}

	if deps.UnlinkIssue != nil {
		deps.UnlinkIssue = func(string) error { return errDryRun }
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

// heldBackTasks holds back the writes to Taskwarrior.
func heldBackTasks(deps seams.Tasks) seams.Tasks {
	if deps.Add != nil {
		deps.Add = func(string) (string, error) { return "", errDryRun }
	}

	if deps.Start != nil {
		deps.Start = func(string) error { return errDryRun }
	}

	if deps.Stop != nil {
		deps.Stop = func(string) error { return errDryRun }
	}

	if deps.Done != nil {
		deps.Done = func(string) error { return errDryRun }
	}

	if deps.Annotate != nil {
		deps.Annotate = func(string, string) error { return errDryRun }
	}

	if deps.Modify != nil {
		deps.Modify = func(string, string) error { return errDryRun }
	}

	if deps.Undo != nil {
		deps.Undo = func() (string, error) { return "", errDryRun }
	}

	if deps.Sync != nil {
		deps.Sync = func() (string, error) { return "", errDryRun }
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
