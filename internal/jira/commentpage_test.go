// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestIssuePagesTheCommentsPastTheFirstPage(t *testing.T) {
	t.Parallel()

	third := `{"author":{"displayName":"Bo"},"body":"three","created":""}`

	cases := map[string]struct {
		total      int
		pages      map[string]string
		wantN      int
		wantStarts string
	}{
		// Two comments ride along; the third pages in and completes the list.
		"pages to completion": {
			total:      3,
			pages:      map[string]string{"2": `{"total":3,"comments":[` + third + `]}`},
			wantN:      3,
			wantStarts: "2",
		},
		// A total that the pages never reach: the empty page stops the paging
		// rather than looping, and the list is left partial.
		"stops on an empty page": {
			total: 4,
			pages: map[string]string{
				"2": `{"total":4,"comments":[` + third + `]}`,
				"3": `{"total":4,"comments":[]}`,
			},
			wantN:      3,
			wantStarts: "2,3",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			issueBody := `{"key":"OPS-1","fields":{"summary":"s",` +
				`"status":{"name":"Open","statusCategory":{"key":"new"}},"issuetype":{"name":"Task"},` +
				`"priority":null,"description":"d","reporter":{"displayName":"Ana"},` +
				`"comment":{"total":` + strconv.Itoa(testCase.total) + `,"comments":[` +
				`{"author":{"displayName":"Ana"},"body":"one","created":""},` +
				`{"author":{"displayName":"Ana"},"body":"two","created":""}]}}}`

			var (
				startsMu sync.Mutex
				starts   []string
			)

			record := func(start string) {
				startsMu.Lock()
				defer startsMu.Unlock()

				starts = append(starts, start)
			}

			client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
				if !strings.HasSuffix(request.URL.Path, "/comment") {
					answer(issueBody, "fred")(writer, request)

					return
				}

				start := request.URL.Query().Get("startAt")
				record(start)
				answer(testCase.pages[start], "fred")(writer, request)
			})

			// Act
			detail, err := client.Issue(t.Context(), "OPS-1")
			if err != nil {
				t.Fatalf("Issue returned %v, want nil", err)
			}

			// Assert
			if len(detail.Comments) != testCase.wantN {
				t.Errorf("loaded %d comments, want %d", len(detail.Comments), testCase.wantN)
			}

			startsMu.Lock()
			got := strings.Join(starts, ",")
			startsMu.Unlock()

			if got != testCase.wantStarts {
				t.Errorf("paged at startAt %q, want %q", got, testCase.wantStarts)
			}
		})
	}
}
