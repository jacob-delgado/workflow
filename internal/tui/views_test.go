// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The two views' queries in these tests: the first lists the world's own
// issues, the second its sprint.
const (
	myWorkJQL = "assignee = currentUser()"
	sprintJQL = "sprint in openSprints()"
)

// twoViewConfig is a configuration that names two views, "My work" and then
// "Sprint board".
func twoViewConfig() config.Config {
	cfg := completeConfig()
	cfg.Jira.Views = []config.JiraView{
		{Name: "My work", JQL: myWorkJQL},
		{Name: "Sprint board", JQL: sprintJQL},
	}

	return cfg
}

// twoViewRepo is a world whose second view is filled with its own issue.
func twoViewRepo() *world {
	repo := newWorld()
	repo.viewIssues = map[string][]jira.Issue{
		sprintJQL: {issue("OPS-9", "Sprint task", "new")},
	}

	return repo
}

// twoViewWorld is the two-view world's interface, sized and with everything it
// loads at start loaded.
func twoViewWorld(t *testing.T) tui.Model {
	t.Helper()

	model := sized(t, tui.New(twoViewConfig(), nil, twoViewRepo().deps()), 120, 40)

	return drain(t, model, model.Init())
}

// startedHoldingTheFirstSearch runs Init's loads one at a time and finishes each,
// except the first view's search: its answer is handed back undelivered. The
// search is found by the call it records, not by its place in the batch.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func startedHoldingTheFirstSearch(t *testing.T, repo *world, model tui.Model) (tui.Model, tea.Msg) {
	t.Helper()

	loads, ok := model.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("Init did not start its loads as a batch")
	}

	var held tea.Msg

	for _, load := range loads {
		if load == nil {
			continue
		}

		searchesBefore := len(repo.asked("search " + myWorkJQL))
		msg := load()

		if len(repo.asked("search "+myWorkJQL)) > searchesBefore {
			held = msg

			continue
		}

		model = drain(t, model, func() tea.Msg { return msg })
	}

	if held == nil {
		t.Fatal("Init never searched the first view")
	}

	return model, held
}

// switchedWhileTheFirstSearchIsOut is the two-view world moved on to its second
// view, and loaded there, before the first view's search has answered; that
// answer is handed back undelivered.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func switchedWhileTheFirstSearchIsOut(t *testing.T) (*world, tui.Model, tea.Msg) {
	t.Helper()

	repo := twoViewRepo()
	model := sized(t, tui.New(twoViewConfig(), nil, repo.deps()), 120, 40)
	model, firstViewsAnswer := startedHoldingTheFirstSearch(t, repo, model)

	return repo, typing(t, model, "v"), firstViewsAnswer
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
	repo := newWorld()
	model := sized(t, tui.New(twoViewConfig(), nil, repo.deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	typing(t, model, "v")

	// Assert
	if len(repo.asked("search "+sprintJQL)) == 0 {
		t.Errorf("switching did not run the second view's query; searches were %v", repo.asked("search"))
	}
}

func TestAnAnswerForAViewNoLongerShownIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	_, model, firstViewsAnswer := switchedWhileTheFirstSearchIsOut(t)

	// Act
	updated, _ := model.Update(firstViewsAnswer)

	// Assert
	view := concrete(t, updated).View().Content
	requireScreen(t, view, "Sprint board", "OPS-9")
	refuseScreen(t, view, "Add retries")
}

func TestAnAnswerForAViewNoLongerShownIsStillCached(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, model, firstViewsAnswer := switchedWhileTheFirstSearchIsOut(t)

	// Act
	model.Update(firstViewsAnswer)

	// Assert
	// Only the first view lists two issues; the sprint's one was cached as "cache 1".
	if calls := repo.asked("cache 2"); len(calls) != 1 {
		t.Errorf("the first view's two issues were cached %d times, want once; cache calls were %q",
			len(calls), repo.asked("cache"))
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
