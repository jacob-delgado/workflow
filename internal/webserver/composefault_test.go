// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A read that fails while composing an announcement or a pull request is told
// through fault, not as there being nothing to announce or open.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// route is a request to one endpoint: its method, path and body.
type route struct {
	method, path, body string
}

// announceRoutes are the two requests that compose the announcement: the
// preview, and the post.
func announceRoutes() map[string]route {
	return map[string]route{
		"the preview": {method: http.MethodGet, path: "/api/announcement"},
		"the post":    {method: http.MethodPost, path: "/api/announce", body: `{"channel":"#dev"}`},
	}
}

// pullRoutes are the two requests that compose a pull request: the draft, and
// the open.
func pullRoutes() map[string]route {
	return map[string]route{
		"the draft": {method: http.MethodGet, path: "/api/pull-request/draft"},
		"the open":  {method: http.MethodPost, path: "/api/pull-request", body: openRequestBody},
	}
}

func TestAnUnreachableForgeIsNotNothingToAnnounce(t *testing.T) {
	t.Parallel()

	// The forge's error names where it is, as a client's error does.
	const host = "git.internal.example"

	for name, request := range announceRoutes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			posted := false

			deps := filledDeps()
			deps.Post = func(string, string) error {
				posted = true

				return nil
			}
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{}, false, fmt.Errorf("%w: https://%s", forge.ErrUnreachable, host)
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), request.method, request.path, request.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusBadGateway || failure.Code != api.Unreachable {
				t.Errorf("status = %d, code %q; want 502 and %q", recorder.Code, failure.Code, api.Unreachable)
			}

			if body := recorder.Body.String(); strings.Contains(body, host) {
				t.Errorf("body = %q, names the forge's host", body)
			}

			if posted {
				t.Error("posted an announcement though its pull request could not be read")
			}
		})
	}
}

func TestAFailedBranchReadIsNotNothingToOpen(t *testing.T) {
	t.Parallel()

	// A read that fails says where the repository is, as gitrepo words it.
	readErr := fmt.Errorf("reading the current branch of %s: %w", repoPath, errSeam)

	for name, request := range pullRoutes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			wrote := 0

			deps := openableDeps()
			deps.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, readErr }
			deps.Push = func(string) (proc.Output, error) {
				wrote++

				return fakeOutput(nil, nil), nil
			}
			deps.CreatePull = func(forge.NewPullRequest) (forge.PullRequest, error) {
				wrote++

				return forge.PullRequest{}, nil
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), request.method, request.path, request.body)

			// Assert
			assertGitReadAnswer(t, recorder, wrote, gitReadAnswer{http.StatusInternalServerError, api.Internal, tryAgain})
		})
	}
}
