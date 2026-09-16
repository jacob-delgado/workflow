// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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

// servedDetail answers detailBody and records what was asked for.
func servedDetail(t *testing.T) (jira.IssueDetail, string) {
	t.Helper()

	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Path + "?" + request.URL.RawQuery)
		answer(detailBody, "fred")(writer, request)
	})

	detail, err := client.Issue(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Issue returned %v, want nil", err)
	}

	asked, _ := query.Load().(string)

	return detail, asked
}

func TestIssueReadsTheDetail(t *testing.T) {
	t.Parallel()

	detail, asked := servedDetail(t)

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

	if !strings.HasPrefix(asked, "/rest/api/2/issue/OPS-1?") || !strings.Contains(asked, "comment") ||
		!strings.Contains(asked, "description") {
		t.Errorf("requested %q, want the issue with its description and comments", asked)
	}
}

func TestIssueReadsItsComments(t *testing.T) {
	t.Parallel()

	detail, _ := servedDetail(t)

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

func TestIssueReportsFailures(t *testing.T) {
	t.Parallel()

	_, err := serve(t, failWith(http.StatusNotFound, `{"errorMessages":["Issue Does Not Exist"]}`, "fred")).
		Issue(t.Context(), "OPS-404")
	if !errors.Is(err, jira.ErrRejected) {
		t.Errorf("Issue returned %v, want Jira's reason", err)
	}

	_, err = serve(t, answer("{not json", "fred")).Issue(t.Context(), "OPS-1")
	if err == nil {
		t.Error("Issue accepted a malformed body")
	}

	_, err = jira.New(http.DefaultClient.Do, config.Jira{BaseURL: exampleBaseURL, Token: "", User: ""}).
		Issue(t.Context(), "OPS-1")
	if !errors.Is(err, jira.ErrNoCredential) {
		t.Errorf("Issue returned %v, want ErrNoCredential", err)
	}
}

func TestAddCommentPostsTheBodyAndReturnsTheComment(t *testing.T) {
	t.Parallel()

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

	comment, err := client.AddComment(t.Context(), "OPS/1", "Patch up\n\nwith \"quotes\"")
	if err != nil {
		t.Fatalf("AddComment returned %v, want nil", err)
	}

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

func TestAddCommentReportsFailures(t *testing.T) {
	t.Parallel()

	empty := `{"errorMessages":[],"errors":{"comment":"Comment body can not be empty!"}}`

	_, err := serve(t, failWith(http.StatusBadRequest, empty, "fred")).AddComment(t.Context(), "OPS-1", " ")
	if !errors.Is(err, jira.ErrRejected) || !strings.Contains(err.Error(), "can not be empty") {
		t.Errorf("AddComment returned %v, want Jira's reason", err)
	}

	_, err = serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("{not json"))
	}).AddComment(t.Context(), "OPS-1", "x")
	if err == nil {
		t.Error("AddComment accepted a malformed answer")
	}

	_, err = jira.New(http.DefaultClient.Do, config.Jira{BaseURL: "://bad", Token: token, User: ""}).
		AddComment(t.Context(), "OPS-1", "x")
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

			client := jira.New(http.DefaultClient.Do, config.Jira{BaseURL: tt.base, Token: token, User: ""})
			if got := client.BrowseURL("OPS-1"); got != tt.want {
				t.Errorf("BrowseURL = %q, want %q", got, tt.want)
			}

			// The link is posted to Slack, so no part of a credential may be in it.
			parsed, err := url.Parse(client.BrowseURL("OPS-1"))
			if err == nil && parsed.User != nil {
				t.Errorf("BrowseURL kept userinfo: %q", client.BrowseURL("OPS-1"))
			}
		})
	}
}
