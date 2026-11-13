// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
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

func TestSearchPagesFromStartAt(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.RawQuery)
		writer.Header().Set("Content-Type", jsonMediaType)
		_, _ = writer.Write([]byte(searchBody))
	})

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 50)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if got, _ := query.Load().(string); !strings.Contains(got, "startAt=50") {
		t.Errorf("query = %q, want it to page from startAt=50", got)
	}
}

func TestSearchReadsTheIssuesAndTheTotal(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer(searchBody, "fred"))

	// Act
	result, err := client.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	want := []jira.Issue{
		{Key: "OPS-1", Summary: "Fix login", Status: inProgress, StatusCategory: indeterminate},
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

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Query())
		answer(searchBody, "fred")(writer, request)
	})

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	values, _ := query.Load().(url.Values)

	if values.Get("jql") != jira.AssignedToMe {
		t.Errorf("jql = %q, want %q", values.Get("jql"), jira.AssignedToMe)
	}

	if values.Get("fields") != "summary,status,issuetype,priority" {
		t.Errorf("fields = %q, want only what a row and the detail display", values.Get("fields"))
	}

	if values.Get("maxResults") == "" {
		t.Error("no maxResults was sent; the server's default is not a promise")
	}
}

func TestSearchTreatsAnAnonymousAnswerAsARejectedCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	// A bearer token Data Center does not accept is not refused on search: it
	// is served ANONYMOUSLY, with 200 and an empty list. Without this check a
	// dead token reads exactly like "nothing is assigned to you".
	client := serve(t, answer(`{"total":0,"issues":[]}`, "anonymous"))

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Search returned %v, want ErrUnauthorized", err)
	}
}

func TestSearchTrustsAnAnswerWithoutTheUserHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	// The header is undocumented. Its absence is not evidence of anything, so a
	// proxy that strips it must not turn every answer into an error.
	client := serve(t, answer(searchBody, ""))

	// Act
	result, err := client.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if len(result.Issues) != 2 {
		t.Errorf("got %d issues, want 2", len(result.Issues))
	}
}

func TestSearchReportsAnUnreachableServerByItsCause(t *testing.T) {
	t.Parallel()

	// Arrange
	// net/http's *url.Error quotes the whole request URL — here about 180
	// characters of encoded JQL — ahead of the cause. The pane clips each line,
	// so "i/o timeout" would be lost behind the query.
	failing := func(request *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: request.URL.String(), Err: errFixtureTimeout}
	}

	client := jira.New(failing, bearerConfig(exampleBaseURL))

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
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

	// Arrange
	failing := func(*http.Request) (*http.Response, error) {
		return nil, errFixtureTimeout
	}

	client := jira.New(failing, bearerConfig(exampleBaseURL))

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if !errors.Is(err, jira.ErrUnreachable) || !errors.Is(err, errFixtureTimeout) {
		t.Errorf("Search returned %v, want ErrUnreachable wrapping the cause", err)
	}
}

func TestSearchWithoutACredentialSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	recording := func(*http.Request) (*http.Response, error) {
		sent.Store(true)

		return nil, errFixtureTimeout
	}

	client := jira.New(recording, config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Search returned %v, want ErrNoCredential", err)
	}

	if sent.Load() {
		t.Error("a request went out with no credential to send")
	}
}

func TestSearchReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer("{not json", "fred"))

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("Search returned %v, want the malformed body's syntax error", err)
	}
}

// followed is the issue the page after a cursor follows.
const followed = "OPS-8"

// pageBody is a search answer holding the issues keys name, out of total.
func pageBody(total int, keys ...string) string {
	rows := make([]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, `{"key":"`+key+`","fields":{"summary":"",`+
			`"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	}

	return `{"total":` + strconv.Itoa(total) + `,"issues":[` + strings.Join(rows, ",") + `]}`
}

func TestSearchNamesTheCursorAfterItsLastIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer(pageBody(73, "OPS-7", followed), "fred"))

	// Act
	result, err := client.Search(t.Context(), jira.AssignedToMe, 50)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if want := (jira.Cursor{After: followed, At: 51}); result.Next != want {
		t.Errorf("Next = %+v, want %+v: the page after follows OPS-8, listed at 51", result.Next, want)
	}
}

func TestACursorNamingAnIssueHasMore(t *testing.T) {
	t.Parallel()

	// Arrange
	cursor := jira.Cursor{After: followed, At: 51}

	// Act & Assert
	if !cursor.More() {
		t.Errorf("More() = false for %+v, want true: it names an issue for a page to follow", cursor)
	}
}

func TestSearchThatEndsTheListNamesNoCursor(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer(pageBody(52, "OPS-7", followed), "fred"))

	// Act
	result, err := client.Search(t.Context(), jira.AssignedToMe, 50)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if result.Next.More() {
		t.Errorf("Next = %+v, want none: the page ends the list", result.Next)
	}
}

func TestSearchAfterAsksFromTheIssueItFollows(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Query())
		answer(pageBody(73, followed, "OPS-9"), "fred")(writer, request)
	})

	// Act
	result, err := client.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{After: followed, At: 51})
	if err != nil {
		t.Fatalf("SearchAfter returned %v, want nil", err)
	}

	// Assert
	values, _ := query.Load().(url.Values)
	if values.Get("startAt") != "51" {
		t.Errorf("startAt = %q, want 51, where the issue it follows was listed", values.Get("startAt"))
	}

	if got := issueKeys(result.Issues); got != "OPS-9" {
		t.Errorf("issues = %s, want OPS-9 alone: what follows OPS-8", got)
	}
}

func TestSearchAfterFollowsItsIssueWhereverItMovedInThePage(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two issues joined the list ahead of OPS-8 since its page was read.
	client := serve(t, answer(pageBody(75, "OPS-1", "OPS-2", followed, "OPS-9"), "fred"))

	// Act
	result, err := client.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{After: followed, At: 51})
	if err != nil {
		t.Fatalf("SearchAfter returned %v, want nil", err)
	}

	// Assert
	if got := issueKeys(result.Issues); got != "OPS-9" {
		t.Errorf("issues = %s, want OPS-9 alone: what follows OPS-8 where it is now", got)
	}
}

func TestSearchAfterAnIssueGoneStartsWhereItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	// OPS-8 left the list, so what followed it moved up into its place.
	client := serve(t, answer(pageBody(72, "OPS-9", "OPS-10"), "fred"))

	// Act
	result, err := client.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{After: followed, At: 51})
	if err != nil {
		t.Fatalf("SearchAfter returned %v, want nil", err)
	}

	// Assert
	if got := issueKeys(result.Issues); got != "OPS-9 OPS-10" {
		t.Errorf("issues = %s, want OPS-9 OPS-10: what followed OPS-8, from where it was", got)
	}
}

func TestSearchAfterNoCursorIsTheFirstPage(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Query())
		answer(pageBody(2, "OPS-1", "OPS-2"), "fred")(writer, request)
	})

	// Act
	result, err := client.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{})
	if err != nil {
		t.Fatalf("SearchAfter returned %v, want nil", err)
	}

	// Assert
	values, _ := query.Load().(url.Values)
	if values.Get("startAt") != "0" || issueKeys(result.Issues) != "OPS-1 OPS-2" {
		t.Errorf("startAt = %q and issues = %s, want the first page whole", values.Get("startAt"), issueKeys(result.Issues))
	}
}

func TestSearchAfterReportsAFailedRead(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer("{not json", "fred"))

	// Act
	_, err := client.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{After: followed, At: 51})

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("SearchAfter returned %v, want the malformed body's syntax error", err)
	}
}

// issueKeys is the keys of issues, space-separated.
func issueKeys(issues []jira.Issue) string {
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, string(issue.Key))
	}

	return strings.Join(keys, " ")
}

func TestACursorReadsBackFromItsText(t *testing.T) {
	t.Parallel()

	// Arrange
	cursor := jira.Cursor{After: followed, At: 51}

	// Act
	read, err := jira.ParseCursor(cursor.String())

	// Assert
	if err != nil || read != cursor {
		t.Errorf("ParseCursor(%q) = %+v, %v, want %+v", cursor.String(), read, err, cursor)
	}
}

func TestParseCursorRefusesTextNoPageGave(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"OPS-8", "@51", "OPS-8@", "OPS-8@-1", "OPS-8@x", "OPS-8@99999999999999999999",
		"OPS-8@+51", "OPS-8@051", "OPS-8@-0",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := jira.ParseCursor(text)

			// Assert
			if !errors.Is(err, jira.ErrNotACursor) {
				t.Errorf("ParseCursor(%q) returned %v, want ErrNotACursor", text, err)
			}
		})
	}
}
