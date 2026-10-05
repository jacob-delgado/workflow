// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The api repository's worktrees beside its root.
const (
	apiFeature   = "/home/ana/src/api-feat-x"
	apiGone      = "/home/ana/src/api-gone"
	worktreeHead = "300a7be68d29eb9302518fd80f1cf20267d6feba"
	worktreesAt  = "/api/worktrees"
	shortHead    = "300a7be"
)

var errWorktreeList = errors.New("fatal: not a git repository: /home/ana/src/api/.git/worktrees/x")

// worktreeDeps is repositoryDeps working in api, whose worktrees are its
// root, a feature branch that git keeps locked, and one whose directory is
// gone.
func worktreeDeps() webserver.Deps {
	deps := repositoryDeps(favoritesKept())
	deps.Repositories.Worktrees = func() ([]gitrepo.Worktree, error) {
		return []gitrepo.Worktree{
			{Dir: apiRoot, Branch: prBase, Head: worktreeHead},
			{Dir: apiFeature, Branch: "feat/x", Head: worktreeHead, Locked: true},
			{Dir: apiGone, Head: worktreeHead, Detached: true, Missing: true},
		}, nil
	}

	return deps
}

func TestRepositoriesListTheWorktrees(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, worktreeDeps(), config.Default()), reposPath)

	// Assert
	want := []api.Worktree{
		{Dir: apiRoot, Shown: "~/src/api", Branch: prBase, Head: shortHead, State: api.WorktreeHere},
		{
			Dir: apiFeature, Shown: "~/src/api-feat-x", Branch: "feat/x", Head: shortHead, State: api.WorktreeOther,
			Locked: true,
		},
		{Dir: apiGone, Shown: "~/src/api-gone", Branch: "", Head: shortHead, State: api.WorktreeMissing},
	}

	got := decode[api.Repositories](t, recorder)
	if recorder.Code != http.StatusOK || !slices.Equal(got.Worktrees, want) || got.WorktreesError != "" {
		t.Errorf("status %d, worktrees %+v (%q); want %+v", recorder.Code, got.Worktrees, got.WorktreesError, want)
	}
}

func TestOutsideARepositoryThereAreNoWorktrees(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, repositoryDeps(favoritesKept()), config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)
	if got.Worktrees == nil || len(got.Worktrees) != 0 || got.WorktreesError != "" {
		t.Errorf("worktrees %+v (%q), want an empty list and no error", got.Worktrees, got.WorktreesError)
	}
}

func TestWorktreesThatCannotBeReadSayWhyWithoutAPath(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := worktreeDeps()
	deps.Repositories.Worktrees = func() ([]gitrepo.Worktree, error) {
		return nil, fmt.Errorf("%w: %w", gitrepo.ErrNotARepository, errWorktreeList)
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)
	if recorder.Code != http.StatusOK || got.WorktreesError == "" || strings.Contains(got.WorktreesError, "/home") {
		t.Errorf("status %d, worktrees error %q; want why, without a path", recorder.Code, got.WorktreesError)
	}
}

// postWorktree asks the server over deps to start work on startIssue in a
// new worktree.
func postWorktree(t *testing.T, deps webserver.Deps, info webserver.Info) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, serveWith(t, deps, config.Default(), info), http.MethodPost, worktreesAt,
		`{"issue_key":"`+startIssue+`"}`)
}

func TestCreateWorktreeStartsWorkOnTheIssueBesideTheRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	var made, from string

	deps := worktreeDeps()
	deps.CreateWorktree = func(name, start string) (string, error) {
		made, from = name, start

		return apiRoot + "-" + strings.ReplaceAll(name, "/", "-"), nil
	}

	// Act
	recorder := postWorktree(t, deps, webserver.Info{Version: testVersion})

	// Assert
	got := decode[api.CreatedWorktree](t, recorder)
	wantDir := apiRoot + "-" + strings.ReplaceAll(wantBranchName(t), "/", "-")

	if recorder.Code != http.StatusOK || made != wantBranchName(t) || from != "origin/main" ||
		got.Dir != wantDir || got.Branch != wantBranchName(t) || !strings.HasPrefix(got.Shown, "~/src/api-") {
		t.Errorf("status %d, made %q from %q, answered %+v; want %s's branch from origin/main at %s",
			recorder.Code, made, from, got, startIssue, wantDir)
	}
}

func TestCreateWorktreeRefusesWhenTheBranchExists(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := worktreeDeps()
	deps.Branches = func() ([]string, error) { return []string{wantBranchName(t)}, nil }
	deps.CreateWorktree = func(string, string) (string, error) {
		called = true

		return "", nil
	}

	// Act
	recorder := postWorktree(t, deps, webserver.Info{Version: testVersion})

	// Assert
	if recorder.Code != http.StatusConflict || called {
		t.Errorf("status %d, created %v; want 409 and nothing made", recorder.Code, called)
	}
}

func TestCreateWorktreeNeverForwardsGitsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := worktreeDeps()
	deps.CreateWorktree = func(string, string) (string, error) { return "", errWorktreeList }

	// Act
	recorder := postWorktree(t, deps, webserver.Info{Version: testVersion})

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(failure.Detail, "/home") {
		t.Errorf("status %d, detail %q; want 422 in fixed words", recorder.Code, failure.Detail)
	}
}

func TestCreateWorktreeIsUnavailableWithoutAGitSeam(t *testing.T) {
	t.Parallel()

	// Act
	recorder := postWorktree(t, repositoryDeps(favoritesKept()), webserver.Info{Version: testVersion})

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422 when no worktree can be made", recorder.Code)
	}
}

func TestCreateWorktreeIsRefusedInDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := worktreeDeps()
	deps.CreateWorktree = func(string, string) (string, error) {
		called = true

		return "", nil
	}

	// Act
	recorder := postWorktree(t, deps, webserver.Info{Version: testVersion, DryRun: true})

	// Assert
	if recorder.Code != http.StatusForbidden || called {
		t.Errorf("status %d, created %v; want 403 and nothing made", recorder.Code, called)
	}
}

func TestASnapshotBranchSaysWhichOtherWorktreeHasItCheckedOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// git refuses to check out a branch another worktree has, so the page
	// offers a switch there instead, and needs to know where it is.
	deps := worktreeDeps()
	deps.Branches = func() ([]string, error) { return []string{testBranchName, targetBranch}, nil }
	deps.Repositories.Worktrees = func() ([]gitrepo.Worktree, error) {
		return []gitrepo.Worktree{
			{Dir: apiRoot, Branch: testBranchName, Head: worktreeHead},
			{Dir: apiFeature, Branch: targetBranch, Head: worktreeHead},
		}, nil
	}
	cfg := config.Default()
	cfg.Jira.Project = testProject

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, cfg), "/api/events").Body.String())

	// Assert
	worktrees := map[string]string{}
	for _, branch := range snap.Branches {
		worktrees[branch.Name] = valueOf(branch.Worktree) + " " + valueOf(branch.WorktreeShown)
	}

	want := map[string]string{testBranchName: " ", targetBranch: apiFeature + " ~/src/api-feat-x"}
	if !maps.Equal(worktrees, want) {
		t.Errorf("worktrees by branch = %q, want %q: the one here none, the other its own", worktrees, want)
	}
}

// valueOf is what an optional string holds, or "" when it holds nothing.
func valueOf(optional *string) string {
	if optional == nil {
		return ""
	}

	return *optional
}
