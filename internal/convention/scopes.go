// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import (
	"path"
	"regexp"
	"strings"
)

// scopeInSubject captures the scope of a Conventional Commit subject:
// "feat(tui): add a pane" and "feat(api)!: drop a field" both yield the
// parenthesized scope. A subject with no scope does not match.
var scopeInSubject = regexp.MustCompile(`^` + commitTypePattern + `\(([^)]+)\)!?:`)

// Scopes reads the Conventional Commit scopes out of commit subjects, keeping
// each well-formed scope once in the order it first appears, with surrounding
// whitespace trimmed. A subject with no scope, or a scope holding characters a
// scope may not, contributes nothing, so the result is safe to offer as
// completions.
func Scopes(subjects []string) []string {
	var scopes []string

	seen := map[string]bool{}

	for _, subject := range subjects {
		match := scopeInSubject.FindStringSubmatch(subject)
		if match == nil {
			continue
		}

		value := strings.TrimSpace(match[1])
		if value == "" || seen[value] || ValidateScope(value) != nil {
			continue
		}

		seen[value] = true
		scopes = append(scopes, value)
	}

	return scopes
}

// ScopeSuggestions is the scope completions for a commit: the shared directory
// of the staged files at paths first, then the scopes already used in
// subjects, recent commits' subjects, each once.
func ScopeSuggestions(paths, subjects []string) []string {
	var suggestions []string

	seen := map[string]bool{}
	add := func(value string) {
		if value != "" && !seen[value] {
			seen[value] = true
			suggestions = append(suggestions, value)
		}
	}

	add(sharedScope(paths))

	for _, scope := range Scopes(subjects) {
		add(scope)
	}

	return suggestions
}

// sharedScope is the name of the deepest directory the staged files share, when
// that name is a well-formed scope — "config" for files under internal/config.
// Empty when they share no directory or the name is not a usable scope.
func sharedScope(paths []string) string {
	// Files that share no directory share "", "." or "/", whose base is "." or
	// "/": names refused like any other that is not a scope. The name is held
	// to the scope's shape as it is, not as ValidateScope trims it, which takes
	// a name of spaces alone for an empty scope, and an empty scope as none.
	base := path.Base(sharedDir(paths))
	if !wellFormedScope.MatchString(base) {
		return ""
	}

	return base
}

// sharedDir is the deepest directory every path lies under: "." or "/" when
// every file lies at the root of a relative or an absolute tree, and empty
// when nothing is staged or the files share no first directory.
func sharedDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	shared := path.Dir(paths[0])
	for _, each := range paths[1:] {
		shared = commonDir(shared, path.Dir(each))
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
