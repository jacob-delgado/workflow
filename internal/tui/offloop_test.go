// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A key, or an answer, is dealt with at once: what reads git, the store or the
// disk runs in a command, so a slow repository or a store another session has
// locked never freezes the screen. Each case holds those seams, and the screen
// is drawn without their answers.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldRecentSubjects holds git's recent commit subjects and the scope the store
// learned, which the commit composer suggests from.
func heldRecentSubjects(deps *tui.Deps, held *hold) {
	subjects, learned := deps.Git.RecentSubjects, deps.Store.LastScope
	deps.Git.RecentSubjects = func() ([]string, error) {
		held.wait()

		return subjects()
	}
	deps.Store.LastScope = func() (string, bool) {
		held.wait()

		return learned()
	}
}

// heldTemplates holds the repository's pull request templates and its remote
// branches, which the pull request composer starts from.
func heldTemplates(deps *tui.Deps, held *hold) {
	templates, remotes := deps.Forge.Templates, deps.Git.RemoteBranches
	deps.Forge.Templates = func() []forge.Template {
		held.wait()

		return templates()
	}
	deps.Git.RemoteBranches = func() ([]string, error) {
		held.wait()

		return remotes()
	}
}

// heldScopeRecord holds the store's record of the scope a commit used.
func heldScopeRecord(deps *tui.Deps, held *hold) {
	record := deps.Store.RecordScope
	deps.Store.RecordScope = func(scope string) {
		held.wait()
		record(scope)
	}
}

// heldIssueCache holds the store's cache of an issue list, read and written.
func heldIssueCache(deps *tui.Deps, held *hold) {
	read, write := deps.Store.CachedIssues, deps.Store.CacheIssues
	deps.Store.CachedIssues = func(jql string) ([]jira.Issue, bool) {
		held.wait()

		return read(jql)
	}
	deps.Store.CacheIssues = func(jql string, issues []jira.Issue) {
		held.wait()
		write(jql, issues)
	}
}

// learningWorld is a world whose store learned the config scope.
func learningWorld() *world {
	learning := newWorld()
	learning.learnedScope = "config"

	return learning
}

// templatedWorld is a world with no pull request opened yet and a template to
// open one from.
func templatedWorld() *world {
	templated := withoutPull()
	templated.templates = []forge.Template{{Name: "feature", Body: "## What changes\n"}}

	return templated
}

// viewsWorld is the two-view world.
func viewsWorld() *world {
	repo := twoViewRepo()
	repo.cfg = twoViewConfig()

	return repo
}

func TestAKeyIsAnsweredWhileTheReadsAndWritesItStartsAreOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world func() *world
		wrap  func(deps *tui.Deps, held *hold)
		keys  []string
		want  string
	}{
		"opening the commit composer": {
			world: newWorld, wrap: heldRecentSubjects, keys: []string{"3", "c"}, want: "┏━ Commit",
		},
		"opening the pull request composer": {
			world: templatedWorld, wrap: heldTemplates, keys: []string{"4", "n"}, want: "┏━ Open pull request",
		},
		"committing with a scope": {
			world: learningWorld, wrap: heldScopeRecord, keys: commitKeys("x"), want: "committed fix(config): x",
		},
		"reading the issues again": {
			world: newWorld, wrap: heldIssueCache, keys: []string{"r"}, want: issueKey + " In Progress",
		},
		"switching the issue view": {
			world: viewsWorld, wrap: heldIssueCache, keys: []string{"v"}, want: "Sprint board",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			held := newHold(t)
			started := heldLive(t, held, tt.world(), tt.wrap)
			held.armed.Store(true)

			// Act
			view := holding(t, held, started, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}
