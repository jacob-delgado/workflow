// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// ErrInvalidView reports a jira view missing its name or its query.
var ErrInvalidView = errors.New("invalid jira view")

// JiraView is a named issue list: a label and the JQL that fills it, so the
// interface can offer more than the one built-in "assigned to me" list — a
// sprint, a team filter, the unassigned pile.
type JiraView struct {
	Name string `json:"name"`
	JQL  string `json:"jql"`
}

// Issues is where the Issues list draws from beyond Jira.
type Issues struct {
	// Forge adds the issues assigned to you on the repository's own forge,
	// GitHub or GitLab, to the list beside Jira's: for a repository whose
	// project tracks its work there. It is set per repository, in that
	// repository's file, over the home file's default. With no Jira, the
	// forge's issues are the whole list whatever it says.
	Forge bool `json:"forge"`
}

// validateHeaders refuses two jira.headers that differ only in case: HTTP
// reads them as one header, and each request would send whichever the map
// happened to set last.
func (c Config) validateHeaders() error {
	if first, second, same := sameNamed(slices.Collect(maps.Keys(c.Jira.Headers)), http.CanonicalHeaderKey); same {
		return fmt.Errorf("%w: jira.headers %q and %q name the same header", ErrSameName, first, second)
	}

	return nil
}

// sameNamed finds two names that key gives one form, in sorted order so the
// refusal reads the same each time.
func sameNamed(names []string, key func(string) string) (string, string, bool) {
	slices.Sort(names)
	seen := make(map[string]string, len(names))

	for _, name := range names {
		if earlier, ok := seen[key(name)]; ok {
			return earlier, name, true
		}

		seen[key(name)] = name
	}

	return "", "", false
}

// validateViews refuses a view missing its name or its query, so a half-written
// view fails at load rather than showing an empty pane with no way to tell why.
func (c Config) validateViews() error {
	for _, view := range c.Jira.Views {
		if strings.TrimSpace(view.Name) == "" || strings.TrimSpace(view.JQL) == "" {
			return fmt.Errorf("%w: each view needs a name and a jql", ErrInvalidView)
		}
	}

	return nil
}
