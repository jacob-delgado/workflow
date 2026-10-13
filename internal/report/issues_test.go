// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/report"
)

// issueKey and issueSummary are the issue the mappers are given.
const (
	issueKey     = "PROJ-7"
	issueSummary = "Fix it"
)

func TestIssuesPageListsNoneAsEmptyArrays(t *testing.T) {
	t.Parallel()

	// Act
	page := report.IssuesPage(jira.SearchResult{}, 50)

	// Assert
	if page.Issues == nil || len(page.Issues) != 0 {
		t.Errorf("Issues = %#v, want an empty list", page.Issues)
	}

	if page.Unavailable == nil || len(page.Unavailable) != 0 {
		t.Errorf("Unavailable = %#v, want an empty list", page.Unavailable)
	}

	if page.StartAt != 50 {
		t.Errorf("StartAt = %d, want the caller's 50", page.StartAt)
	}
}

func TestIssuesPageMapsEachIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	result := jira.SearchResult{
		Issues: []jira.Issue{{
			Key: issueKey, Summary: issueSummary, Status: "In Progress",
			StatusCategory: jira.CategoryIndeterminate, Type: "Bug", Priority: "High",
		}},
		Total:       9,
		Unavailable: []string{"the forge's issues"},
	}

	// Act
	page := report.IssuesPage(result, 0)

	// Assert
	high := "High"

	want := api.Issue{
		Key: issueKey, Tracker: api.IssueTrackerJira, Summary: issueSummary, Status: "In Progress",
		StatusCategory: api.StatusCategoryIndeterminate, Type: "Bug", Priority: &high,
	}
	if len(page.Issues) != 1 || page.Issues[0].Key != want.Key || *page.Issues[0].Priority != high ||
		page.Issues[0].StatusCategory != want.StatusCategory || page.Issues[0].Tracker != want.Tracker {
		t.Errorf("Issues = %+v, want [%+v]", page.Issues, want)
	}

	if page.Total != 9 || len(page.Unavailable) != 1 {
		t.Errorf("Total, Unavailable = %d, %v, want 9 and the forge's issues", page.Total, page.Unavailable)
	}
}

func TestTrackerOfTellsAForgeNumberFromAJiraKey(t *testing.T) {
	t.Parallel()

	cases := map[jira.Key]api.IssueTracker{
		"42":     api.IssueTrackerForge,
		"#42":    api.IssueTrackerForge,
		"PROJ-1": api.IssueTrackerJira,
	}

	for issueKey, want := range cases {
		t.Run(string(issueKey), func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := report.TrackerOf(issueKey); got != want {
				t.Errorf("TrackerOf(%q) = %q, want %q", issueKey, got, want)
			}
		})
	}
}

func TestIssueDetailLeavesOutWhatTheTrackerDidNotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	posted := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	detail := jira.IssueDetail{
		Issue:        jira.Issue{Key: issueKey, Summary: issueSummary, Status: "To Do", StatusCategory: jira.CategoryNew},
		Reporter:     "ann",
		Description:  "It broke.",
		Comments:     []jira.Comment{{Author: "bo", Body: "first"}, {Author: "cy", Body: "second", Created: posted}},
		CommentTotal: 5,
	}

	// Act
	got := report.IssueDetail(detail, "https://jira.example/browse/"+issueKey)

	// Assert
	if got.Priority != nil || got.Assignee != nil {
		t.Errorf("Priority, Assignee = %v, %v, want both left out", got.Priority, got.Assignee)
	}

	wantComments := []api.Comment{{Author: "bo", Body: "first"}, {Author: "cy", Body: "second", Created: &posted}}
	if !reflect.DeepEqual(got.Comments, wantComments) {
		t.Errorf("Comments = %+v, want first (no time) then second at %v", got.Comments, posted)
	}

	if got.URL != "https://jira.example/browse/"+issueKey || got.CommentTotal != 5 || got.Reporter != "ann" {
		t.Errorf("URL, CommentTotal, Reporter = %q, %d, %q", got.URL, got.CommentTotal, got.Reporter)
	}
}

func TestACommentKeepsItsAuthorBodyAndTime(t *testing.T) {
	t.Parallel()

	// Arrange
	posted := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	// Act
	got := report.Comment(jira.Comment{Author: "bo", Body: "done", Created: posted})

	// Assert
	if got.Author != "bo" || got.Body != "done" || got.Created == nil || !got.Created.Equal(posted) {
		t.Errorf("Comment = %+v, want bo's done at %v", got, posted)
	}
}
