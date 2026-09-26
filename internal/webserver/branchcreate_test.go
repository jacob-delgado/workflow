// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

	return convention.NewBranchNaming(cfg.Branch.Template, cfg.Branch.DefaultPrefix,
		cfg.Branch.Prefixes, cfg.Branch.SlugLimit).
		Name("Bug", startIssue, "Fix token redaction")
}

// assertCreateBranchSaysTryAgain checks a start-work request answered 422, with
// a detail that names the issue and what to do next.
func assertCreateBranchSaysTryAgain(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	want := "the branch for " + startIssue + " could not be created; try again, or run workflow branch " +
		startIssue + " from a terminal to see why"

	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != want {
		t.Errorf("status = %d, detail %q; want 422 saying %q", recorder.Code, failure.Detail, want)
	}
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

	if failure := decode[api.Problem](t, recorder); failure.Code != api.Conflict {
		t.Errorf("code = %q, want conflict", failure.Code)
	}
}

func TestCreateBranchSaysWhyTheIssueCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Without the issue's type and summary the branch cannot be named. The
	// tracker's failure is answered as fault tells its class — the error carries
	// the tracker's address the way its client words it — and never names the
	// host.
	cases := map[string]struct {
		cause      error
		wantStatus int
		want       string
	}{
		"a credential not accepted": {
			cause: jira.ErrUnauthorized, wantStatus: http.StatusUnprocessableEntity,
			want: "Jira did not accept the configured credential",
		},
		"no answer": {
			cause: jira.ErrUnreachable, wantStatus: http.StatusBadGateway,
			want: "could not be reached",
		},
		"no such issue": {
			cause: jira.ErrNotFound, wantStatus: http.StatusNotFound,
			want: "was not found",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Issue = func(jira.Key) (jira.IssueDetail, error) {
				return jira.IssueDetail{}, fmt.Errorf("reading https://%s/rest/api/2/issue/%s: %w",
					jiraHost, startIssue, tt.cause)
			}
			deps.CreateBranch = func(string, string) error { return nil }

			// Act
			recorder := doCreateBranch(t, deps)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.wantStatus || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status = %d, detail %q; want %d saying %q", recorder.Code, failure.Detail, tt.wantStatus, tt.want)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the tracker's host", recorder.Body.String())
			}
		})
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
	assertCreateBranchSaysTryAgain(t, recorder)
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

func TestCreateBranchSucceedsEvenIfTheRereadFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is created and checked out, but the read that would confirm it
	// fails; the created branch is answered by its name rather than told as a
	// failure a retry would then find already done.
	created := false
	deps := filledDeps()
	branch := deps.Branch
	deps.Branch = func() (gitrepo.Branch, error) {
		if created {
			return gitrepo.Branch{}, errSeam
		}

		return branch()
	}
	deps.CreateBranch = func(string, string) error {
		created = true

		return nil
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the branch was created despite the re-read failing", recorder.Code)
	}

	if answered := decode[api.Branch](t, recorder); answered.Name != wantBranchName(t) {
		t.Errorf("branch = %+v, want the created branch %q", answered, wantBranchName(t))
	}
}

func TestCreateBranchAnswersTheCreatedNameWhenNoBranchCanBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every branch read fails, so the base is unknown and the new branch comes off
	// HEAD; the branch as read before cannot stand in, and the created name does.
	var created string

	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }
	deps.CreateBranch = func(name, _ string) error {
		created = name

		return nil
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a branch that was created", recorder.Code)
	}

	if answered := decode[api.Branch](t, recorder); created == "" || answered.Name != created {
		t.Errorf("branch = %+v, created %q; want the created branch answered", answered, created)
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

func TestCreateBranchNeverForwardsGitsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// Switching to the new branch checks its tree out, which in a partial clone
	// fetches from the remote, so git's own words can name it; the detail names
	// the issue and what to do, and never the host.
	deps := filledDeps()
	deps.CreateBranch = func(name, _ string) error {
		return fmt.Errorf("creating branch %s: %w: fatal: unable to access 'https://%s/acme/repo.git/'",
			name, errSeam, gitHost)
	}

	// Act
	recorder := doCreateBranch(t, deps)

	// Assert
	want := "git would not create the branch for " + startIssue +
		"; run workflow branch " + startIssue + " from a terminal to see git's reason"

	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != want {
		t.Errorf("status = %d, detail %q; want 422 saying %q", recorder.Code, failure.Detail, want)
	}

	if strings.Contains(recorder.Body.String(), gitHost) {
		t.Errorf("body = %q, leaks the remote's host", recorder.Body.String())
	}
}
