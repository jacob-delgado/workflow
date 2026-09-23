// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestTransitionNeverForwardsTheJiraHost(t *testing.T) {
	t.Parallel()

	// Each failure Jira's client can answer a move with, carrying the host the
	// way the client words it; the answer says what to do, in words that tell
	// the classes apart, and never names the host.
	unprocessable := http.StatusUnprocessableEntity
	cases := map[string]struct {
		err        error
		wantStatus int
		want       string
	}{
		"a refusal": {
			err:        fmt.Errorf("%w: the workflow at https://%s forbids it", jira.ErrRejected, jiraHost),
			wantStatus: unprocessable, want: "Jira refused",
		},
		"no API at the address": {
			err:        fmt.Errorf("%w: https://%s/rest/api/2/issue/PROJ-412/transitions", jira.ErrNoAPI, jiraHost),
			wantStatus: unprocessable, want: "no Jira API answers",
		},
		"a credential not accepted": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrUnauthorized),
			wantStatus: unprocessable, want: "did not accept the configured credential",
		},
		"a credential refused": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrForbidden),
			wantStatus: unprocessable, want: "did not accept the configured credential",
		},
		// The client refuses these three before it asks Jira anything, so the
		// answer names what is missing or unusable, not what Jira did.
		"no token configured": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrNoCredential),
			wantStatus: unprocessable, want: "no Jira token is configured",
		},
		"a base URL that is no address": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrInvalidBaseURL),
			wantStatus: unprocessable, want: "jira.base_url is not a usable address",
		},
		"a base URL holding a password": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrCredentialInBaseURL),
			wantStatus: unprocessable, want: "jira.base_url is not a usable address",
		},
		"asked to wait": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, httpx.RateLimited(http.Header{"Retry-After": {"30"}})),
			wantStatus: http.StatusBadGateway, want: "wait and try again",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := writableDeps(new([]linkCall), new([]jira.Transition))
			deps.Transition = func(jira.Key, jira.Transition, []jira.FieldValue) error { return tt.err }

			// Act
			recorder := post(t, deps, reviewConfig(), movePath)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.wantStatus || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status/detail = %d/%q, want %d saying %q", recorder.Code, failure.Detail, tt.wantStatus, tt.want)
			}

			if strings.Contains(recorder.Body.String(), jiraHost) {
				t.Errorf("body = %q, leaks the Jira host", recorder.Body.String())
			}
		})
	}
}

func TestAMissingIssueIsNotFoundBeforeARefusal(t *testing.T) {
	t.Parallel()

	// Jira answers a missing issue with a 404 that gives its reason, which the
	// client reports as both a missing resource and a refusal: missing wins.
	missing := fmt.Errorf("%w: %w", jira.ErrNotFound, fmt.Errorf("%w: Issue Does Not Exist", jira.ErrRejected))
	cases := map[string]struct {
		unwire func(*webserver.Deps)
		path   string
	}{
		"reading the moves": {
			unwire: func(deps *webserver.Deps) {
				deps.Transitions = func(jira.Key) ([]jira.Transition, error) { return nil, missing }
			},
			path: movePath,
		},
		"the link": {
			unwire: func(deps *webserver.Deps) {
				deps.LinkPullRequest = func(jira.Key, string, string) error { return missing }
			},
			path: linkPath,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := writableDeps(new([]linkCall), new([]jira.Transition))
			tt.unwire(&deps)

			// Act
			recorder := post(t, deps, reviewConfig(), tt.path)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusNotFound || failure.Code != api.NotFound {
				t.Errorf("status/code = %d/%s, want 404/not_found", recorder.Code, failure.Code)
			}
		})
	}
}
