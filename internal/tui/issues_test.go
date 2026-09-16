// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// issue builds a fixture row. A function rather than package variables, which
// gochecknoglobals forbids in tests as well.
func issue(key, summary, category string) jira.Issue {
	return jira.Issue{Key: key, Summary: summary, Status: "In Progress", StatusCategory: category}
}

// assigned is a search that finds exactly these issues.
func assigned(issues ...jira.Issue) tui.IssueSearch {
	return func() (jira.SearchResult, error) {
		return jira.SearchResult{Issues: issues, Total: len(issues)}, nil
	}
}

// failing is a search that fails with err.
func failing(err error) tui.IssueSearch {
	return func() (jira.SearchResult, error) {
		return jira.SearchResult{Issues: nil, Total: 0}, err
	}
}

// started runs the model's Init command and feeds its message back in, which is
// exactly what Bubble Tea does at start.
func started(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init returned no command, want the issue search")
	}

	next, _ := model.Update(cmd())

	return concrete(t, next)
}

// issuesScreen is a started model with a search, at a size that shows the rail.
func issuesScreen(t *testing.T, search tui.IssueSearch) tui.Model {
	t.Helper()

	return started(t, sized(t, tui.New(completeConfig(), nil, search), 120, 40))
}

func TestIssuesPaneSaysLoadingBeforeTheSearchAnswers(t *testing.T) {
	t.Parallel()

	view := sized(t, tui.New(completeConfig(), nil, assigned(issue("OPS-1", "Fix login", "new"))), 120, 40).View()

	if !strings.Contains(view, "loading") {
		t.Errorf("the pane does not say it is loading:\n%s", view)
	}

	if strings.Contains(view, "OPS-1") {
		t.Errorf("an issue appeared before the search answered:\n%s", view)
	}
}

func TestInitSearchesOnlyWhenItsCommandRuns(t *testing.T) {
	t.Parallel()

	var searched atomic.Bool

	search := func() (jira.SearchResult, error) {
		searched.Store(true)

		return jira.SearchResult{Issues: nil, Total: 0}, nil
	}

	cmd := tui.New(completeConfig(), nil, search).Init()

	// Building the command must not block on the network: Bubble Tea runs it
	// off the update loop, which is what keeps the screen responsive.
	if searched.Load() {
		t.Fatal("Init ran the search itself, want it deferred to the command")
	}

	cmd()

	if !searched.Load() {
		t.Error("running Init's command did not search")
	}
}

func TestIssuesPaneListsTheAssignedIssues(t *testing.T) {
	t.Parallel()

	view := issuesScreen(t, assigned(
		issue("OPS-1", "Fix login", "indeterminate"),
		issue("OPS-2", "Rotate keys", "new"),
	)).View()

	// The first row is selected, and its glyph carries the status by shape.
	if !strings.Contains(view, "▸ ◐ OPS-1 Fix login") {
		t.Errorf("the first issue is not listed and selected:\n%s", view)
	}

	if !strings.Contains(view, "○ OPS-2 Rotate keys") {
		t.Errorf("the second issue is not listed:\n%s", view)
	}
}

func TestIssuesPaneSaysWhenNothingIsAssigned(t *testing.T) {
	t.Parallel()

	view := issuesScreen(t, assigned()).View()

	if !strings.Contains(view, "no open issues assigned to you") {
		t.Errorf("an empty list does not say so:\n%s", view)
	}
}

func TestIssuesPaneFailsWithoutTakingTheScreenDown(t *testing.T) {
	t.Parallel()

	view := issuesScreen(t, failing(jira.ErrUnauthorized)).View()

	if !strings.Contains(view, "failed") {
		t.Errorf("the pane does not show its failure:\n%s", view)
	}

	// The reason, where there is room to read it.
	if !strings.Contains(view, "the credential was not accepted") {
		t.Errorf("the detail does not give the reason:\n%s", view)
	}

	// Everything else is still there: one service failing is one pane's problem.
	for _, want := range []string{devChannel, "bearer token", "5 Slack", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("the failure took %q off the screen:\n%s", want, view)
		}
	}
}

func TestJAndKMoveTheSelectionAndStopAtTheEnds(t *testing.T) {
	t.Parallel()

	screen := issuesScreen(t, assigned(
		issue("OPS-1", "First", "new"),
		issue("OPS-2", "Second", "new"),
		issue("OPS-3", "Third", "new"),
	))

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

			if view := press(t, screen, tt.keys...).View(); !strings.Contains(view, tt.want) {
				t.Errorf("after %v, want %q selected:\n%s", tt.keys, tt.want, view)
			}
		})
	}
}

func TestListKeysDoNothingOffTheIssuesPane(t *testing.T) {
	t.Parallel()

	screen := issuesScreen(t, assigned(issue("OPS-1", "First", "new"), issue("OPS-2", "Second", "new")))

	// Focus Branch, press j, come back: the selection must not have moved.
	view := press(t, screen, "2", "j", "1").View()

	if !strings.Contains(view, "▸ ○ OPS-1") {
		t.Errorf("j moved the list while another pane had focus:\n%s", view)
	}
}

func TestTheListScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	issues := make([]jira.Issue, 0, 20)
	for index := 1; index <= 20; index++ {
		issues = append(issues, issue(fmt.Sprintf("OPS-%d", index), "work", "new"))
	}

	screen := issuesScreen(t, assigned(issues...))

	keys := make([]string, 19)
	for index := range keys {
		keys[index] = "j"
	}

	view := press(t, screen, keys...).View()

	if !strings.Contains(view, "▸ ○ OPS-20") {
		t.Errorf("the last issue scrolled out of view when selected:\n%s", view)
	}
}

func TestAnUnrecognizedStatusCategoryStillRenders(t *testing.T) {
	t.Parallel()

	view := issuesScreen(t, assigned(issue("OPS-1", "Custom", "something-new"))).View()

	if !strings.Contains(view, "· OPS-1") {
		t.Errorf("an unknown status category did not get a neutral glyph:\n%s", view)
	}
}

func TestTheDetailShowsTheSelectedIssue(t *testing.T) {
	t.Parallel()

	view := press(t, issuesScreen(t, assigned(
		issue("OPS-1", "Fix login", "new"),
		issue("OPS-2", "Rotate keys", "new"),
	)), "j").View()

	if !strings.Contains(view, "OPS-2 Rotate keys") || !strings.Contains(view, "In Progress") {
		t.Errorf("the detail does not show the selected issue:\n%s", view)
	}
}

func TestTheDetailSaysWhenTheListIsCapped(t *testing.T) {
	t.Parallel()

	capped := func() (jira.SearchResult, error) {
		return jira.SearchResult{Issues: []jira.Issue{issue("OPS-1", "One", "new")}, Total: 73}, nil
	}

	if view := issuesScreen(t, capped).View(); !strings.Contains(view, "showing 1 of 73") {
		t.Errorf("a capped list does not say how many there were:\n%s", view)
	}
}

func TestANarrowTerminalShowsTheListFullWidth(t *testing.T) {
	t.Parallel()

	// With no rail, the focused pane's own content takes the whole body — here
	// the list, not the selected issue's detail.
	model := started(t, sized(t, tui.New(completeConfig(), nil, assigned(
		issue("OPS-1", "Fix login", "new"),
		issue("OPS-2", "Rotate keys", "new"),
	)), 80, 30))

	view := model.View()

	if !strings.Contains(view, "▸ ○ OPS-1 Fix login") || !strings.Contains(view, "○ OPS-2 Rotate keys") {
		t.Errorf("the narrow view does not show the list:\n%s", view)
	}
}

func TestTheAnswerRendersTheSameWhicheverArrivesFirst(t *testing.T) {
	t.Parallel()

	search := assigned(issue("OPS-1", "Fix login", "new"))

	// Bubble Tea gives no ordering guarantee between the size and the answer.
	sizedFirst := started(t, sized(t, tui.New(completeConfig(), nil, search), 120, 40)).View()
	answerFirst := sized(t, started(t, tui.New(completeConfig(), nil, search)), 120, 40).View()

	if sizedFirst != answerFirst {
		t.Errorf("the order of size and answer changed the screen:\n%s\n---\n%s", sizedFirst, answerFirst)
	}
}

// Compile-time proof that a func literal of the right shape is an IssueSearch,
// so the wiring in internal/cli cannot drift from what these tests exercise.
var _ tui.IssueSearch = func() (jira.SearchResult, error) { return jira.SearchResult{}, nil }
