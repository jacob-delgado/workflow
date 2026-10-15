// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The 409 a pull request's draft or open is refused with says why: the pull
// request that is already open, or that there is no work to propose.

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestOpenPullRequestIsAConflictWhenOneIsAlreadyOpen(t *testing.T) {
	t.Parallel()

	// The open one is named the way its forge marks a number.
	cases := map[string]struct {
		kind       forge.Kind
		wantDetail string
	}{
		"on GitHub": {forge.KindGitHub, "#42 is already open for this branch"},
		"on GitLab": {forge.KindGitLab, "!42 is already open for this branch"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := openableDeps()
			deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 42, URL: webtest.PRURL}, true, nil
			}
			handler := webtest.ServeWith(t, deps, config.Default(), webserver.Info{
				Version:   webtest.TestVersion,
				ForgeKind: tt.kind,
			})

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusConflict || failure.Detail != tt.wantDetail {
				t.Errorf("status = %d, detail %q; want 409 saying %q", recorder.Code, failure.Detail, tt.wantDetail)
			}
		})
	}
}

func TestGetPullRequestDraftSaysThereIsNoWorkToPropose(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := openableDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{Name: webtest.TestBranchName, Base: webtest.TestBase}, nil
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	const want = "there is no branch with commits to open a pull request for"
	if failure := webtest.Decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict ||
		failure.Detail != want {
		t.Errorf("status = %d, detail %q; want 409 saying %q", recorder.Code, failure.Detail, want)
	}
}
