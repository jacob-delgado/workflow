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
