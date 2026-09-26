// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The 409 a pull request's draft or open is refused with says why: the pull
// request that is already open, or that there is no work to propose.

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
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
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{Number: 42, URL: prURL}, true, nil
			}
			handler := serveWith(t, deps, config.Default(), webserver.Info{Version: testVersion, ForgeKind: tt.kind})

			// Act
			recorder := send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)

			// Assert
			failure := decode[api.Problem](t, recorder)
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
	deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: testBranchName, Base: testBase}, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/pull-request/draft")

	// Assert
	const want = "there is no branch with commits to open a pull request for"
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict || failure.Detail != want {
		t.Errorf("status = %d, detail %q; want 409 saying %q", recorder.Code, failure.Detail, want)
	}
}
