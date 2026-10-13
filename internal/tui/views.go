// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// activeView is the view the Issues pane is showing.
func (m Model) activeView() jira.View {
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

	m.viewIndex = around(m.viewIndex, len(m.views)).next()

	previous := m.issues
	m.issues = issueList{
		branchKeys: previous.branchKeys, branchesKnown: previous.branchesKnown, taskWords: previous.taskWords,
	}

	return m.searching(), tea.Batch(m.readCachedIssues(), m.relistIssues())
}

// viewSuffix names the active view beside the Issues pane's title, but only when
// there is more than one view — with a single list the name is just noise.
func (m Model) viewSuffix(p pane) string {
	if p != paneIssues || len(m.views) <= 1 {
		return ""
	}

	return m.marks.separator + sanitize.Line(m.activeView().Name)
}
