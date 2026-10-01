// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// An open the forge did not make is answered the way a read's failure is,
// through fault, unless the forge turned the pull request down with its own
// reason or GitLab does not know a reviewer or assignee it names: those words
// are the caller's to act on.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// lacksAScope is how the answer to a token the forge refused names the likely
// cause, in forge.Advice's words.
const lacksAScope = "may lack a scope this needs"

// failingOpenDeps is openableDeps with the branch already pushed, so the
// failure is the open itself, and a create seam that fails with err.
func failingOpenDeps(err error) webserver.Deps {
	deps := openableDeps()
	deps.Branch = func() (gitrepo.Branch, error) { return pushedBranch(), nil }
	deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) { return forge.PullRequest{}, err }

	return deps
}

func TestOpenPullRequestClassifiesAFailedOpenThroughFault(t *testing.T) {
	t.Parallel()

	// The client words these failures with the forge's API base; no answer may
	// repeat it.
	apiBase := "https://" + forgeHost + "/api/v3"
	cases := map[string]struct {
		err        error
		wantStatus int
		wantCode   api.ProblemCode
		wantDetail string
	}{
		"a redirect the client refused": {
			err:        httpx.Unreachable(forge.ErrUnreachable, apiBase, httpx.ErrRedirected),
			wantStatus: http.StatusBadGateway, wantCode: api.Unreachable, wantDetail: "redirect",
		},
		"a forge limiting requests": {
			err:        httpx.RateLimited(http.Header{"Retry-After": {"30"}}),
			wantStatus: http.StatusBadGateway, wantCode: api.Unreachable, wantDetail: waitAndTryAgain,
		},
		"a repository the token cannot see": {
			err:        fmt.Errorf("%w: %s/acme/repo", forge.ErrNoRepository, forgeHost),
			wantStatus: http.StatusNotFound, wantCode: api.NotFound, wantDetail: "not found",
		},
		"a refusal of what the token may do": {
			err:        fmt.Errorf("%w: Resource not accessible by integration", forge.ErrRefused),
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.Unprocessable, wantDetail: lacksAScope,
		},
		"a failure nothing more is known of": {
			err:        fmt.Errorf("opening at %s: %w", apiBase, errSeam),
			wantStatus: http.StatusInternalServerError, wantCode: api.Internal, wantDetail: tryAgain,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := failingOpenDeps(tt.err)

			// Act
			recorder := doOpen(t, deps, openRequestBody)

			// Assert
			failure := decode[api.Problem](t, recorder)

			answered := recorder.Code == tt.wantStatus && failure.Code == tt.wantCode
			if !answered || !strings.Contains(failure.Detail, tt.wantDetail) {
				t.Errorf("status = %d, code %q, detail %q; want %d and %q saying %q",
					recorder.Code, failure.Code, failure.Detail, tt.wantStatus, tt.wantCode, tt.wantDetail)
			}

			if body := recorder.Body.String(); strings.Contains(body, forgeHost) {
				t.Errorf("body = %q, names the forge's host", body)
			}
		})
	}
}

func TestOpenPullRequestKeepsTheWordsTheCallerCanActOn(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"a pull request the forge turned down": fmt.Errorf("%w: A pull request already exists for acme:%s",
			forge.ErrRejected, testBranchName),
		"an assignee GitLab does not know": fmt.Errorf("%w: octocatt", forge.ErrNoUser),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := failingOpenDeps(err)

			// Act
			recorder := doOpen(t, deps, openRequestBody)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != err.Error() {
				t.Errorf("status = %d, detail %q; want 422 saying %q", recorder.Code, failure.Detail, err.Error())
			}
		})
	}
}

func TestARefusedTokenIsToldInTheWordsTheTerminalUses(t *testing.T) {
	t.Parallel()

	// Arrange
	refusal := &forge.RefusalError{
		Kind: forge.KindGitLab, Status: forge.ErrRefused,
		Reason: "insufficient_scope: The request requires higher privileges than provided by the access token. " +
			"(needs the api scope)",
	}
	advice, _ := forge.Advice(refusal)

	// Act
	recorder := doOpen(t, failingOpenDeps(refusal), openRequestBody)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != advice {
		t.Errorf("status/detail = %d/%q, want 422 with the terminal's words %q", recorder.Code, failure.Detail, advice)
	}
}

func TestARefusedTokensDetailNamesNoInternalHost(t *testing.T) {
	t.Parallel()

	// Arrange
	refusal := &forge.RefusalError{
		Kind: forge.KindGitLab, Status: forge.ErrUnauthorized,
		Reason: "access_denied: Sign in at https://sso.corp.internal/login first.",
	}

	// Act
	recorder := doOpen(t, failingOpenDeps(refusal), openRequestBody)

	// Assert
	if strings.Contains(recorder.Body.String(), "sso.corp.internal") {
		t.Errorf("body = %q, names the internal host", recorder.Body.String())
	}
}
