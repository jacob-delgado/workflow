// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// errSeam is what a failing seam returns; the server maps every seam error to a
// generic 500 so the wire message carries no detail.
var errSeam = errors.New("the seam failed")

// Fixtures the tests share.
const (
	testKey        = "PROJ-412"
	testReporter   = "Ana Lopez"
	testBranchName = "fix/PROJ-412"
	testAuthor     = "octocat"
	testVersion    = "1.2.3"
	testBugJQL     = "type = Bug"
	testBase       = "origin/main"
	// noBranchSeam is the shared name for the "no branch seam" case the write
	// handlers' unavailability tables each exercise.
	noBranchSeam = "no branch seam"
	// loopbackHost is the Host the shared request helpers send, so requests pass
	// the loopback guard the same way a browser on 127.0.0.1 does. A test that
	// exercises the guard sets its own Host instead.
	loopbackHost = "127.0.0.1:7000"
)

// filledDeps is a Deps with every seam populated with canned answers. A test
// nils a seam to exercise the not-configured path, or replaces one to fail.
func filledDeps() webserver.Deps {
	return webserver.Deps{
		Search: func(string, int) (jira.SearchResult, error) {
			return jira.SearchResult{
				Issues: []jira.Issue{{
					Key: testKey, Summary: "Fix token redaction", Status: "In Progress",
					StatusCategory: "indeterminate", Type: "Bug", Priority: "High",
				}},
				Total: 1,
			}, nil
		},
		Issue: func(key jira.Key) (jira.IssueDetail, error) {
			return jira.IssueDetail{
				Issue: jira.Issue{
					Key: key, Summary: "Fix token redaction", Status: "In Progress",
					StatusCategory: "indeterminate", Type: "Bug",
				},
				Reporter:     testReporter,
				Description:  "Tokens reach the log.",
				Comments:     []jira.Comment{{Author: testReporter, Body: "Repro'd", Created: time.Unix(0, 0).UTC()}},
				CommentTotal: 1,
			}, nil
		},
		Branch: func() (gitrepo.Branch, error) {
			return gitrepo.Branch{
				Name: testBranchName, Base: testBase, Ahead: 2, Head: "abc123",
				Commits: []gitrepo.Commit{{Hash: "abc123", Subject: "feat: redact"}},
			}, nil
		},
		Changes: func() ([]gitrepo.Change, error) {
			return []gitrepo.Change{{Path: "internal/config/config.go", Staged: 'M'}}, nil
		},
		FindPull: func(string) (forge.PullRequest, bool, error) {
			pull := forge.PullRequest{Number: 42, URL: "https://x/42", Title: "redact", Mergeable: forge.MergeClean}

			return pull, true, nil
		},
		CheckCI: func(forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{
				State: forge.CIPassed, Total: 3, Done: 3,
				Checks: []forge.Check{{Name: "build", State: forge.CIPassed}},
			}, nil
		},
		Author: func() (string, error) { return testAuthor, nil },
	}
}

// serve builds the API handler over deps and cfg. Handler fails only when the
// embedded spec cannot load, which is a build defect, so the test fails there.
func serve(t *testing.T, deps webserver.Deps, cfg config.Config) http.Handler {
	t.Helper()

	return serveWith(t, deps, cfg, webserver.Info{Version: testVersion, DryRun: true})
}

// serveWith is serve with the caller's Info, for the tests that need a specific
// stream interval.
func serveWith(t *testing.T, deps webserver.Deps, cfg config.Config, info webserver.Info) http.Handler {
	t.Helper()

	handler, err := webserver.Handler(deps, cfg, info, nil)
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	return handler
}

// get sends a GET and returns the recorder.
func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, handler, http.MethodGet, target, "")
}

// send sends a request with an optional body and returns the recorder.
func send(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	request.Host = loopbackHost

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// decode unmarshals the recorder's JSON body into T, failing the test on error.
func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T

	err := json.Unmarshal(recorder.Body.Bytes(), &value)
	if err != nil {
		t.Fatalf("decoding %T from %q: %v", value, recorder.Body.String(), err)
	}

	return value
}

func TestGetHealthReportsTheBuild(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, webserver.Deps{}, config.Default()), "/api/health")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	health := decode[api.Health](t, recorder)
	if health.Version != testVersion || !health.DryRun {
		t.Errorf("health = %+v, want version 1.2.3 and dry_run true", health)
	}
}

func TestListViewsFallsBackToTheBuiltInList(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/views")

	// Assert
	views := decode[api.ViewList](t, recorder)
	if len(views.Views) != 1 || views.Views[0].Name != "Assigned to me" {
		t.Errorf("views = %+v, want the one built-in list", views.Views)
	}
}

func TestListViewsListsTheConfiguredViews(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: "Sprint", JQL: "sprint in openSprints()"}}

	// Act
	views := decode[api.ViewList](t, get(t, serve(t, filledDeps(), cfg), "/api/views"))

	// Assert
	if len(views.Views) != 1 || views.Views[0].Name != "Sprint" || views.Views[0].Jql != "sprint in openSprints()" {
		t.Errorf("views = %+v, want the configured Sprint view", views.Views)
	}
}

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

	failure := decode[api.Error](t, recorder)
	if failure.Code != api.Internal || strings.Contains(failure.Message, "seam") {
		t.Errorf("error = %+v, want a generic internal error", failure)
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

func TestGetIssueIsNotFoundWithoutATracker(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Issue = nil

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}

func TestGetBranchReturnsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, filledDeps(), config.Default()), "/api/branch"))

	// Assert
	if branch.Name != testBranchName || branch.Base != testBase || branch.Ahead != 2 {
		t.Errorf("branch = %+v, want the current branch", branch)
	}
}

func TestGetBranchIsEmptyOutsideARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = nil

	// Act
	branch := decode[api.Branch](t, get(t, serve(t, deps, config.Default()), "/api/branch"))

	// Assert
	if branch.Name != "" || len(branch.Commits) != 0 {
		t.Errorf("branch = %+v, want an empty branch", branch)
	}
}

func TestListChangesReturnsTheWorkingTree(t *testing.T) {
	t.Parallel()

	// Act
	changes := decode[api.ChangeList](t, get(t, serve(t, filledDeps(), config.Default()), "/api/changes"))

	// Assert
	if len(changes.Changes) != 1 || changes.Changes[0].Path != "internal/config/config.go" {
		t.Errorf("changes = %+v, want the one staged file", changes.Changes)
	}
}

func TestGetSlackReturnsTheDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Slack.Token = "xoxb-t"
	cfg.Slack.Channel = "#dev"

	// Act
	slack := decode[api.Slack](t, get(t, serve(t, filledDeps(), cfg), "/api/slack"))

	// Assert
	if slack.Channel != "#dev" || slack.Author != testAuthor {
		t.Errorf("slack = %+v, want #dev and octocat", slack)
	}
}

func TestGetSlackHasNoAuthorWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = func() (string, error) { return "", errSeam }

	// Act
	slack := decode[api.Slack](t, get(t, serve(t, deps, config.Default()), "/api/slack"))

	// Assert
	if slack.Author != "" {
		t.Errorf("author = %q, want empty when the forge cannot say", slack.Author)
	}
}
