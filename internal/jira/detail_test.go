// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// detailBody is shaped like Data Center's GET /rest/api/2/issue/{key} with
// comments, as read from a live instance: a null priority is legal, and each
// comment's date is written with milliseconds and a numeric zone.
const detailBody = `{"key":"OPS-1","fields":{"summary":"Fix login",` +
	`"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},` +
	`"issuetype":{"name":"Bug"},"priority":null,` +
	`"description":"Login fails when\r\nthe token is empty.",` +
	`"reporter":{"displayName":"Ana Lopez","name":"ana"},` +
	`"comment":{"startAt":0,"maxResults":1000,"total":2,"comments":[` +
	`{"author":{"displayName":"Ana Lopez"},"body":"Repro'd on 8.2.1","created":"2026-09-03T10:22:12.928+0000"},` +
	`{"author":{"displayName":"Fred"},"body":"Patch up shortly","created":"not a date"}]}}}`

// detailServer is a Jira that answers detailBody, and a record of the path and
// query it was asked.
func detailServer(t *testing.T) (jira.Client, *atomic.Value) {
	t.Helper()

	var asked atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		asked.Store(request.URL.Path + "?" + request.URL.RawQuery)
		answer(detailBody, "fred")(writer, request)
	})

	return client, &asked
}

func TestIssueReadsTheDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	client, asked := detailServer(t)

	// Act
	detail, err := client.Issue(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Issue returned %v, want nil", err)
	}

	// Assert
	wantIssue := jira.Issue{
		Key: "OPS-1", Summary: "Fix login", Status: inProgress, StatusCategory: indeterminate,
		Type: "Bug", Priority: "",
	}

	if detail.Issue != wantIssue {
		t.Errorf("Issue = %+v, want %+v", detail.Issue, wantIssue)
	}

	// The \r of a Windows line ending is dropped on the way in.
	if detail.Description != "Login fails when\nthe token is empty." || detail.Reporter != "Ana Lopez" {
		t.Errorf("Description, Reporter = %q, %q", detail.Description, detail.Reporter)
	}

	requested, _ := asked.Load().(string)
	if !strings.HasPrefix(requested, "/rest/api/2/issue/OPS-1?") || !strings.Contains(requested, "comment") ||
		!strings.Contains(requested, "description") {
		t.Errorf("requested %q, want the issue with its description and comments", requested)
	}
}

func TestIssueReadsItsComments(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := detailServer(t)

	// Act
	detail, err := client.Issue(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Issue returned %v, want nil", err)
	}

	// Assert
	if len(detail.Comments) != 2 || detail.CommentTotal != 2 {
		t.Fatalf("got %d comments of %d, want 2 of 2", len(detail.Comments), detail.CommentTotal)
	}

	first := detail.Comments[0]
	if first.Author != "Ana Lopez" || first.Body != "Repro'd on 8.2.1" ||
		!first.Created.Equal(time.Date(2026, 9, 3, 10, 22, 12, 928e6, time.UTC)) {
		t.Errorf("first comment = %+v", first)
	}

	// A date Jira wrote some other way is shown without one, not refused.
	if !detail.Comments[1].Created.IsZero() {
		t.Errorf("an unreadable date parsed as %v, want the zero time", detail.Comments[1].Created)
	}
}

// fullDetailBody is an issue carrying every field the detail now reads: an
// assignee, labels, components, fix versions, a parent, a subtask, and two
// issue links (one on each side of the relationship) plus a third that names
// neither side, which is dropped rather than shown as a link to nothing.
const fullDetailBody = `{"key":"OPS-1","fields":{"summary":"Fix login",` +
	`"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},` +
	`"issuetype":{"name":"Bug"},"priority":{"name":"High"},"description":"d",` +
	`"reporter":{"displayName":"Ana Lopez"},"assignee":{"displayName":"Fred Ops"},` +
	`"labels":["backend","urgent"],"components":[{"name":"auth"},{"name":"api"}],` +
	`"fixVersions":[{"name":"1.2.0"}],` +
	`"parent":{"key":"OPS-0","fields":{"summary":"Epic login","status":{"name":"Open"}}},` +
	`"subtasks":[{"key":"OPS-2","fields":{"summary":"Write test","status":{"name":"To Do"}}}],` +
	`"issuelinks":[` +
	`{"type":{"inward":"is blocked by","outward":"blocks"},` +
	`"outwardIssue":{"key":"OPS-3","fields":{"summary":"Deploy","status":{"name":"Closed"}}}},` +
	`{"type":{"inward":"is blocked by","outward":"blocks"},` +
	`"inwardIssue":{"key":"OPS-4","fields":{"summary":"Related","status":{"name":"Open"}}}},` +
	`{"type":{"inward":"duplicates","outward":"is duplicated by"}}],` +
	`"comment":{"total":0,"comments":[]}}}`

func TestIssueReadsTheWholeIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer(fullDetailBody, "fred"))

	// Act
	detail, err := client.Issue(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Issue returned %v, want nil", err)
	}

	// Assert
	if detail.Assignee != "Fred Ops" {
		t.Errorf("Assignee = %q, want Fred Ops", detail.Assignee)
	}

	if strings.Join(detail.Labels, ",") != "backend,urgent" ||
		strings.Join(detail.Components, ",") != "auth,api" ||
		strings.Join(detail.FixVersions, ",") != "1.2.0" {
		t.Errorf("labels/components/fixVersions = %v / %v / %v", detail.Labels, detail.Components, detail.FixVersions)
	}

	if detail.Parent != (jira.LinkedIssue{Key: "OPS-0", Summary: "Epic login", Status: "Open"}) {
		t.Errorf("Parent = %+v", detail.Parent)
	}

	wantSubtask := jira.LinkedIssue{Key: "OPS-2", Summary: "Write test", Status: "To Do"}
	if len(detail.Subtasks) != 1 || detail.Subtasks[0] != wantSubtask {
		t.Errorf("Subtasks = %+v", detail.Subtasks)
	}

	// The two links use the same type from opposite sides, so the inward wording
	// ("is blocked by") and the outward wording ("blocks") must not be confused.
	wantLinks := []jira.IssueLink{
		{Relation: "blocks", Issue: jira.LinkedIssue{Key: "OPS-3", Summary: "Deploy", Status: "Closed"}},
		{Relation: "is blocked by", Issue: jira.LinkedIssue{Key: "OPS-4", Summary: "Related", Status: "Open"}},
	}
	if !reflect.DeepEqual(detail.IssueLinks, wantLinks) {
		t.Errorf("IssueLinks = %+v, want %+v", detail.IssueLinks, wantLinks)
	}
}

func TestIssueLeavesAbsentFieldsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	// The common shape: a null assignee and no labels, components, versions,
	// parent, subtasks or links.
	body := `{"key":"OPS-1","fields":{"summary":"s",` +
		`"status":{"name":"Open","statusCategory":{"key":"new"}},` +
		`"issuetype":{"name":"Task"},"priority":null,"description":"d",` +
		`"reporter":{"displayName":"Ana"},"assignee":null,` +
		`"comment":{"total":0,"comments":[]}}}`
	client := serve(t, answer(body, "fred"))

	// Act
	detail, err := client.Issue(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Issue returned %v, want nil", err)
	}

	// Assert
	if detail.Assignee != "" || len(detail.Labels) != 0 || len(detail.Components) != 0 ||
		len(detail.FixVersions) != 0 || len(detail.Subtasks) != 0 || len(detail.IssueLinks) != 0 {
		t.Errorf("absent fields = %+v, want all empty", detail)
	}

	if detail.Parent != (jira.LinkedIssue{}) {
		t.Errorf("Parent = %+v, want the zero LinkedIssue", detail.Parent)
	}
}

func TestIssueReportsJirasReason(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, failWith(http.StatusNotFound, `{"errorMessages":["Issue Does Not Exist"]}`, "fred"))

	// Act
	_, err := client.Issue(t.Context(), "OPS-404")

	// Assert
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "Issue Does Not Exist") {
		t.Errorf("Issue returned %v, want Jira's reason", err)
	}
}

func TestIssueReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, answer("{not json", "fred"))

	// Act
	_, err := client.Issue(t.Context(), "OPS-1")

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("Issue returned %v, want the malformed body's syntax error", err)
	}
}

func TestIssueWithoutACredentialIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	client := jira.New(http.DefaultClient.Do, config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""})

	// Act
	_, err := client.Issue(t.Context(), "OPS-1")

	// Assert
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Issue returned %v, want ErrNoCredential", err)
	}
}

func TestAddCommentPostsTheBodyAndReturnsTheComment(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		requested atomic.Value
		sent      atomic.Value
	)

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.EscapedPath() + " " + request.Header.Get("Content-Type"))

		var body struct {
			Body string `json:"body"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sent.Store(body.Body)

		writer.Header().Set("X-Ausername", "fred")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"author":{"displayName":"Fred"},"body":"Patch up",` +
			`"created":"2026-09-16T08:00:00.000-0600"}`))
	})

	// Act
	comment, err := client.AddComment(t.Context(), "OPS/1", "Patch up\n\nwith \"quotes\"")
	if err != nil {
		t.Fatalf("AddComment returned %v, want nil", err)
	}

	// Assert
	if got := requested.Load(); got != "POST /rest/api/2/issue/OPS%2F1/comment "+jsonMediaType {
		t.Errorf("requested %v, want a JSON POST to the escaped issue's comments", got)
	}

	if got := sent.Load(); got != "Patch up\n\nwith \"quotes\"" {
		t.Errorf("sent body %q, want the text exactly", got)
	}

	if comment.Author != "Fred" || !comment.Created.Equal(time.Date(2026, 9, 16, 14, 0, 0, 0, time.UTC)) {
		t.Errorf("AddComment = %+v", comment)
	}
}

func TestAddCommentRewritesMarkdownWhenTheInstanceIsConfiguredFor(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Value

	client := serveMarkdown(t, func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Body string `json:"body"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sent.Store(body.Body)

		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"author":{"displayName":"Fred"},"body":"x",` +
			`"created":"2026-09-16T08:00:00.000-0600"}`))
	})

	// Act
	_, err := client.AddComment(t.Context(), "OPS-1", "See **the docs** at `run()`.")
	if err != nil {
		t.Fatalf("AddComment returned %v, want nil", err)
	}

	// Assert
	if got := sent.Load(); got != "See *the docs* at {{run()}}." {
		t.Errorf("sent body %q, want the wiki markup", got)
	}
}

func TestAddCommentPostsMarkdownVerbatimByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Body string `json:"body"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sent.Store(body.Body)

		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"author":{"displayName":"Fred"},"body":"x",` +
			`"created":"2026-09-16T08:00:00.000-0600"}`))
	})

	// Act
	_, err := client.AddComment(t.Context(), "OPS-1", "See **the docs** at `run()`.")
	if err != nil {
		t.Fatalf("AddComment returned %v, want nil", err)
	}

	// Assert
	// The default leaves the text untouched, so an instance that already writes
	// wiki markup is not mangled by a conversion it never asked for.
	if got := sent.Load(); got != "See **the docs** at `run()`." {
		t.Errorf("sent body %q, want the Markdown unchanged", got)
	}
}

func TestLinkPullRequestPostsARemoteLink(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		requested atomic.Value
		sent      atomic.Value
	)

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		requested.Store(request.Method + " " + request.URL.EscapedPath() + " " + request.Header.Get("Content-Type"))

		var body struct {
			Object struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"object"`
		}

		_ = json.NewDecoder(request.Body).Decode(&body)
		sent.Store(body.Object.URL + " " + body.Object.Title)

		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":10001}`))
	})

	// Act
	err := client.LinkPullRequest(t.Context(), "OPS/1", "https://forge/pull/42", "fix(config): redact tokens")
	if err != nil {
		t.Fatalf("LinkPullRequest returned %v, want nil", err)
	}

	// Assert
	if got := requested.Load(); got != "POST /rest/api/2/issue/OPS%2F1/remotelink "+jsonMediaType {
		t.Errorf("requested %v, want a JSON POST to the escaped issue's remote links", got)
	}

	if got := sent.Load(); got != "https://forge/pull/42 fix(config): redact tokens" {
		t.Errorf("sent %q, want the pull request's URL and title", got)
	}
}

func TestAddCommentReportsJirasReason(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := `{"errorMessages":[],"errors":{"comment":"Comment body can not be empty!"}}`
	client := serve(t, failWith(http.StatusBadRequest, empty, "fred"))

	// Act
	_, err := client.AddComment(t.Context(), "OPS-1", " ")

	// Assert
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "can not be empty") {
		t.Errorf("AddComment returned %v, want Jira's reason", err)
	}
}

func TestAddCommentReportsAnUnreadableAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("{not json"))
	})

	// Act
	_, err := client.AddComment(t.Context(), "OPS-1", "x")

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("AddComment returned %v, want the malformed answer's syntax error", err)
	}
}

func TestAddCommentRefusesAnInvalidBaseURL(t *testing.T) {
	t.Parallel()

	// Arrange
	client := jira.New(http.DefaultClient.Do, config.Jira{BaseURL: "://bad", Token: token, User: ""})

	// Act
	_, err := client.AddComment(t.Context(), "OPS-1", "x")

	// Assert
	if !errors.Is(err, jira.ErrInvalidBaseURL) {
		t.Errorf("AddComment returned %v, want ErrInvalidBaseURL", err)
	}
}

func TestBrowseURLLinksAnIssueWithoutACredential(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		base string
		want string
	}{
		"plain":            {base: "https://jira.example.com", want: "https://jira.example.com/browse/OPS-1"},
		"a context path":   {base: "https://example.com/jira/", want: "https://example.com/jira/browse/OPS-1"},
		"a password in it": {base: "https://ana:sekret@jira.example.com", want: "https://jira.example.com/browse/OPS-1"},
		"not a url":        {base: "://nope", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := jira.New(http.DefaultClient.Do, config.Jira{BaseURL: tt.base, Token: token, User: ""})

			// Act
			got := client.BrowseURL("OPS-1")

			// Assert
			if got != tt.want {
				t.Errorf("BrowseURL = %q, want %q", got, tt.want)
			}

			// The link is posted to Slack, so no part of a credential may be in it.
			parsed, err := url.Parse(got)
			if err == nil && parsed.User != nil {
				t.Errorf("BrowseURL kept userinfo: %q", got)
			}
		})
	}
}
