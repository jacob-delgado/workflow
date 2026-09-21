// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// onFieldNav accepts a pending scope completion when tab could take one, and
// otherwise moves to the next field.
func (c commitComposer) onFieldNav(m Model, msg tea.KeyPressMsg) commitComposer {
	if c.focus == fieldScope && c.scopeCanComplete() {
		return c.typed(m, msg)
	}

	return c.focusOn((c.focus + 1) % composerFields)
}

// scopeCanComplete reports a scope suggestion that would extend what is typed, so
// tab completes it rather than moving on.
func (c commitComposer) scopeCanComplete() bool {
	suggestion := c.scope.CurrentSuggestion()

	return suggestion != "" && suggestion != c.scope.Value()
}

// withScopeSuggestions offers the shared directory of the staged files and the
// scopes already in use as completions for the scope field. With nothing to
// suggest the field is left plain.
func (c commitComposer) withScopeSuggestions(paths []string, recentSubjects func() ([]string, error)) commitComposer {
	suggestions := scopeSuggestions(paths, recentSubjects)
	if len(suggestions) == 0 {
		return c
	}

	c.scope.SetSuggestions(suggestions)
	c.scope.ShowSuggestions = true

	return c
}

// stagedPaths are the paths of the files staged for the next commit.
func (m Model) stagedPaths() []string {
	var paths []string

	for _, change := range m.changes.changes {
		if change.IsStaged() {
			paths = append(paths, change.Path)
		}
	}

	return paths
}

// scopeSuggestions is the scope completions for a commit: the shared directory of
// the staged files first, then the scopes already used in recent commits, each
// once. A missing or unreadable history contributes nothing.
func scopeSuggestions(paths []string, recentSubjects func() ([]string, error)) []string {
	var suggestions []string

	seen := map[string]bool{}
	add := func(value string) {
		if value != "" && !seen[value] {
			seen[value] = true
			suggestions = append(suggestions, value)
		}
	}

	add(sharedScope(paths))

	for _, scope := range recentScopes(recentSubjects) {
		add(scope)
	}

	return suggestions
}

// recentScopes reads the Conventional Commit scopes from recent commit subjects,
// or none when the repository cannot be read or has no such seam.
func recentScopes(recentSubjects func() ([]string, error)) []string {
	if recentSubjects == nil {
		return nil
	}

	subjects, err := recentSubjects()
	if err != nil {
		return nil
	}

	return convention.Scopes(subjects)
}

// sharedScope is the name of the deepest directory the staged files share, when
// that name is a well-formed scope — "config" for files under internal/config.
// Empty when they share no directory or the name is not a usable scope.
func sharedScope(paths []string) string {
	dir := sharedDir(paths)
	if dir == "" {
		return ""
	}

	base := path.Base(dir)
	if convention.ValidateScope(base) != nil {
		return ""
	}

	return base
}

// sharedDir is the deepest directory every path lies under, or empty when they
// share none — files at the repository root, or spread across separate trees.
func sharedDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	shared := path.Dir(paths[0])
	for _, each := range paths[1:] {
		shared = commonDir(shared, path.Dir(each))
	}

	if shared == "." || shared == "/" {
		return ""
	}

	return shared
}

// commonDir is the directory prefix two directories share, by path segment.
func commonDir(first, second string) string {
	firstParts, secondParts := strings.Split(first, "/"), strings.Split(second, "/")

	var shared []string

	for index := 0; index < len(firstParts) && index < len(secondParts); index++ {
		if firstParts[index] != secondParts[index] {
			break
		}

		shared = append(shared, firstParts[index])
	}

	return strings.Join(shared, "/")
}
