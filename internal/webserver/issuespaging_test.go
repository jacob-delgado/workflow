// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestListIssuesNamesWhereTheNextPageStarts(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Jira.SearchAfter = func(context.Context, string, jira.Cursor) (jira.SearchResult, error) {
		return jira.SearchResult{
			Issues: []jira.Issue{{Key: webtest.TestKey}}, Total: 2,
			Next: jira.Cursor{After: webtest.TestKey, At: 0},
		}, nil
	}

	// Act
	page := webtest.Decode[api.IssuesPage](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues"))

	// Assert
	if want := webtest.TestKey + "@0"; page.Next != want {
		t.Errorf("next = %q, want %q: the page after follows its last issue", page.Next, want)
	}
}

func TestListIssuesAsksForThePageAfterTheIssueItsCursorNames(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked jira.Cursor

	deps := webtest.FilledDeps()
	deps.Jira.SearchAfter = func(_ context.Context, _ string, after jira.Cursor) (jira.SearchResult, error) {
		asked = after

		return jira.SearchResult{}, nil
	}

	// Act
	_ = webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues?after="+webtest.TestKey+"@49")

	// Assert
	if want := (jira.Cursor{After: webtest.TestKey, At: 49}); asked != want {
		t.Errorf("asked after %+v, want %+v", asked, want)
	}
}

func TestListIssuesRefusesACursorNoPageGave(t *testing.T) {
	t.Parallel()

	// Arrange
	searched := false
	deps := webtest.FilledDeps()
	deps.Jira.SearchAfter = func(context.Context, string, jira.Cursor) (jira.SearchResult, error) {
		searched = true

		return jira.SearchResult{}, nil
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/issues?after="+webtest.TestKey)

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadRequest || failure.Code != api.ProblemCodeBadRequest {
		t.Errorf("status/code = %d/%s, want 400/bad_request", recorder.Code, failure.Code)
	}

	if searched {
		t.Error("the tracker was searched after a cursor no page gave")
	}
}
