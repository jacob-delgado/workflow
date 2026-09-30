// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// defaultViewName titles the built-in list used when a configuration names no
// views of its own.
const defaultViewName = "Assigned to me"

// issueView is a named issue list and the JQL that fills it.
type issueView struct {
	name string
	jql  string
}

// issueViews are the views a configuration offers, each scoped to your issues
// unless its query names the assignee, or the one built-in list — open issues
// assigned to you — when it names none. The scoped query is the view's own, so
// the issue cache is keyed by what was searched.
func issueViews(configured []config.JiraView) []issueView {
	if len(configured) == 0 {
		return []issueView{{name: defaultViewName, jql: jira.AssignedToMe}}
	}

	views := make([]issueView, 0, len(configured))
	for _, view := range configured {
		views = append(views, issueView{name: view.Name, jql: jira.ScopedToMe(view.JQL)})
	}

	return views
}

// activeView is the view the Issues pane is showing.
func (m Model) activeView() issueView {
	return m.views[m.viewIndex]
}

// nextIssueView moves to the next configured view and loads it from scratch,
// dropping the old view's filter, places and selection but keeping what the
// issues' marks are read from. With one view there is nowhere
// to go.
func (m Model) nextIssueView() (Model, tea.Cmd) {
	if len(m.views) <= 1 {
		return m, nil
	}

	m.viewIndex = (m.viewIndex + 1) % len(m.views)

	previous := m.issues
	m.issues = m.seededIssues()
	m.issues.branchKeys, m.issues.branchesKnown = previous.branchKeys, previous.branchesKnown
	m.issues.taskWords = previous.taskWords

	if !m.issues.settled {
		m.issues.loading = true
	}

	return m, m.relistIssues()
}

// viewSuffix names the active view beside the Issues pane's title, but only when
// there is more than one view — with a single list the name is just noise.
func (m Model) viewSuffix(p pane) string {
	if p != paneIssues || len(m.views) <= 1 {
		return ""
	}

	return m.marks.separator + m.activeView().name
}
