// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// errFixtureTimeout stands in for whatever the transport failed with.
var errFixtureTimeout = errors.New("i/o timeout")

// searchBody is shaped like a real Data Center search answer: two issues, and a
// total larger than the page, because a working list is capped.
const searchBody = `{"startAt":0,"maxResults":50,"total":73,"issues":[` +
	`{"key":"OPS-1","fields":{"summary":"Fix login",` +
	`"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}}}},` +
	`{"key":"OPS-2","fields":{"summary":"Rotate keys",` +
	`"status":{"name":"To Do","statusCategory":{"key":"new"}}}}]}`

// answer writes a JSON body, optionally naming the user Jira thinks it served.
func answer(body, user string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if user != "" {
			writer.Header().Set("X-Ausername", user)
		}

		writer.Header().Set("Content-Type", jsonMediaType)

		_, _ = writer.Write([]byte(body))
	}
}

func TestSearchReadsTheIssuesAndTheTotal(t *testing.T) {
	t.Parallel()

	result, err := serve(t, answer(searchBody, "fred")).Search(t.Context(), jira.AssignedToMe)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	want := []jira.Issue{
		{Key: "OPS-1", Summary: "Fix login", Status: "In Progress", StatusCategory: "indeterminate"},
		{Key: "OPS-2", Summary: "Rotate keys", Status: "To Do", StatusCategory: "new"},
	}

	if len(result.Issues) != len(want) {
		t.Fatalf("got %d issues, want %d", len(result.Issues), len(want))
	}

	for index, issue := range want {
		if result.Issues[index] != issue {
			t.Errorf("issue %d = %+v, want %+v", index, result.Issues[index], issue)
		}
	}

	// The page is capped, so the total is what says there is more.
	if result.Total != 73 {
		t.Errorf("Total = %d, want 73", result.Total)
	}
}

func TestSearchAsksForOnlyWhatTheListShows(t *testing.T) {
	t.Parallel()

	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Query())
		answer(searchBody, "fred")(writer, request)
	})

	_, err := client.Search(t.Context(), jira.AssignedToMe)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	values, _ := query.Load().(url.Values)

	if values.Get("jql") != jira.AssignedToMe {
		t.Errorf("jql = %q, want %q", values.Get("jql"), jira.AssignedToMe)
	}

	if values.Get("fields") != "summary,status" {
		t.Errorf("fields = %q, want only what a row and the detail display", values.Get("fields"))
	}

	if values.Get("maxResults") == "" {
		t.Error("no maxResults was sent; the server's default is not a promise")
	}
}

func TestSearchTreatsAnAnonymousAnswerAsARejectedCredential(t *testing.T) {
	t.Parallel()

	// A bearer token Data Center does not accept is not refused on search: it
	// is served ANONYMOUSLY, with 200 and an empty list. Without this check a
	// dead token reads exactly like "nothing is assigned to you".
	_, err := serve(t, answer(`{"total":0,"issues":[]}`, "anonymous")).Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Search returned %v, want ErrUnauthorized", err)
	}
}

func TestSearchTrustsAnAnswerWithoutTheUserHeader(t *testing.T) {
	t.Parallel()

	// The header is undocumented. Its absence is not evidence of anything, so a
	// proxy that strips it must not turn every answer into an error.
	result, err := serve(t, answer(searchBody, "")).Search(t.Context(), jira.AssignedToMe)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	if len(result.Issues) != 2 {
		t.Errorf("got %d issues, want 2", len(result.Issues))
	}
}

func TestSearchReportsAnUnreachableServerByItsCause(t *testing.T) {
	t.Parallel()

	// net/http's *url.Error quotes the whole request URL — here about 180
	// characters of encoded JQL — ahead of the cause. The pane clips each line,
	// so "i/o timeout" would be lost behind the query.
	failing := func(request *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: request.URL.String(), Err: errFixtureTimeout}
	}

	client := jira.New(failing, bearerConfig(exampleBaseURL))

	_, err := client.Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrUnreachable) || !errors.Is(err, errFixtureTimeout) {
		t.Fatalf("Search returned %v, want ErrUnreachable wrapping the cause", err)
	}

	if strings.Contains(err.Error(), "jql") || strings.Contains(err.Error(), "currentUser") {
		t.Errorf("the error quoted the query ahead of its cause: %v", err)
	}

	if strings.Contains(err.Error(), token) {
		t.Errorf("the error carried the token: %v", err)
	}
}

func TestSearchReportsAPlainTransportError(t *testing.T) {
	t.Parallel()

	failing := func(*http.Request) (*http.Response, error) {
		return nil, errFixtureTimeout
	}

	_, err := jira.New(failing, bearerConfig(exampleBaseURL)).Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrUnreachable) || !errors.Is(err, errFixtureTimeout) {
		t.Errorf("Search returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestSearchWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	var sent atomic.Bool

	recording := func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errFixtureTimeout
	}

	client := jira.New(recording, config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

	_, err := client.Search(t.Context(), jira.AssignedToMe)
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Search returned %v, want ErrNoCredential", err)
	}

	if sent.Load() {
		t.Error("a request went out with no credential to send")
	}
}

func TestSearchReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	_, err := serve(t, answer("{not json", "fred")).Search(t.Context(), jira.AssignedToMe)
	if err == nil {
		t.Error("Search accepted a malformed body, want an error")
	}
}
