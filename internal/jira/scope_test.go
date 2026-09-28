// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestScopedToMeWrapsTheQueryAndKeepsOrderByLast(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jql  string
		want string
	}{
		"an OR with an ORDER BY": {
			jql:  "project = A OR project = B ORDER BY updated DESC",
			want: "(project = A OR project = B) AND assignee = currentUser() ORDER BY updated DESC",
		},
		"no ORDER BY": {
			jql:  "sprint in openSprints()",
			want: "(sprint in openSprints()) AND assignee = currentUser()",
		},
		"a lowercase order by": {
			jql:  "project = A order by created",
			want: "(project = A) AND assignee = currentUser() ORDER BY created",
		},
		"a view that is only an ORDER BY": {
			jql:  "ORDER BY created DESC",
			want: "assignee = currentUser() ORDER BY created DESC",
		},
		"an ORDER BY on its own line": {
			jql:  "project = A\nORDER BY\tcreated",
			want: "(project = A) AND assignee = currentUser() ORDER BY created",
		},
		"a padded query": {
			jql:  "  project = A  ",
			want: "(project = A) AND assignee = currentUser()",
		},
		"order by quoted before the real one": {
			jql:  `summary ~ " order by x" ORDER BY created`,
			want: `(summary ~ " order by x") AND assignee = currentUser() ORDER BY created`,
		},
		"an order by inside a quoted string": {
			jql:  `project = WEB AND summary ~ "sort order by date"`,
			want: `(project = WEB AND summary ~ "sort order by date") AND assignee = currentUser()`,
		},
		"an ORDER BY inside a function's argument": {
			jql:  `issueFunction in subtasksOf("project = WEB ORDER BY rank")`,
			want: `(issueFunction in subtasksOf("project = WEB ORDER BY rank")) AND assignee = currentUser()`,
		},
		"an ORDER BY inside parentheses": {
			jql:  "issueFunction in subtasksOf(project = WEB ORDER BY rank)",
			want: "(issueFunction in subtasksOf(project = WEB ORDER BY rank)) AND assignee = currentUser()",
		},
		"order by in single quotes before the real one": {
			jql:  `summary ~ 'order by x' ORDER BY created`,
			want: `(summary ~ 'order by x') AND assignee = currentUser() ORDER BY created`,
		},
		"order by in a string holding an escaped quote": {
			jql:  `summary ~ "say \" order by x" ORDER BY created`,
			want: `(summary ~ "say \" order by x") AND assignee = currentUser() ORDER BY created`,
		},
		"a string that never closes": {
			jql:  `summary ~ "open ORDER BY created`,
			want: `summary ~ "open ORDER BY created`,
		},
		"a parenthesis that never closes": {
			jql:  "(project = A ORDER BY rank",
			want: "(project = A ORDER BY rank",
		},
		"a parenthesis closed before it opens": {
			jql:  "project = A) OR (project = B ORDER BY rank",
			want: "project = A) OR (project = B ORDER BY rank",
		},
		"a query that names the assignee": {
			jql:  "assignee is EMPTY",
			want: "assignee is EMPTY",
		},
		"a query that names the assignee in capitals": {
			jql:  "Assignee = bob",
			want: "Assignee = bob",
		},
		"the built-in list": {
			jql:  jira.AssignedToMe,
			want: jira.AssignedToMe,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := jira.ScopedToMe(tt.jql)

			// Assert
			if got != tt.want {
				t.Errorf("ScopedToMe(%q) = %q, want %q", tt.jql, got, tt.want)
			}
		})
	}
}

func TestKeysAssignedToMeNamesEveryKeyOnce(t *testing.T) {
	t.Parallel()

	// Act
	got := jira.KeysAssignedToMe([]jira.Key{"B", "A", "A"})

	// Assert
	if want := "key in (A, B) AND assignee = currentUser() AND statusCategory != done"; got != want {
		t.Errorf("KeysAssignedToMe = %q, want %q", got, want)
	}
}

func TestSearchLenientAsksJiraNotToValidateTheQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.Query())
		writer.Header().Set("Content-Type", jsonMediaType)
		_, _ = writer.Write([]byte(searchBody))
	})

	// Act
	_, err := client.SearchLenient(t.Context(), "key in (PROJ-1, GONE-9)", 50)
	if err != nil {
		t.Fatalf("SearchLenient returned %v, want nil", err)
	}

	// Assert
	got, _ := query.Load().(url.Values)
	if got.Get("validateQuery") != "false" || got.Get("startAt") != "50" ||
		!strings.Contains(got.Get("jql"), "GONE-9") {
		t.Errorf("query = %v, want the jql from startAt 50 with validateQuery=false", got)
	}
}

func TestSearchStillValidatesTheQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.RawQuery)
		writer.Header().Set("Content-Type", jsonMediaType)
		_, _ = writer.Write([]byte(searchBody))
	})

	// Act
	_, err := client.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("Search returned %v, want nil", err)
	}

	// Assert
	if got, _ := query.Load().(string); got == "" || strings.Contains(got, "validateQuery") {
		t.Errorf("query = %q, want a search that leaves validation on", got)
	}
}
