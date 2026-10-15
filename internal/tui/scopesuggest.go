// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// withScopeSuggestions offers suggestions as completions for the scope field.
// With nothing to suggest the field is left plain.
func (c commitComposer) withScopeSuggestions(suggestions []string) commitComposer {
	if len(suggestions) == 0 {
		return c
	}

	c.scope.SetSuggestions(suggestions)
	c.scope.ShowSuggestions = true

	return c
}

// readScopes is the command that reads, for the composer opened as opened,
// the scope suggestions from paths and the log, and, when learn is set because
// no kept draft names a scope, the scope last used in this repository: git and
// the store are read off the update loop, so the composer never waits on them.
func readScopes(ctx context.Context, deps Deps, opened int, paths []string, learn bool) tea.Cmd {
	recentSubjects, lastScope := deps.Git.RecentSubjects, deps.Store.LastScope
	if !learn {
		lastScope = nil
	}

	if recentSubjects == nil && lastScope == nil {
		return nil
	}

	return func() tea.Msg {
		subjects := recentCommitSubjects(ctx, recentSubjects)

		read := scopesRead{opened: opened, suggestions: convention.ScopeSuggestions(paths, subjects)}
		if lastScope == nil {
			return read
		}

		if learned, ok := lastScope(ctx); ok {
			read.learned = learned
		}

		return read
	}
}

// scopesRead is what the commit composer opened as opened suggests for its
// scope, and the scope last used here, empty when none was learned.
type scopesRead struct {
	opened      int
	suggestions []string
	learned     string
}

var _ applier = scopesRead{}

// apply offers the suggestions, and puts the scope last used in the field while
// it still holds the configured default it opened on, unless the composer they
// were read for has closed — even if another has opened since.
func (read scopesRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(commitComposer)
	if !open || composer.opened != read.opened {
		return m, nil
	}

	if read.learned != "" && composer.scope.Value() == m.cfg.Commit.DefaultScope {
		composer.scope.SetValue(read.learned)
	}

	m.overlay = composer.withScopeSuggestions(read.suggestions)

	return m, nil
}

// stagedPaths are the paths of the files staged for the next commit.
func (l changeList) stagedPaths() []string {
	var paths []string

	for _, change := range l.changes {
		if change.IsStaged() {
			paths = append(paths, change.Path)
		}
	}

	return paths
}

// recentCommitSubjects reads the subjects of recent commits, or none when the
// repository cannot be read or has no such seam: a missing or unreadable
// history contributes nothing to the scope's completions.
func recentCommitSubjects(ctx context.Context, recentSubjects func(context.Context) ([]string, error)) []string {
	if recentSubjects == nil {
		return nil
	}

	subjects, err := recentSubjects(ctx)
	if err != nil {
		return nil
	}

	return subjects
}
