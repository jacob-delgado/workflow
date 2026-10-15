// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestListIssuesReturnsAPage(t *testing.T) {
	t.Parallel()

	// Act
	page := webtest.Decode[api.IssuesPage](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(), config.Default()),
		"/api/issues?start_at=0"))

	// Assert
	if page.Total != 1 || len(page.Issues) != 1 || page.Issues[0].Key != webtest.TestKey {
		t.Errorf("page = %+v, want the one issue", page)
	}

	if page.Issues[0].StatusCategory != api.StatusCategoryIndeterminate {
		t.Errorf("status_category = %q, want indeterminate", page.Issues[0].StatusCategory)
	}
}

func TestListIssuesIsEmptyWhenTheTrackerIsNotConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Search = nil

	// Act
	page := webtest.Decode[api.IssuesPage](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues"))

	// Assert
	if page.Total != 0 || len(page.Issues) != 0 {
		t.Errorf("page = %+v, want an empty page", page)
	}
}

func TestListIssuesReportsASeamFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
		return jira.SearchResult{}, webtest.ErrSeam
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}

	failure := webtest.Decode[api.Problem](t, recorder)
	if failure.Code != api.ProblemCodeInternal || strings.Contains(failure.Detail, "seam") {
		t.Errorf("error = %+v, want a generic internal error", failure)
	}
}

func TestListIssuesRefusesAnUnknownView(t *testing.T) {
	t.Parallel()

	// Arrange
	// A typo in the view must not quietly answer with the default view's issues.
	searched := false
	deps := webtest.FilledDeps()
	deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
		searched = true

		return jira.SearchResult{}, nil
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues?view=nope")

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.ProblemCodeNotFound {
		t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
	}

	if searched {
		t.Error("the tracker was searched for a view that does not exist")
	}
}

func TestListIssuesUsesTheNamedView(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotJQL string

	deps := webtest.FilledDeps()
	deps.Jira.Search = func(_ context.Context, jql string, _ int) (jira.SearchResult, error) {
		gotJQL = jql

		return jira.SearchResult{}, nil
	}

	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: webtest.TestBugView, JQL: webtest.TestBugJQL}}

	// Act
	_ = webtest.Get(t, webtest.Serve(t, deps, cfg), "/api/issues?view="+webtest.TestBugView)

	// Assert
	if want := jira.ScopedToMe(webtest.TestBugJQL); gotJQL != want {
		t.Errorf("searched %q, want the named view's JQL scoped to you, %q", gotJQL, want)
	}
}

func TestGetIssueReturnsTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	detail := webtest.Decode[api.IssueDetail](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(), config.Default()),
		"/api/issues/"+webtest.TestKey))

	// Assert
	if detail.Key != webtest.TestKey || detail.Reporter != webtest.TestReporter || detail.CommentTotal != 1 {
		t.Errorf("detail = %+v, want the full issue", detail)
	}
}

func TestGetIssueLinksTheIssueAndNamesItsAssignee(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	issue := deps.Jira.Issue
	deps.Jira.Issue = func(ctx context.Context, key jira.Key) (jira.IssueDetail, error) {
		detail, err := issue(ctx, key)
		detail.Assignee = webtest.TestAuthor

		return detail, err
	}
	deps.Jira.BrowseURL = func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }

	// Act
	detail := webtest.Decode[api.IssueDetail](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()),
		"/api/issues/"+webtest.TestKey))

	// Assert
	if detail.URL != "https://jira.example.com/browse/"+webtest.TestKey {
		t.Errorf("url = %q, want the issue's page in the tracker", detail.URL)
	}

	if detail.Assignee == nil || *detail.Assignee != webtest.TestAuthor {
		t.Errorf("assignee = %v, want %q", detail.Assignee, webtest.TestAuthor)
	}
}

func TestGetIssueHasNoLinkOrAssigneeWhenNeitherIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	// webtest.FilledDeps names no assignee and wires no BrowseURL: the link is
	// empty rather than absent, and the assignee is left out.
	handler := webtest.Serve(t, webtest.FilledDeps(), config.Default())

	// Act
	recorder := webtest.Get(t, handler, "/api/issues/"+webtest.TestKey)

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
	deps := webtest.FilledDeps()
	deps.Jira.Issue = nil

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if failure := webtest.Decode[api.Problem](t, recorder); recorder.Code != http.StatusUnprocessableEntity ||
		failure.Code != api.ProblemCodeUnprocessable {
		t.Errorf("status/code = %d/%s, want 422/unprocessable", recorder.Code, failure.Code)
	}
}

func TestGetIssueIsNotFoundForAMissingIssue(t *testing.T) {
	t.Parallel()

	// Each case is how the tracker says there is no such issue: Jira's own
	// answer, or forge issues read where no repository names a forge.
	cases := map[string]error{
		"not in Jira":         jira.ErrNotFound,
		"no forge repository": forge.ErrNoRepository,
	}

	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Jira.Issue = func(context.Context, jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, cause }

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues/PROJ-404")

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusNotFound || failure.Code != api.ProblemCodeNotFound {
				t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
			}

			if !strings.Contains(failure.Detail, "PROJ-404") {
				t.Errorf("detail = %q, want the specific issue key so it is the dedicated 404 path", failure.Detail)
			}
		})
	}
}

func TestGetIssueIsUnreachableWhenTheTrackerIsDown(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Issue = func(context.Context, jira.Key) (jira.IssueDetail, error) {
		return jira.IssueDetail{}, jira.ErrUnreachable
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if failure := webtest.Decode[api.Problem](t, recorder); recorder.Code != http.StatusBadGateway ||
		failure.Code != api.ProblemCodeUnreachable {
		t.Errorf("status/code = %d/%s, want 502/unreachable", recorder.Code, failure.Code)
	}
}

func TestGetIssueReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Issue = func(context.Context, jira.Key) (jira.IssueDetail, error) {
		return jira.IssueDetail{}, webtest.ErrSeam
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestListIssuesSaysEachIssuesTrackerAndWhichCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.Search = func(context.Context, string, int) (jira.SearchResult, error) {
		return jira.SearchResult{
			Issues:      []jira.Issue{{Key: "57", Summary: "Typo"}, {Key: webtest.TestKey, Summary: "Leak"}},
			Total:       2,
			Unavailable: []string{"the forge's issues"},
		}, nil
	}

	// Act
	page := webtest.Decode[api.IssuesPage](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()),
		"/api/issues?start_at=0"))

	// Assert
	if len(page.Issues) != 2 || page.Issues[0].Tracker != api.IssueTrackerForge ||
		page.Issues[1].Tracker != api.IssueTrackerJira {
		t.Errorf("issues = %+v, want 57 from the forge and %s from Jira", page.Issues, webtest.TestKey)
	}

	if len(page.Unavailable) != 1 || page.Unavailable[0] != "the forge's issues" {
		t.Errorf("unavailable = %v, want the forge's issues named", page.Unavailable)
	}
}
