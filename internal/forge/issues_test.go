// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The issue endpoints each forge answers on, for githubRepo()/gitlabRepo().
const (
	githubIssuesSearch = "/search/issues"
	gitlabIssuesList   = "/projects/group%2Fsub%2Frepo/issues"
	githubIssuePath    = "/repos/example/repo/issues/42"
	gitlabIssuePath    = "/projects/group%2Fsub%2Frepo/issues/7"
)

// sampleTitle is the title the issue fixtures share.
const sampleTitle = "the bug"

func TestAssignedIssuesListsAForgesRepoIssues(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo      forge.Repo
		routes    map[string]string
		listPath  string
		wantQuery []string
		want      forge.Issue
	}{
		"asks GitHub search, scoped to the repository": {
			repo: githubRepo(),
			routes: map[string]string{
				githubIssuesSearch: `{"items":[{"number":42,"html_url":"https://github.com/example/repo/issues/42",` +
					`"title":"the bug"}]}`,
			},
			listPath:  githubIssuesSearch,
			wantQuery: []string{"is:issue", "assignee:@me", "repo:example/repo"},
			want:      forge.Issue{Number: 42, URL: "https://github.com/example/repo/issues/42", Title: sampleTitle},
		},
		"asks GitLab for the project's assigned issues": {
			repo: gitlabRepo(),
			routes: map[string]string{
				gitlabUserPath: gitlabWhoami,
				gitlabIssuesList: `[{"iid":7,"web_url":"https://gitlab.com/group/sub/repo/-/issues/7",` +
					`"title":"the bug"}]`,
			},
			listPath:  gitlabIssuesList,
			wantQuery: []string{"assignee_username=me", "state=opened"},
			want:      forge.Issue{Number: 7, URL: "https://gitlab.com/group/sub/repo/-/issues/7", Title: sampleTitle},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeRouting(t, tt.routes)

			// Act
			issues, err := client.AssignedIssues(t.Context(), tt.repo)

			// Assert
			if err != nil || len(issues) != 1 || issues[0] != tt.want {
				t.Fatalf("AssignedIssues = %+v, %v, want [%+v]", issues, err, tt.want)
			}

			decoded, _ := url.QueryUnescape(requestTo(*seen, tt.listPath).query)
			for _, part := range tt.wantQuery {
				if !strings.Contains(decoded, part) {
					t.Errorf("query %q is missing %q", decoded, part)
				}
			}
		})
	}
}

func TestReadIssueReadsBodyAndAuthor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		number int
		routes map[string]string
		want   forge.IssueDetail
	}{
		"reads a GitHub issue": {
			repo: githubRepo(), number: 42,
			routes: map[string]string{
				githubIssuePath: `{"number":42,"html_url":"https://github.com/example/repo/issues/42",` +
					`"title":"the bug","body":"it broke","user":{"login":"ana"}}`,
			},
			want: forge.IssueDetail{
				Issue: forge.Issue{Number: 42, URL: "https://github.com/example/repo/issues/42", Title: sampleTitle},
				Body:  "it broke", Author: "ana",
			},
		},
		"reads a GitLab issue": {
			repo: gitlabRepo(), number: 7,
			routes: map[string]string{
				gitlabIssuePath: `{"iid":7,"web_url":"https://gitlab.com/group/sub/repo/-/issues/7",` +
					`"title":"the bug","description":"it broke","author":{"username":"ben"}}`,
			},
			want: forge.IssueDetail{
				Issue: forge.Issue{Number: 7, URL: "https://gitlab.com/group/sub/repo/-/issues/7", Title: sampleTitle},
				Body:  "it broke", Author: "ben",
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeRouting(t, tt.routes)

			// Act
			detail, err := client.ReadIssue(t.Context(), tt.repo, tt.number)

			// Assert
			if err != nil || detail != tt.want {
				t.Errorf("ReadIssue = %+v, %v, want %+v", detail, err, tt.want)
			}
		})
	}
}

func TestCloseIssueClosesOnEachForge(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo       forge.Repo
		number     int
		reply      string
		wantMethod string
		wantField  string
		wantValue  string
	}{
		"GitHub sets the state to closed": {
			repo: githubRepo(), number: 42, reply: `{"number":42,"state":"closed"}`,
			wantMethod: http.MethodPatch, wantField: "state", wantValue: "closed",
		},
		"GitLab sends the close event": {
			repo: gitlabRepo(), number: 7, reply: `{"iid":7,"state":"closed"}`,
			wantMethod: http.MethodPut, wantField: "state_event", wantValue: "close",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeAnswering(t, http.StatusOK, tt.reply)

			// Act
			err := client.CloseIssue(t.Context(), tt.repo, tt.number)
			// Assert
			if err != nil {
				t.Fatalf("CloseIssue: %v", err)
			}

			sent := lastRequest(t, seen)
			if sent.method != tt.wantMethod || sent.body[tt.wantField] != tt.wantValue {
				t.Errorf("close sent %s %+v, want %s %s=%s", sent.method, sent.body, tt.wantMethod, tt.wantField, tt.wantValue)
			}
		})
	}
}

func TestAssignedIssuesReportsAForgeFailure(t *testing.T) {
	t.Parallel()

	for name, repo := range map[string]forge.Repo{github: githubRepo(), gitlab: gitlabRepo()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusInternalServerError, "")

			// Act
			_, err := client.AssignedIssues(t.Context(), repo)

			// Assert
			if err == nil {
				t.Errorf("AssignedIssues on %v returned no error for a failing forge", repo.Kind)
			}
		})
	}
}

func TestForgeIssueMethodsRejectAnUnknownForge(t *testing.T) {
	t.Parallel()

	unknown := forge.Repo{Kind: forge.KindUnknown, Host: "example.com", Path: "who/knows"}

	cases := map[string]func(forge.Client, forge.Repo) error{
		"AssignedIssues": func(client forge.Client, repo forge.Repo) error {
			_, err := client.AssignedIssues(context.Background(), repo)

			return err
		},
		"ReadIssue": func(client forge.Client, repo forge.Repo) error {
			_, err := client.ReadIssue(context.Background(), repo, 1)

			return err
		},
		"CloseIssue": func(client forge.Client, repo forge.Repo) error {
			return client.CloseIssue(context.Background(), repo, 1)
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusOK, `{}`)

			// Act
			err := call(client, unknown)

			// Assert
			if err == nil {
				t.Errorf("%s returned no error for an unknown forge", name)
			}
		})
	}
}
