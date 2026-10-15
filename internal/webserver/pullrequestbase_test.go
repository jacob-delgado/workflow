// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// releaseOnlySubject is the one commit the branch holds past release.
const releaseOnlySubject = "fix: only past release"

// ontoReleaseDeps can compose a pull request, and read the branch onto
// release, past which it holds one commit of its own; any other base is none
// the repository holds.
func ontoReleaseDeps() webserver.Deps {
	deps := openableDeps()
	deps.Git.BranchOnto = func(_ context.Context, base string) (gitrepo.Branch, error) {
		if base != "release" {
			return gitrepo.Branch{}, gitrepo.ErrNoSuchBase
		}

		branch := pushedBranch()
		branch.Base = "origin/release"
		branch.Commits = []gitrepo.Commit{{Hash: webtest.TestCommitHash, Subject: releaseOnlySubject}}

		return branch, nil
	}

	return deps
}

func TestGetPullRequestDraftComposesOntoTheBaseItIsGiven(t *testing.T) {
	t.Parallel()

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, ontoReleaseDeps(), config.Default()),
		"/api/pull-request/draft?base=release")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	draft := webtest.Decode[api.PullRequestDraft](t, recorder)
	if draft.Title != releaseOnlySubject || draft.Base != "release" {
		t.Errorf("draft = %+v, want release's own commit as the title, into release", draft)
	}
}

func TestGetPullRequestDraftIsNotFoundOntoABaseNothingHolds(t *testing.T) {
	t.Parallel()

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, ontoReleaseDeps(), config.Default()),
		"/api/pull-request/draft?base=nonesuch")

	// Assert
	problem := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || problem.Code != api.ProblemCodeNotFound {
		t.Errorf("status = %d, problem %+v; want a 404 not_found", recorder.Code, problem)
	}
}

func TestGetPullRequestDraftIsNotFoundOntoABaseGitWouldReadAsAnOption(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := ontoReleaseDeps()
	deps.Git.BranchOnto = func(context.Context, string) (gitrepo.Branch, error) {
		return gitrepo.Branch{}, gitrepo.ErrOptionLikeRef
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/pull-request/draft?base=-x")

	// Assert
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}
