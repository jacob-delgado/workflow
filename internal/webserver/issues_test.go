// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestListIssuesReturnsAPage(t *testing.T) {
	t.Parallel()

	// Act
	page := decode[api.IssuesPage](t, get(t, serve(t, filledDeps(), config.Default()), "/api/issues?start_at=0"))

	// Assert
	if page.Total != 1 || len(page.Issues) != 1 || page.Issues[0].Key != testKey {
		t.Errorf("page = %+v, want the one issue", page)
	}

	if page.Issues[0].StatusCategory != api.StatusCategoryIndeterminate {
		t.Errorf("status_category = %q, want indeterminate", page.Issues[0].StatusCategory)
	}
}

func TestListIssuesIsEmptyWhenTheTrackerIsNotConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Search = nil

	// Act
	page := decode[api.IssuesPage](t, get(t, serve(t, deps, config.Default()), "/api/issues"))

	// Assert
	if page.Total != 0 || len(page.Issues) != 0 {
		t.Errorf("page = %+v, want an empty page", page)
	}
}

func TestListIssuesReportsASeamFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Search = func(string, int) (jira.SearchResult, error) { return jira.SearchResult{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}

	failure := decode[api.Problem](t, recorder)
	if failure.Code != api.Internal || strings.Contains(failure.Detail, "seam") {
		t.Errorf("error = %+v, want a generic internal error", failure)
	}
}

func TestListIssuesRefusesAnUnknownView(t *testing.T) {
	t.Parallel()

	// Arrange
	// A typo in the view must not quietly answer with the default view's issues.
	searched := false
	deps := filledDeps()
	deps.Search = func(string, int) (jira.SearchResult, error) {
		searched = true

		return jira.SearchResult{}, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues?view=nope")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.NotFound {
		t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
	}

	if searched {
		t.Error("the tracker was searched for a view that does not exist")
	}
}

func TestGetIssueReturnsTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	detail := decode[api.IssueDetail](t, get(t, serve(t, filledDeps(), config.Default()), "/api/issues/"+testKey))

	// Assert
	if detail.Key != testKey || detail.Reporter != testReporter || detail.CommentTotal != 1 {
		t.Errorf("detail = %+v, want the full issue", detail)
	}
}

func TestGetIssueLinksTheIssueAndNamesItsAssignee(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	issue := deps.Issue
	deps.Issue = func(key jira.Key) (jira.IssueDetail, error) {
		detail, err := issue(key)
		detail.Assignee = testAuthor

		return detail, err
	}
	deps.BrowseURL = func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }

	// Act
	detail := decode[api.IssueDetail](t, get(t, serve(t, deps, config.Default()), "/api/issues/"+testKey))

	// Assert
	if detail.URL != "https://jira.example.com/browse/"+testKey {
		t.Errorf("url = %q, want the issue's page in the tracker", detail.URL)
	}

	if detail.Assignee == nil || *detail.Assignee != testAuthor {
		t.Errorf("assignee = %v, want %q", detail.Assignee, testAuthor)
	}
}

func TestGetIssueHasNoLinkOrAssigneeWhenNeitherIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps names no assignee and wires no BrowseURL: the link is empty
	// rather than absent, and the assignee is left out.
	handler := serve(t, filledDeps(), config.Default())

	// Act
	recorder := get(t, handler, "/api/issues/"+testKey)

	// Assert
	var raw map[string]any

	err := json.Unmarshal(recorder.Body.Bytes(), &raw)
	if err != nil {
		t.Fatalf("decoding %q: %v", recorder.Body.String(), err)
	}

	if link, ok := raw["url"]; !ok || link != "" {
		t.Errorf("url = %v (present %t), want an empty string", link, ok)
	}

	if _, ok := raw["assignee"]; ok {
		t.Errorf("assignee = %v, want it absent for an unassigned issue", raw["assignee"])
	}
}

func TestGetIssueIsUnprocessableWithoutATracker(t *testing.T) {
	t.Parallel()

	// Arrange
	// No tracker configured is not a missing issue: 404 is reserved for an issue
	// that genuinely does not exist, so this answers 422.
	deps := filledDeps()
	deps.Issue = nil

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusUnprocessableEntity ||
		failure.Code != api.Unprocessable {
		t.Errorf("status/code = %d/%s, want 422/unprocessable", recorder.Code, failure.Code)
	}
}

func TestGetIssueIsNotFoundForAMissingIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, jira.ErrNotFound }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-404")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.NotFound {
		t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
	}

	if !strings.Contains(failure.Detail, "PROJ-404") {
		t.Errorf("detail = %q, want the specific issue key so it is the dedicated 404 path", failure.Detail)
	}
}

func TestGetIssueIsUnreachableWhenTheTrackerIsDown(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, jira.ErrUnreachable }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusBadGateway ||
		failure.Code != api.Unreachable {
		t.Errorf("status/code = %d/%s, want 502/unreachable", recorder.Code, failure.Code)
	}
}
