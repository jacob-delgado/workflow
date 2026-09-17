// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// forgeRouting serves a body per request path and records every request, for the
// reads that now make more than one call. A path it does not know answers with
// an empty list, which reads as "nothing there" rather than an error.
func forgeRouting(t *testing.T, routes map[string]string) (forge.Client, *[]recorded) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []recorded
	)

	note := func(request *http.Request) {
		lock.Lock()
		defer lock.Unlock()

		seen = append(seen, recorded{method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		note(request)

		body, ok := routes[request.URL.EscapedPath()]
		if !ok {
			body = "[]"
		}

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret), &seen
}

// requestTo is the first recorded request to a path, or a zero request when none
// reached it.
func requestTo(seen []recorded, path string) recorded {
	for _, request := range seen {
		if request.path == path {
			return request
		}
	}

	return recorded{}
}

func TestFindPullRequestReadsReviewState(t *testing.T) {
	t.Parallel()

	// Arrange
	// ana approved then had it dismissed, so it no longer counts; ben requests
	// changes; cass approves; a comment is not a stance.
	routes := map[string]string{
		githubPullsPath:        `[{"number":9,"html_url":"https://x/9","title":"fix: token","draft":false}]`,
		githubPullsPath + "/9": `{"mergeable":false}`,
		githubPullsPath + "/9/reviews": `[` +
			`{"state":"APPROVED","user":{"login":"ana"}},` +
			`{"state":"DISMISSED","user":{"login":"ana"}},` +
			`{"state":"CHANGES_REQUESTED","user":{"login":"ben"}},` +
			`{"state":"COMMENTED","user":{"login":"dan"}},` +
			`{"state":"APPROVED","user":{"login":"cass"}}]`,
	}

	client, _ := forgeRouting(t, routes)

	// Act
	found, _, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || found.Approvals != 1 || !found.ChangesRequested || found.Mergeable != forge.MergeConflicts {
		t.Errorf("review state = %+v, %v; want 1 approval, changes requested, conflicts", found, err)
	}
}

func TestFindPullRequestToleratesUnreadableReviewState(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only the list answers usefully; the detail and reviews reads get nothing
	// that parses. A pull request found is better than none, so the review fields
	// stay at their zero rather than failing the whole find.
	routes := map[string]string{
		githubPullsPath: `[{"number":3,"html_url":"https://x/3","title":"fix: token","draft":false}]`,
	}

	client, _ := forgeRouting(t, routes)

	// Act
	found, ok, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if err != nil || !ok || found.Approvals != 0 || found.ChangesRequested || found.Mergeable != forge.MergeUnknown {
		t.Errorf("FindPullRequest = %+v, %v, %v; want the pull found with its review state unknown", found, ok, err)
	}
}
