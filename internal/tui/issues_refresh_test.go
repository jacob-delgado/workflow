// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Search failure and refresh, split from issues_test.go.

import (
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAFailedSearchReadsAsAFailureInTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, failing(jira.ErrUnauthorized)).View().Content

	// Assert
	requireScreen(t, view, "✗ Jira did not accept the token")
	refuseScreen(t, view, "issues: the credential was not accepted")
}

func TestRefreshMarksTheIssuesTitleInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	refreshing, _ := pressed(t, model, "r")

	// Assert
	requireScreen(t, refreshing.View().Content, "1 Issues ◐")
}

// listsThenFails is a search that lists the issues the first time and fails
// every time after, as a tracker that goes down between the load and a
// refresh does.
func listsThenFails(issues ...jira.Issue) func(startAt int) (jira.SearchResult, error) {
	var searches atomic.Int32

	return func(int) (jira.SearchResult, error) {
		if searches.Add(1) > 1 {
			return jira.SearchResult{Issues: nil, Total: 0}, jira.ErrUnreachable
		}

		return jira.SearchResult{Issues: issues, Total: len(issues)}, nil
	}
}

// failedMark is what the rail draws beneath the issues a failed search kept.
const failedMark = "✗ failed · see detail"

// rowMatching is the index of the first screen row whose text matches pattern,
// and the text it matched there.
func rowMatching(t *testing.T, view string, pattern *regexp.Regexp) (int, string) {
	t.Helper()

	for row, line := range strings.Split(ansi.Strip(view), "\n") {
		if found := pattern.FindString(line); found != "" {
			return row, found
		}
	}

	t.Fatalf("no row of the screen matches %q:\n%s", pattern, ansi.Strip(view))

	return 0, ""
}

func TestAFailedRefreshKeepsTheListedIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	model := issuesScreen(t, listsThenFails(issue(issueKey, issueSummary, "indeterminate")))

	// Act
	refreshed := typing(t, model, "r")

	// Assert
	requireScreen(t, refreshed.View().Content, issueKey+" "+issueSummary, failedMark)
}

func TestAFailedRefreshMarksTheFailureBeneathAFullRail(t *testing.T) {
	t.Parallel()

	// Arrange
	// More issues than the tallest rail has rows, so the list alone fills it.
	model := issuesScreen(t, listsThenFails(manyIssues(60)...))

	// Act
	refreshed := typing(t, model, "r")

	// Assert
	requireScreen(t, refreshed.View().Content, "▸ ○ OPS-1 work", failedMark)
}

func TestAClickAboveTheFailureMarkPicksTheIssueDrawnThere(t *testing.T) {
	t.Parallel()

	// Arrange
	// Far enough down that the rail scrolls, then a refresh that fails, so the
	// mark takes a row beneath the scrolled list.
	keys := append(slices.Repeat([]string{"j"}, 45), "r")
	screen := typing(t, issuesScreen(t, listsThenFails(manyIssues(60)...)), keys...)
	// The detail shows the failure, so the first issue key on screen is the
	// rail's first row.
	row, drawn := rowMatching(t, screen.View().Content, regexp.MustCompile(`OPS-\d+`))

	// Act
	view := click(t, screen, 5, row).View().Content

	// Assert
	requireScreen(t, view, "▸ ○ "+drawn+" work")
}

func TestAClickOnTheFailureMarkPicksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	screen := typing(t, issuesScreen(t, listsThenFails(manyIssues(60)...)), "r")
	row, _ := rowMatching(t, screen.View().Content, regexp.MustCompile(regexp.QuoteMeta(failedMark)))

	// Act
	view := click(t, screen, 5, row).View().Content

	// Assert
	requireScreen(t, view, "▸ ○ OPS-1 work")
}
