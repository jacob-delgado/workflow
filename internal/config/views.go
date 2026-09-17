// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
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
