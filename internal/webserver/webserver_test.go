// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestGetHealthReportsTheBuild(t *testing.T) {
	t.Parallel()

	// Arrange
	dryRun := webserver.Info{Version: testVersion, DryRun: true}

	// Act
	recorder := get(t, serveWith(t, webserver.Deps{}, config.Default(), dryRun), "/api/health")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	health := decode[api.Health](t, recorder)
	if health.Version != testVersion || !health.DryRun {
		t.Errorf("health = %+v, want version 1.2.3 and dry_run true", health)
	}
}

func TestGetHealthNamesTheForgeAndItsOwnWords(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind                forge.Kind
		wantKind            api.HealthForgeKind
		wantNoun, wantSigil string
	}{
		"GitLab": {
			kind: forge.KindGitLab, wantKind: api.HealthForgeKindGitlab, wantNoun: "merge request", wantSigil: "!",
		},
		"GitHub": {
			kind: forge.KindGitHub, wantKind: api.HealthForgeKindGithub, wantNoun: "pull request", wantSigil: "#",
		},
		"a forge unnamed": {
			kind: forge.KindUnknown, wantKind: api.HealthForgeKindUnknown, wantNoun: "pull request", wantSigil: "#",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			info := webserver.Info{Version: testVersion, ForgeKind: tt.kind}

			// Act
			recorder := get(t, serveWith(t, webserver.Deps{}, config.Default(), info), "/api/health")

			// Assert
			health := decode[api.Health](t, recorder)
			if health.ForgeKind != tt.wantKind || health.ForgeNoun != tt.wantNoun || health.ForgeSigil != tt.wantSigil {
				t.Errorf("health = %+v, want forge_kind %q, forge_noun %q and forge_sigil %q",
					health, tt.wantKind, tt.wantNoun, tt.wantSigil)
			}
		})
	}
}

func TestWhatThePageIsToldSaysMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	nothingOpen := func(deps webserver.Deps) webserver.Deps {
		deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
			return forge.PullRequest{}, false, nil
		}

		return deps
	}
	noCommits := func(deps webserver.Deps) webserver.Deps {
		deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
			return gitrepo.Branch{Name: testBranchName, Base: testBase}, nil
		}

		return deps
	}
	reviewersRefused := func(deps webserver.Deps) webserver.Deps {
		deps.Forge.CreatePullRequest = func(context.Context, forge.NewPullRequest) (forge.PullRequest, error) {
			return forge.PullRequest{Number: 7, URL: prURL, Title: prTitle}, forge.ErrRefused
		}

		return deps
	}
	noCreate := func(deps webserver.Deps) webserver.Deps {
		deps.Forge.CreatePullRequest = nil

		return deps
	}
	unchanged := func(deps webserver.Deps) webserver.Deps { return deps }
	linkable := func(deps webserver.Deps) webserver.Deps {
		deps.Jira.LinkPullRequest = func(context.Context, jira.Key, string, string) error { return nil }

		return deps
	}

	// Each case is a request the page makes and the answer it shows, on GitLab.
	const openPath = "/api/pull-request"

	announceBody := `{"channel":"#dev"}`
	cases := map[string]struct {
		mutate       func(webserver.Deps) webserver.Deps
		method, path string
		body         string
	}{
		"nothing to draft":         {noCommits, http.MethodGet, "/api/pull-request/draft", ""},
		"nothing to open":          {noCommits, http.MethodPost, openPath, openRequestBody},
		"opening is not available": {noCreate, http.MethodPost, openPath, openRequestBody},
		"reviewers were not added": {reviewersRefused, http.MethodPost, openPath, openRequestBody},
		"nothing to preview":       {nothingOpen, http.MethodGet, "/api/announcement", ""},
		"nothing to announce":      {nothingOpen, http.MethodPost, "/api/announce", announceBody},
		"linking is not available": {unchanged, http.MethodPost, linkPath, ""},
		"nothing to link":          {linkable, http.MethodPost, linkPath, ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := tt.mutate(openableDeps())
			deps.Messaging.Post = func(context.Context, string, string) error { return nil }
			info := webserver.Info{Version: testVersion, ForgeKind: forge.KindGitLab}

			// Act
			recorder := send(t, serveWith(t, deps, config.Default(), info), tt.method, tt.path, tt.body)

			// Assert
			told := recorder.Body.String()
			if !strings.Contains(told, "merge request") || strings.Contains(told, "pull request") {
				t.Errorf("answer = %q, want it in GitLab's words: a merge request", told)
			}
		})
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
	want := "(sprint in openSprints()) AND assignee = currentUser()"
	if len(views.Views) != 1 || views.Views[0].Name != "Sprint" || views.Views[0].Jql != want {
		t.Errorf("views = %+v, want the configured Sprint view, scoped to you", views.Views)
	}
}

func TestAnErrorIsAnRFC9457Problem(t *testing.T) {
	t.Parallel()

	// Arrange
	// Any failure is answered as application/problem+json with the problem's type,
	// title and status populated — the RFC 9457 shape, not the old code+message.
	deps := filledDeps()
	deps.Jira.Issue = func(context.Context, jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/issues/PROJ-1")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if ct := recorder.Header().Get("Content-Type"); ct != problemJSON {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}

	if failure.Type == "" || failure.Title == "" || failure.Status != http.StatusInternalServerError {
		t.Errorf("problem = %+v, want type, title and status 500 populated", failure)
	}
}
