// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package seams_test

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
)

// bundles are the seam bundles every surface takes, by name.
func bundles() map[string]reflect.Type {
	return map[string]reflect.Type{
		"Jira":         reflect.TypeFor[seams.Jira](),
		"Git":          reflect.TypeFor[seams.Git](),
		"Forge":        reflect.TypeFor[seams.Forge](),
		"Messaging":    reflect.TypeFor[seams.Messaging](),
		"Store":        reflect.TypeFor[seams.Store](),
		"Settings":     reflect.TypeFor[seams.Settings](),
		"Setup":        reflect.TypeFor[seams.Setup](),
		"Repositories": reflect.TypeFor[seams.Repositories](),
		"Hooks":        reflect.TypeFor[seams.Hooks](),
		"Tasks":        reflect.TypeFor[seams.Tasks](),
	}
}

// withoutContext are the seams that take no context, each with why: none asks
// anything a caller could stop waiting on.
func withoutContext() map[string]string {
	return map[string]string{
		"Jira.BrowseURL":              "builds a link for someone to click",
		"Forge.Templates":             "reads the work tree's files through os.Root, which takes no context",
		"Messaging.RefreshDirectory":  "drops what the directory holds in memory",
		"Settings.Read":               "reads local files through config.LoadLayersAt, which takes no context",
		"Settings.RemoveLocalData":    "renames and removes files, a clean that must finish once begun",
		"Setup.Offer":                 "says where the file may go, asking nothing",
		"Repositories.Subdirectories": "lists a directory through os.ReadDir, which takes no context",
	}
}

// notYet are the seams that do I/O but take no context yet. Each group's
// conversion deletes its own; the list ends empty.
func notYet() []string {
	return []string{
		"Jira.Search", "Jira.SearchLenient", "Jira.Issue", "Jira.Transitions", "Jira.Transition", "Jira.Comment",
		"Jira.Assign", "Jira.AddWorklog", "Jira.LinkPullRequest", "Jira.Activity",

		"Git.Branch", "Git.Changes", "Git.CommitsBetween", "Git.Diff", "Git.Stage", "Git.Unstage", "Git.Discard",
		"Git.CreateBranch", "Git.Branches", "Git.Checkout", "Git.RemoteBranches", "Git.ChangedPaths",
		"Git.CodeOwnersAt", "Git.RecentSubjects", "Git.CreateWorktree", "Git.Fetch", "Git.Commit", "Git.Push",
		"Git.Amend", "Git.Fixup", "Git.Rebase", "Git.Finish", "Git.IssueLinks", "Git.LinkIssue", "Git.UnlinkIssue",

		"Forge.FindPullRequest", "Forge.CreatePullRequest", "Forge.EditPullRequest", "Forge.RewriteDescription",
		"Forge.CheckStatus", "Forge.Rerun", "Forge.JobLog", "Forge.Merge", "Forge.MergeMethods",
		"Forge.ReviewRequests", "Forge.Author", "Forge.IsGroup", "Forge.Activity",

		"Store.LastScope", "Store.RecordScope", "Store.Announced", "Store.RecordAnnounce", "Store.CachedIssues",
		"Store.CacheIssues", "Store.OwnerLinks", "Store.LinkOwner", "Store.ForgetOwner", "Store.RepoGroups",
		"Store.SetRepoGroups", "Store.LastGroups", "Store.RecordGroups", "Store.Favorites", "Store.Favor",
		"Store.Unfavor",

		"Settings.Save", "Settings.LocalData",

		"Setup.Check", "Setup.Write",

		"Repositories.Look", "Repositories.Worktrees",

		"Hooks.Run", "Hooks.Existing", "Hooks.Write",
	}
}

// seamFields is every function field of every bundle, by its bundle and
// field name, with whether it takes a context first.
func seamFields() map[string]bool {
	contextType := reflect.TypeFor[context.Context]()
	fields := map[string]bool{}

	for bundle, kind := range bundles() {
		for field := range kind.Fields() {
			if field.Type.Kind() != reflect.Func {
				continue
			}

			takes := field.Type.NumIn() > 0 && field.Type.In(0) == contextType
			fields[bundle+"."+field.Name] = takes
		}
	}

	return fields
}

// A seam that does I/O is asked under the context of whatever asked it — a web
// request's, the command's, the terminal session's — so it can stop when its
// asker has gone, and the wiring never holds a context of its own.
func TestEverySeamThatDoesIOTakesAContextFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	exempt, pending := withoutContext(), notYet()

	// Act
	var missing []string

	for name, takes := range seamFields() {
		_, isExempt := exempt[name]
		if !takes && !isExempt && !slices.Contains(pending, name) {
			missing = append(missing, name)
		}
	}

	// Assert
	if len(missing) > 0 {
		slices.Sort(missing)
		t.Errorf("seams taking no context first: %s; take ctx context.Context first, or name the seam and why "+
			"in withoutContext", strings.Join(missing, ", "))
	}
}

// An entry naming no seam, or one that takes a context already, is stale: a
// list that only grows would excuse a seam it no longer needs to.
func TestEverySeamListedAsTakingNoContextTakesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := slices.Concat(notYet(), slices.Collect(maps.Keys(withoutContext())))
	fields := seamFields()

	// Act
	var stale []string

	for _, name := range listed {
		takes, isField := fields[name]
		if !isField || takes {
			stale = append(stale, name)
		}
	}

	// Assert
	if len(stale) > 0 {
		slices.Sort(stale)
		t.Errorf("listed as taking no context, but no such seam or it takes one: %s; delete them",
			strings.Join(stale, ", "))
	}
}
