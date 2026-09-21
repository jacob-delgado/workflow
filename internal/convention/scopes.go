// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import (
	"regexp"
	"strings"
)

// scopeInSubject captures the scope of a Conventional Commit subject:
// "feat(tui): add a pane" and "feat(api)!: drop a field" both yield the
// parenthesized scope. A subject with no scope does not match.
func scopeInSubject() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z]+\(([^)]+)\)!?:`)
}

// Scopes reads the Conventional Commit scopes out of commit subjects, keeping
// each well-formed scope once in the order it first appears, with surrounding
// whitespace trimmed. A subject with no scope, or a scope holding characters a
// scope may not, contributes nothing, so the result is safe to offer as
// completions.
func Scopes(subjects []string) []string {
	var scopes []string

	seen := map[string]bool{}

	for _, subject := range subjects {
		match := scopeInSubject().FindStringSubmatch(subject)
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
