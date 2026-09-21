// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// sprintJQL is the second view's query in these tests.
const sprintJQL = "sprint in openSprints()"

// twoViewWorld is a world whose configuration names two views, the second
// filled with its own issue.
func twoViewWorld(t *testing.T) tui.Model {
	t.Helper()

	cfg := completeConfig()
	cfg.Jira.Views = []config.JiraView{
		{Name: "My work", JQL: "assignee = currentUser()"},
		{Name: "Sprint board", JQL: sprintJQL},
	}

	repo := newWorld()
	repo.viewIssues = map[string][]jira.Issue{
		sprintJQL: {issue("OPS-9", "Sprint task", "new")},
	}

	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)

	return drain(t, model, model.Init())
}

func TestTheFirstViewShowsAtStartAndNamesItself(t *testing.T) {
	t.Parallel()

	// Act
	view := twoViewWorld(t).View().Content

	// Assert
	requireScreen(t, view, "My work", issueKey)
}

func TestTheViewKeyMovesToTheNextView(t *testing.T) {
	t.Parallel()

	// Arrange
	model := twoViewWorld(t)

	// Act
	switched := typing(t, model, "v")

	// Assert
	requireScreen(t, switched.View().Content, "Sprint board", "OPS-9", "Sprint task")
}

func TestSwitchingViewsRunsTheOtherViewsQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Jira.Views = []config.JiraView{
		{Name: "My work", JQL: "assignee = currentUser()"},
		{Name: "Sprint board", JQL: sprintJQL},
	}

	repo := newWorld()
	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	typing(t, model, "v")

	// Assert
	if len(repo.asked("search "+sprintJQL)) == 0 {
		t.Errorf("switching did not run the second view's query; searches were %v", repo.asked("search"))
	}
}

func TestTheViewKeyDoesNothingWithASingleView(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := repo.live(t, 120, 40)
	before := len(repo.asked("search"))

	// Act
	typing(t, model, "v")

	// Assert
	if got := len(repo.asked("search")); got != before {
		t.Errorf("pressing v with one view searched again (%d then %d)", before, got)
	}
}

func TestASingleViewOffersNoSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	// completeConfig names no views, so the one built-in list is in effect.
	model := newWorld().live(t, 120, 40)

	// Act
	view := model.View().Content

	// Assert
	refuseScreen(t, footerLine(view), "v switch view")
	refuseScreen(t, view, "Assigned to me")
}
