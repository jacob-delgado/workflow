// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// issue builds a fixture row. A function rather than package variables, which
// gochecknoglobals forbids in tests as well.
func issue(key, summary, category string) jira.Issue {
	return jira.Issue{Key: key, Summary: summary, Status: "In Progress", StatusCategory: category}
}

// assigned is a search that finds exactly these issues.
func assigned(issues ...jira.Issue) func(startAt int) (jira.SearchResult, error) {
	return func(int) (jira.SearchResult, error) {
		return jira.SearchResult{Issues: issues, Total: len(issues)}, nil
	}
}

// searching is an interface whose only outside dependency is this search.
func searching(search func(startAt int) (jira.SearchResult, error)) tui.Deps {
	return tui.Deps{Jira: tui.JiraDeps{Search: ignoreJQL(search)}}
}

// ignoreJQL adapts a search that does not care which view it answers to the
// seam, which passes the active view's JQL.
func ignoreJQL(search func(startAt int) (jira.SearchResult, error)) func(string, int) (jira.SearchResult, error) {
	return func(_ string, startAt int) (jira.SearchResult, error) { return search(startAt) }
}

// failing is a search that fails with err.
func failing(err error) func(startAt int) (jira.SearchResult, error) {
	return func(int) (jira.SearchResult, error) {
		return jira.SearchResult{Issues: nil, Total: 0}, err
	}
}

// started runs the model's Init commands and feeds their messages back in, which
// is what Bubble Tea does at start.
func started(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init returned no command, want the loads started")
	}

	return deliver(t, model, cmd)
}

// deliver runs a command — every command in a batch — and feeds each message
// back in, as Bubble Tea does. The commands that follow are not run: a test
// runs those when it wants them.
func deliver(t *testing.T, model tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()

	msg := cmd()

	batch, isBatch := msg.(tea.BatchMsg)
	if !isBatch {
		next, _ := model.Update(msg)

		return concrete(t, next)
	}

	for _, each := range batch {
		if each != nil {
			model = deliver(t, model, each)
		}
	}

	return model
}

// issuesScreen is a started model with a search, at a size that shows the rail.
func issuesScreen(t *testing.T, search func(startAt int) (jira.SearchResult, error)) tui.Model {
	t.Helper()

	return started(t, sized(t, tui.New(completeConfig(), nil, searching(search)), 120, 40))
}

func TestIssuesPaneSaysLoadingBeforeTheSearchAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := searching(assigned(issue("OPS-1", "Fix login", "new")))

	// Act
	view := sized(t, tui.New(completeConfig(), nil, deps), 120, 40).View()

	// Assert
	// Other panes are loading too; the heavy border is the Issues pane's own.
	requireScreen(t, view, focused("1 Issues"), "┃ loading…")
	refuseScreen(t, view, "OPS-1")
}

func TestInitSearchesOnlyWhenItsCommandRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	var searched atomic.Bool

	search := func(int) (jira.SearchResult, error) {
		searched.Store(true)

		return jira.SearchResult{Issues: nil, Total: 0}, nil
	}

	model := tui.New(completeConfig(), nil, searching(search))

	// Act: build the command
	cmd := model.Init()

	// Assert: nothing has searched yet
	// Building the command must not block on the network: Bubble Tea runs it
	// off the update loop, which is what keeps the screen responsive.
	if searched.Load() {
		t.Fatal("Init ran the search itself, want it deferred to the command")
	}

	// Act: run it
	cmd()

	// Assert: now it has
	if !searched.Load() {
		t.Error("running Init's command did not search")
	}
}

func TestIssuesPaneListsTheAssignedIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	search := assigned(issue("OPS-1", "Fix login", "indeterminate"), issue("OPS-2", "Rotate keys", "new"))

	// Act
	view := issuesScreen(t, search).View()

	// Assert
	// The first row is selected, and its glyph carries the status by shape.
	requireScreen(t, view, "▸ ◐ OPS-1 Fix login", "○ OPS-2 Rotate keys")
}

func TestSlashFiltersTheIssueListAsYouType(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := issuesScreen(t, assigned(
		issue("OPS-1", "Fix login", "indeterminate"), issue("OPS-2", "Rotate keys", "new")))

	// Act: open the filter and narrow to the second issue
	filtered := typing(t, listed, "/", "R", "o", "t")

	// Assert: only the match shows, and the filter is on the bottom row
	requireScreen(t, filtered.View(), "OPS-2 Rotate keys", "filter: Rot")
	refuseScreen(t, filtered.View(), "OPS-1")

	// Act: esc restores the full list
	restored := typing(t, filtered, keyEsc)

	// Assert: the whole list is back and the filter row is gone
	requireScreen(t, restored.View(), "OPS-1 Fix login", "OPS-2 Rotate keys")
	refuseScreen(t, restored.View(), "filter:")
}

func TestFilteringToNothingSaysSoAndBackspaceWidensIt(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := issuesScreen(t, assigned(
		issue("OPS-1", "Fix issue", "indeterminate"), issue("OPS-2", "Fix bug", "new")))

	// Act: narrow past any match
	empty := typing(t, listed, "/", "F", "i", "x", "z")

	// Assert: the list says nothing matches
	requireScreen(t, empty.View(), "no issue matches the filter")

	// Act: backspace back to a match
	widened := typing(t, empty, "backspace")

	// Assert: both issues return under the shorter filter
	requireScreen(t, widened.View(), "OPS-1 Fix issue", "OPS-2 Fix bug", "filter: Fix")
}

func TestArrowsMoveAndEnterKeepsTheFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := issuesScreen(t, assigned(
		issue("OPS-1", "Fix issue", "indeterminate"), issue("OPS-2", "Fix bug", "new")))

	// Act
	result := typing(t, listed, "/", "F", "i", "x", "down", "enter")

	// Assert
	requireScreen(t, result.View(), "▸ ○ OPS-2 Fix bug", "filter: Fix")
}

// manyIssues builds count numbered issue rows.
func manyIssues(count int) []jira.Issue {
	issues := make([]jira.Issue, 0, count)
	for index := 1; index <= count; index++ {
		issues = append(issues, issue(fmt.Sprintf("OPS-%d", index), "work", "new"))
	}

	return issues
}

func TestReachingTheEndOfATruncatedListLoadsTheNextPage(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.pageSize = 3
	world.issues = manyIssues(5)

	// Act: open the list
	listed := world.live(t, 120, 40)

	// Assert: only the first page is loaded, and the count says so
	requireScreen(t, listed.View(), "showing 3 of 5")

	// Act: move to the end of the loaded page
	paged := typing(t, listed, "j", "j")

	// Assert: the next page loaded, and the count now covers the whole list
	refuseScreen(t, paged.View(), "showing 3 of 5")

	if searches := len(world.asked("search")); searches < 2 {
		t.Errorf("searched %d time(s), want a second page loaded at the end", searches)
	}
}

func TestIssuesPaneSaysWhenAViewIsEmpty(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, assigned()).View()

	// Assert
	// The text names no particular query: a view may be a sprint or a filter,
	// not only the issues assigned to you.
	requireScreen(t, view, "no issues in this view")
}

func TestIssuesPaneFailsWithoutTakingTheScreenDown(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, failing(jira.ErrUnauthorized)).View()

	// Assert
	// The pane shows its failure, and the detail the reason, where there is
	// room to read it.
	requireScreen(t, view, "✗ failed", "the credential was not accepted")

	// Everything else is still there: one service failing is one pane's problem.
	requireScreen(t, view, devChannel, "bearer token", "5 Slack", "quit")
}

func TestJAndKMoveTheSelectionAndStopAtTheEnds(t *testing.T) {
	t.Parallel()

	const secondSelected = "▸ ○ OPS-2"

	cases := map[string]struct {
		keys []string
		want string
	}{
		"j moves down":           {keys: []string{"j"}, want: secondSelected},
		"down arrow moves down":  {keys: []string{"down"}, want: secondSelected},
		"k moves back up":        {keys: []string{"j", "j", "k"}, want: secondSelected},
		"up arrow moves up":      {keys: []string{"j", "up"}, want: "▸ ○ OPS-1"},
		"it stops at the bottom": {keys: []string{"j", "j", "j", "j"}, want: "▸ ○ OPS-3"},
		"it stops at the top":    {keys: []string{"k", "k"}, want: "▸ ○ OPS-1"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			screen := issuesScreen(t, assigned(
				issue("OPS-1", "First", "new"),
				issue("OPS-2", "Second", "new"),
				issue("OPS-3", "Third", "new"),
			))

			// Act
			view := press(t, screen, tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestListKeysDoNothingOffTheIssuesPane(t *testing.T) {
	t.Parallel()

	// Arrange
	screen := issuesScreen(t, assigned(issue("OPS-1", "First", "new"), issue("OPS-2", "Second", "new")))

	// Act
	// Focus Branch, press j, come back.
	view := press(t, screen, "2", "j", "1").View()

	// Assert
	requireScreen(t, view, "▸ ○ OPS-1")
	refuseScreen(t, view, "▸ ○ OPS-2")
}

func TestTheListScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	// Arrange
	// More issues than the tallest rail has rows.
	issues := make([]jira.Issue, 0, 60)
	for index := 1; index <= 60; index++ {
		issues = append(issues, issue(fmt.Sprintf("OPS-%d", index), "work", "new"))
	}

	screen := issuesScreen(t, assigned(issues...))

	// Act
	view := press(t, screen, slices.Repeat([]string{"j"}, 59)...).View()

	// Assert
	requireScreen(t, view, "▸ ○ OPS-60")
	refuseScreen(t, view, "○ OPS-1 work")
}

func TestAnUnrecognizedStatusCategoryStillRenders(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, assigned(issue("OPS-1", "Custom", "something-new"))).View()

	// Assert
	requireScreen(t, view, "· OPS-1")
}

func TestTheDetailShowsTheSelectedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	screen := issuesScreen(t, assigned(issue("OPS-1", "Fix login", "new"), issue("OPS-2", "Rotate keys", "new")))

	// Act
	view := press(t, screen, "j").View()

	// Assert
	// The rail lists both; only the detail box opens on an issue's key.
	requireScreen(t, view, "│ OPS-2 Rotate keys", "In Progress")
	refuseScreen(t, view, "│ OPS-1 Fix login")
}

func TestTheDetailSaysWhenTheListIsCapped(t *testing.T) {
	t.Parallel()

	// Arrange
	capped := func(int) (jira.SearchResult, error) {
		return jira.SearchResult{Issues: []jira.Issue{issue("OPS-1", "One", "new")}, Total: 73}, nil
	}

	// Act
	view := issuesScreen(t, capped).View()

	// Assert
	requireScreen(t, view, "showing 1 of 73")
}

func TestANarrowTerminalShowsTheListFullWidth(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := searching(assigned(issue("OPS-1", "Fix login", "new"), issue("OPS-2", "Rotate keys", "new")))

	// Act
	view := started(t, sized(t, tui.New(completeConfig(), nil, deps), 79, 30)).View()

	// Assert
	// With no rail, the focused pane's own content takes the whole body — here
	// the list, not the selected issue's detail.
	requireScreen(t, view, "▸ ○ OPS-1 Fix login", "○ OPS-2 Rotate keys")
}

func TestTheAnswerRendersTheSameWhicheverArrivesFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	search := assigned(issue("OPS-1", "Fix login", "new"))

	// Act
	// Bubble Tea gives no ordering guarantee between the size and the answer.
	sizedFirst := started(t, sized(t, tui.New(completeConfig(), nil, searching(search)), 120, 40)).View()
	answerFirst := sized(t, started(t, tui.New(completeConfig(), nil, searching(search))), 120, 40).View()

	// Assert
	requireScreen(t, sizedFirst, "▸ ○ OPS-1 Fix login")

	if sizedFirst != answerFirst {
		t.Errorf("the order of size and answer changed the screen:\n%s\n---\n%s", sizedFirst, answerFirst)
	}
}

func TestRRefreshesTheIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	refreshing := newWorld()
	model := refreshing.live(t, 120, 40)
	before := len(refreshing.asked("search"))

	// Act
	typing(t, model, "r")

	// Assert
	if searches := len(refreshing.asked("search")); searches != before+1 {
		t.Errorf("searched %d times, want once more than the %d before r", searches, before)
	}
}

func TestAFailedSearchReadsAsAFailureInTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, failing(jira.ErrUnauthorized)).View()

	// Assert
	requireScreen(t, view, "✗ the credential was not accepted")
	refuseScreen(t, view, "issues: the credential was not accepted")
}

func TestRefreshMarksTheIssuesTitleInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	refreshing, _ := pressed(t, model, "r")

	// Assert
	requireScreen(t, refreshing.View(), "1 Issues ◐")
}
