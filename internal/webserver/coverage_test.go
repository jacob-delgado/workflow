// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestListIssuesUsesTheNamedView(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotJQL string

	deps := filledDeps()
	deps.Search = func(jql string, _ int) (jira.SearchResult, error) {
		gotJQL = jql

		return jira.SearchResult{}, nil
	}

	cfg := config.Default()
	cfg.Jira.Views = []config.JiraView{{Name: testBugView, JQL: testBugJQL}}

	// Act
	_ = get(t, serve(t, deps, cfg), "/api/issues?view="+testBugView)

	// Assert
	if gotJQL != testBugJQL {
		t.Errorf("searched %q, want the named view's JQL", gotJQL)
	}
}

func TestGetIssueReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestGetBranchReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/branch")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestListChangesIsEmptyWithoutARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Changes = nil

	// Act
	changes := decode[api.ChangeList](t, get(t, serve(t, deps, config.Default()), "/api/changes"))

	// Assert
	if len(changes.Changes) != 0 {
		t.Errorf("changes = %+v, want empty without a repository", changes.Changes)
	}
}

func TestListChangesReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Changes = func() ([]gitrepo.Change, error) { return nil, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/changes")

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestGetMessagingHasNoAuthorWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = nil

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, deps, config.Default()), "/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty without a forge", destination.Author)
	}
}

func TestGetReviewMapsEveryCIState(t *testing.T) {
	t.Parallel()

	// Arrange
	// One CI carrying every state — the overall state and one check per state —
	// so the mapping is exercised for all of them at once.
	deps := filledDeps()
	deps.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{
			State: forge.CIRunning,
			Checks: []forge.Check{
				{Name: "none", State: forge.CINone},
				{Name: "passed", State: forge.CIPassed},
				{Name: "failed", State: forge.CIFailed},
			},
		}, nil
	}

	// Act
	review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

	// Assert
	if review.Ci == nil || review.Ci.State != api.Running || len(review.Ci.Checks) != 3 {
		t.Fatalf("ci = %+v, want state running and three checks", review.Ci)
	}

	want := []api.CIState{api.None, api.Passed, api.Failed}
	for i, check := range review.Ci.Checks {
		if check.State != want[i] {
			t.Errorf("check %d state = %q, want %q", i, check.State, want[i])
		}
	}
}

func TestGetReviewMapsMergeability(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from forge.Mergeability
		want api.PullRequestMergeable
	}{
		"unknown":   {forge.MergeUnknown, api.Unknown},
		"clean":     {forge.MergeClean, api.Clean},
		"conflicts": {forge.MergeConflicts, api.Conflicts},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 1, Mergeable: tt.from}, true, nil
			}

			// Act
			review := decode[api.Review](t, get(t, serve(t, deps, config.Default()), "/api/review"))

			// Assert
			if review.Pull == nil || review.Pull.Mergeable != tt.want {
				t.Errorf("mergeable = %v, want %q", review.Pull, tt.want)
			}
		})
	}
}

func TestUpdateConfigSetsANewSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.Token = "old-secret-0000"

	next := cfg
	next.Jira.Token = "brand-new-secret-1111" // a real new value, neither empty nor the mask

	// Act
	recorder := send(t, serve(t, webserver.Deps{}, cfg), http.MethodPut, "/api/config", marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, err := config.LoadFile(cfg.Path)
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Jira.Token.Reveal() != "brand-new-secret-1111" {
		t.Errorf("saved token = %q, want the new value written", saved.Jira.Token.Reveal())
	}
}

func TestUpdateConfigResolvesHeaders(t *testing.T) {
	t.Parallel()

	// Arrange
	// One header comes back masked (keep the stored value); one comes back with a
	// new value (replace it).
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	cfg.Jira.Headers = map[string]string{"CF-Id": "stored-id-secret", "CF-Team": "stored-team"}

	next := cfg
	next.Jira.Headers = map[string]string{"CF-Id": config.Redact("stored-id-secret"), "CF-Team": "new-team"}

	// Act
	recorder := send(t, serve(t, webserver.Deps{}, cfg), http.MethodPut, "/api/config", marshal(t, next))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	saved, err := config.LoadFile(cfg.Path)
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	if saved.Jira.Headers["CF-Id"] != "stored-id-secret" {
		t.Errorf("CF-Id = %q, want the stored value kept behind the mask", saved.Jira.Headers["CF-Id"])
	}

	if saved.Jira.Headers["CF-Team"] != "new-team" {
		t.Errorf("CF-Team = %q, want the new value written", saved.Jira.Headers["CF-Team"])
	}
}

func TestUpdateConfigReportsASaveFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path whose parent directory does not exist cannot be written.
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), "missing", ".workflow.json")

	// Act
	recorder := send(t, serve(t, webserver.Deps{}, cfg), http.MethodPut, "/api/config", marshal(t, config.Default()))

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the file cannot be written", recorder.Code)
	}
}
