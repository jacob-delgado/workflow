// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The issue filledDeps' Issue seam returns: a Bug titled "Fix token redaction".
const startIssue = "PROJ-401"

// wantBranchName is the branch the convention names for that issue, the oracle a
// create test checks the handler against rather than hard-coding a brittle string.
func wantBranchName(t *testing.T) string {
	t.Helper()

	cfg := config.Default()

	return convention.NewBranchNaming(cfg.Branch.Template, cfg.Branch.DefaultPrefix, cfg.Branch.Prefixes).
		Name("Bug", startIssue, "Fix token redaction")
}

// doCreateBranch posts a start-work request for startIssue against a server over
// deps.
func doCreateBranch(t *testing.T, deps webserver.Deps) *httptest.ResponseRecorder {
	t.Helper()

	body := `{"issue_key":"` + startIssue + `"}`

	return send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/branches", body)
}

func TestCreateBranchStartsWorkOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	// No Branches seam, so no branch exists yet and the create goes ahead.
	var created string

	deps := filledDeps()
	deps.CreateBranch = func(name, _ string) error {
		created = name

		return nil
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if created != wantBranchName(t) {
		t.Errorf("created %q, want the convention's name %q", created, wantBranchName(t))
	}
}

func TestCreateBranchBranchesFromTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	// filledDeps' branch has base origin/main, so a new branch starts from it.
	var start string

	deps := filledDeps()
	deps.CreateBranch = func(_, base string) error {
		start = base

		return nil
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if start != testBase {
		t.Errorf("branched from %q, want the base %q", start, testBase)
	}
}

func TestCreateBranchRefusesWhenOneAlreadyExists(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return []string{wantBranchName(t)}, nil }
	deps.CreateBranch = func(string, string) error {
		called = true

		return nil
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 when a branch already exists", recorder.Code)
	}

	if called {
		t.Error("created a branch despite one already existing")
	}

	if failure := decode[api.Error](t, recorder); failure.Code != api.Conflict {
		t.Errorf("code = %q, want conflict", failure.Code)
	}
}

func TestCreateBranchReportsATrackerFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// Without the issue's type and summary the branch cannot be named.
	deps := filledDeps()
	deps.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }
	deps.CreateBranch = func(string, string) error { return nil }

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the issue cannot be read", recorder.Code)
	}
}

func TestCreateBranchReportsAFailedCreate(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.CreateBranch = func(string, string) error { return errSeam }

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the branch cannot be created", recorder.Code)
	}
}

func TestCreateBranchReportsABranchListFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branches = func() ([]string, error) { return nil, errSeam }
	deps.CreateBranch = func(string, string) error { return nil }

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the branch list cannot be read", recorder.Code)
	}
}

func TestCreateBranchRejectsAnEmptyIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := filledDeps()
	deps.CreateBranch = func(string, string) error {
		called = true

		return nil
	}

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, "/api/branches", `{"issue_key":""}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for an empty issue", recorder.Code)
	}

	if called {
		t.Error("created a branch for an empty issue")
	}
}

func TestCreateBranchReportsWhenTheNewBranchCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch read fails, so the base is unknown and the post-create read that
	// would confirm the switch fails too; the request reports the failure.
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	deps.CreateBranch = func(string, string) error { return nil }

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when the new branch cannot be read", recorder.Code)
	}
}

func TestCreateBranchIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Creating a branch needs the create, tracker, and branch-read seams; missing
	// any one means it is not available. filledDeps leaves CreateBranch nil, so the
	// other two cases set it to isolate the seam under test.
	withCreate := func(deps webserver.Deps) webserver.Deps {
		deps.CreateBranch = func(string, string) error { return nil }

		return deps
	}

	cases := map[string]func(webserver.Deps) webserver.Deps{
		"no create seam": func(deps webserver.Deps) webserver.Deps { return deps },
		"no tracker seam": func(deps webserver.Deps) webserver.Deps {
			deps = withCreate(deps)
			deps.Issue = nil

			return deps
		},
		noBranchSeam: func(deps webserver.Deps) webserver.Deps {
			deps = withCreate(deps)
			deps.Branch = nil

			return deps
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := doCreateBranch(t, mutate(filledDeps()))

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422 when creating a branch is not available", recorder.Code)
			}
		})
	}
}
