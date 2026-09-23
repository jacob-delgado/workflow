// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

const (
	// credentialRefused is how Jira's refusal of the configured credential is
	// told.
	credentialRefused = "did not accept the configured credential"
	// undocumentedStatus is how a status the forge does not document is told,
	// whether or not the forge explained it.
	undocumentedStatus = "does not document"
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
			wantStatus: unprocessable, want: credentialRefused,
		},
		"a credential refused": {
			err:        fmt.Errorf("reading https://%s: %w", jiraHost, jira.ErrForbidden),
			wantStatus: unprocessable, want: credentialRefused,
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
			wantStatus: http.StatusBadGateway, want: waitAndTryAgain,
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

func TestAForgeFailureSaysWhatToDo(t *testing.T) {
	t.Parallel()

	// Each failure the forge's client can answer a read with, carrying the host
	// the way a wrapped error can; the answer tells the classes apart — a
	// setting to fix before one to wait out — and never names the host.
	unprocessable, unreachable := http.StatusUnprocessableEntity, http.StatusBadGateway
	cases := map[string]struct {
		cause      error
		wantStatus int
		want       string
	}{
		"no token found":                {forge.ErrNoToken, unprocessable, "no forge token was found"},
		"a kind without its host":       {forge.ErrKindNeedsHost, unprocessable, "forge.kind is set without forge.host"},
		"a token not accepted":          {forge.ErrUnauthorized, unprocessable, "did not accept the token"},
		"no API at the address":         {forge.ErrNoAPI, unprocessable, "no forge API answered"},
		"an answer that is not JSON":    {forge.ErrNotJSON, unprocessable, "no forge API answered"},
		"a refusal that may be a limit": {forge.ErrRefused, unreachable, "wait a minute"},
		"a status it does not document": {forge.ErrUnexpectedStatus, unreachable, undocumentedStatus},
		"a status the forge explained": {
			fmt.Errorf("%w: 500 Internal Server Error", forge.ErrRejected), unreachable, undocumentedStatus,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.FindPull = func(string) (forge.PullRequest, bool, error) {
				return forge.PullRequest{}, false, fmt.Errorf("reading https://%s/api/v3: %w", forgeHost, tt.cause)
			}

			// Act
			recorder := get(t, serve(t, deps, config.Default()), "/api/review")

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != tt.wantStatus || !strings.Contains(failure.Detail, tt.want) {
				t.Errorf("status/detail = %d/%q, want %d saying %q", recorder.Code, failure.Detail, tt.wantStatus, tt.want)
			}

			if strings.Contains(recorder.Body.String(), forgeHost) {
				t.Errorf("body = %q, leaks the forge host", recorder.Body.String())
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

func TestAnAnswerThatCannotBeWrittenSaysWhatToDo(t *testing.T) {
	t.Parallel()

	// Arrange
	// The health answer's body cannot be written, so the safety net answers in
	// its place, opaque about why and plain about what to do.
	writer := &firstWriteFails{}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)
	request.Host = loopbackHost

	// Act
	serve(t, filledDeps(), config.Default()).ServeHTTP(writer, request)

	// Assert
	var failure api.Problem

	err := json.Unmarshal(writer.body.Bytes(), &failure)
	if err != nil || failure.Code != api.Internal || !strings.Contains(failure.Detail, "try again") {
		t.Errorf("answer = %q (%v), want an internal problem that says to try again", writer.body.String(), err)
	}
}

// firstWriteFails fails the first body write, as a connection can, and keeps
// what is written after it.
type firstWriteFails struct {
	header http.Header
	failed bool
	body   bytes.Buffer
}

func (f *firstWriteFails) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}

	return f.header
}

func (f *firstWriteFails) Write(written []byte) (int, error) {
	if !f.failed {
		f.failed = true

		return 0, errSeam
	}

	kept, err := f.body.Write(written)
	if err != nil {
		return kept, fmt.Errorf("keeping the answer: %w", err)
	}

	return kept, nil
}

func (f *firstWriteFails) WriteHeader(int) {}
