// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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

// forgePaging serves each path's pages: the request's page parameter picks one
// of the path's pages, the first when it names none. A page past the last fails
// the test and is refused, as GitHub's search refuses a page past the results it
// serves, because a listing read to its end never asks for one.
func forgePaging(t *testing.T, pages map[string][]string) forge.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		page, err := strconv.Atoi(request.URL.Query().Get("page"))
		if err != nil {
			page = 1
		}

		bodies := pages[request.URL.EscapedPath()]
		if page < 1 || page > len(bodies) {
			t.Errorf("asked for page %d of %s, which has %d", page, request.URL.EscapedPath(), len(bodies))
			writer.WriteHeader(http.StatusUnprocessableEntity)

			return
		}

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = writer.Write([]byte(bodies[page-1]))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret)
}

// listingOf is a JSON array of count items numbered on from first, each as item
// writes it.
func listingOf(first, count int, item func(number int) string) string {
	items := make([]string, 0, count)
	for number := first; number < first+count; number++ {
		items = append(items, item(number))
	}

	return "[" + strings.Join(items, ",") + "]"
}

// githubNumbered and gitlabNumbered are a listed item as each forge numbers it.
func githubNumbered(number int) string { return `{"number":` + strconv.Itoa(number) + `}` }

func gitlabNumbered(number int) string { return `{"iid":` + strconv.Itoa(number) + `}` }

// searchPage is one page of a GitHub search answer: count items numbered on
// from first, of total found in all.
func searchPage(first, count, total int) string {
	return `{"total_count":` + strconv.Itoa(total) + `,"items":` + listingOf(first, count, githubNumbered) + `}`
}

// fullPages is count pages of a hundred items each, numbered on from 1, each as
// page writes it from its first number.
func fullPages(count int, page func(first int) string) []string {
	const perPage = 100

	bodies := make([]string, 0, count)
	for index := range count {
		bodies = append(bodies, page(index*perPage+1))
	}

	return bodies
}

func TestAssignedIssuesReadsEveryPage(t *testing.T) {
	t.Parallel()

	gitlabPage := func(first int) string { return listingOf(first, 100, gitlabNumbered) }

	cases := map[string]struct {
		repo  forge.Repo
		pages map[string][]string
		want  int
	}{
		"GitHub reads on past a full page": {
			repo:  githubRepo(),
			pages: map[string][]string{githubIssuesSearch: {searchPage(1, 100, 101), searchPage(101, 1, 101)}},
			want:  101,
		},
		"GitHub asks for no page past its count": {
			repo:  githubRepo(),
			pages: map[string][]string{githubIssuesSearch: {searchPage(1, 100, 100)}},
			want:  100,
		},
		"GitHub stops where its search stops serving": {
			repo: githubRepo(),
			pages: map[string][]string{githubIssuesSearch: fullPages(10, func(first int) string {
				return searchPage(first, 100, 1500)
			})},
			want: 1000,
		},
		"GitLab reads on until a short page": {
			repo: gitlabRepo(),
			pages: map[string][]string{
				gitlabUserPath:   {gitlabWhoami},
				gitlabIssuesList: {gitlabPage(1), listingOf(101, 1, gitlabNumbered)},
			},
			want: 101,
		},
		"GitLab stops at the page bound": {
			repo:  gitlabRepo(),
			pages: map[string][]string{gitlabUserPath: {gitlabWhoami}, gitlabIssuesList: fullPages(21, gitlabPage)},
			want:  2000,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forgePaging(t, tt.pages)

			// Act
			issues, err := client.AssignedIssues(t.Context(), tt.repo)

			// Assert
			if err != nil || len(issues) != tt.want || issues[len(issues)-1].Number != tt.want {
				t.Errorf("AssignedIssues read %d issues, %v; want %d, the last numbered %d",
					len(issues), err, tt.want, tt.want)
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

func TestReadIssueReadsWhetherTheIssueIsClosed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		number int
		path   string
		reply  string
		want   bool
	}{
		"a closed GitHub issue": {
			repo: githubRepo(), number: 42, path: githubIssuePath, reply: `{"number":42,"state":"closed"}`, want: true,
		},
		"an open GitHub issue": {
			repo: githubRepo(), number: 42, path: githubIssuePath, reply: `{"number":42,"state":"open"}`, want: false,
		},
		"a closed GitLab issue": {
			repo: gitlabRepo(), number: 7, path: gitlabIssuePath, reply: `{"iid":7,"state":"closed"}`, want: true,
		},
		"an opened GitLab issue": {
			repo: gitlabRepo(), number: 7, path: gitlabIssuePath, reply: `{"iid":7,"state":"opened"}`, want: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeRouting(t, map[string]string{tt.path: tt.reply})

			// Act
			detail, err := client.ReadIssue(t.Context(), tt.repo, tt.number)

			// Assert
			if err != nil || detail.Closed != tt.want {
				t.Errorf("ReadIssue = %+v, %v; want closed %v", detail, err, tt.want)
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
