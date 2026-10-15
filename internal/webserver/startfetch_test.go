// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// fetchHost is where origin is, which a failed fetch's error names and no
// answer may repeat.
const fetchHost = "git.internal.example"

// errFetchTimedOut is what stopped the fetch.
var errFetchTimedOut = errors.New("timeout")

// errFetch is a fetch git could not make, naming where origin is.
var errFetch = fmt.Errorf("fatal: unable to access 'https://%s/acme.git/': %w", fetchHost, errFetchTimedOut)

// startCalls records, in order, the fetch and the creates a start of work made.
type startCalls struct {
	calls []string
}

// wire binds deps' fetch, branch and worktree creates over the calls, the
// fetch answering fetchErr.
func (c *startCalls) wire(deps webserver.Deps, fetchErr error) webserver.Deps {
	deps.Git.Fetch = func(context.Context) error {
		c.calls = append(c.calls, "fetch")

		return fetchErr
	}
	deps.Git.CreateBranch = func(_ context.Context, name, _ string) error {
		c.calls = append(c.calls, "branch "+name)

		return nil
	}
	deps.Git.CreateWorktree = func(_ context.Context, name, _ string) (string, error) {
		c.calls = append(c.calls, "worktree "+name)

		return "/home/dev/src/acme-" + name, nil
	}

	return deps
}

// startWorkPaths are the two ways to start work, each fetching first.
func startWorkPaths() map[string]string {
	return map[string]string{"here": branchesAt, "in a new worktree": "/api/worktrees"}
}

func TestStartWorkFetchesBeforeItCreates(t *testing.T) {
	t.Parallel()

	for name, path := range startWorkPaths() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &startCalls{}
			handler := webtest.Serve(t, calls.wire(webtest.FilledDeps(), nil), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, path, `{"issue_key":"`+startIssue+`"}`)

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
			}

			if len(calls.calls) != 2 || calls.calls[0] != "fetch" {
				t.Errorf("calls = %v, want a fetch, then the create", calls.calls)
			}
		})
	}
}

func TestStartWorkThatCannotFetchCreatesNothingAndSaysSo(t *testing.T) {
	t.Parallel()

	for name, path := range startWorkPaths() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &startCalls{}
			handler := webtest.Serve(t, calls.wire(webtest.FilledDeps(), errFetch), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, path, `{"issue_key":"`+startIssue+`"}`)

			// Assert
			failure := assertProblem(t, recorder, http.StatusBadGateway, "the base remote could not be fetched")
			if failure.Code != api.ProblemCodeFetchFailed {
				t.Errorf("code = %q, want %q, which the page offers to branch from what you have on",
					failure.Code, api.ProblemCodeFetchFailed)
			}

			if strings.Contains(recorder.Body.String(), fetchHost) {
				t.Errorf("the refusal %s names origin's host", recorder.Body)
			}

			if !slices.Equal(calls.calls, []string{"fetch"}) {
				t.Errorf("calls = %v, want the fetch alone", calls.calls)
			}
		})
	}
}

func TestStartWorkFromWhatYouHaveFetchesNothing(t *testing.T) {
	t.Parallel()

	for name, path := range startWorkPaths() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			calls := &startCalls{}
			handler := webtest.Serve(t, calls.wire(webtest.FilledDeps(), errFetch), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, path, `{"issue_key":"`+startIssue+`","fetch":false}`)

			// Assert
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
			}

			if slices.Contains(calls.calls, "fetch") {
				t.Errorf("calls = %v, want no fetch", calls.calls)
			}
		})
	}
}

func TestStartWorkWithNoBaseHasNothingToFetch(t *testing.T) {
	t.Parallel()

	// Arrange
	calls := &startCalls{}
	deps := calls.wire(webtest.FilledDeps(), errFetch)
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{Name: webtest.PRBase}, nil }

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, "/api/branches",
		`{"issue_key":"`+startIssue+`"}`)

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
	}

	if slices.Contains(calls.calls, "fetch") {
		t.Errorf("calls = %v, want no fetch with no base to refresh", calls.calls)
	}
}
