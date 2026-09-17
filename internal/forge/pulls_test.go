// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Repositories the pull request tests talk about. The GitLab one is nested,
// because GitLab projects are, and the path has to reach the API whole.
func githubRepo() forge.Repo {
	return forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "example/repo"}
}

func gitlabRepo() forge.Repo {
	return forge.Repo{Kind: forge.KindGitLab, Host: "gitlab.com", Path: "group/sub/repo"}
}

// featureBranch is the branch every pull request here is opened from, baseBranch
// where it merges, and prTitle what it is called.
const (
	featureBranch = "fix/PROJ-1-token"
	baseBranch    = "main"
	prTitle       = "fix: token"
)

// The API paths the pull request tests talk to, named so a repeated literal does
// not read as a coincidence.
const (
	githubPullsPath  = "/repos/example/repo/pulls"
	gitlabMergesPath = "/projects/group%2Fsub%2Frepo/merge_requests"
)

// recorded is what a fake forge saw of one request.
type recorded struct {
	method, path, query string
	body                map[string]any
}

// forgeAnswering serves one answer and records the request it got.
func forgeAnswering(t *testing.T, status int, body string) (forge.Client, *atomic.Value) {
	t.Helper()

	var seen atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var sent map[string]any

		_ = json.NewDecoder(request.Body).Decode(&sent)
		seen.Store(recorded{
			method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery, body: sent,
		})

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret), &seen
}

// lastRequest is what the fake forge last saw.
func lastRequest(t *testing.T, seen *atomic.Value) recorded {
	t.Helper()

	got, ok := seen.Load().(recorded)
	if !ok {
		t.Fatal("the forge was never asked anything")
	}

	return got
}

func TestFindPullRequestAsksEachForgeForTheBranch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo      forge.Repo
		routes    map[string]string
		listPath  string
		wantQuery []string
		want      forge.PullRequest
	}{
		"asks GitHub": {
			repo: githubRepo(),
			routes: map[string]string{
				githubPullsPath: `[{"number":42,"html_url":"https://github.com/example/repo/pull/42",` +
					`"title":"fix: token","draft":true}]`,
				githubPullsPath + "/42": `{"mergeable":true}`,
				githubPullsPath + "/42/reviews": `[{"state":"APPROVED","user":{"login":"ana"}},` +
					`{"state":"APPROVED","user":{"login":"ben"}},{"state":"COMMENTED","user":{"login":"cass"}}]`,
			},
			listPath: githubPullsPath,
			// head is owner:branch on GitHub, or the filter matches nothing.
			wantQuery: []string{"head=example%3Afix%2FPROJ-1-token", "state=open"},
			want: forge.PullRequest{
				Number: 42, URL: "https://github.com/example/repo/pull/42", Title: prTitle, Draft: true,
				Approvals: 2, Mergeable: forge.MergeClean,
			},
		},
		"asks GitLab": {
			repo: gitlabRepo(),
			routes: map[string]string{
				gitlabMergesPath: `[{"iid":7,` +
					`"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/7",` +
					`"title":"fix: token","draft":false,"merge_status":"can_be_merged"}]`,
				gitlabMergesPath + "/7/approvals": `{"approved_by":[{},{}]}`,
			},
			// The project path is one escaped segment, slashes and all.
			listPath:  gitlabMergesPath,
			wantQuery: []string{"source_branch=fix%2FPROJ-1-token", "state=opened"},
			want: forge.PullRequest{
				Number: 7, URL: "https://gitlab.com/group/sub/repo/-/merge_requests/7", Title: prTitle, Draft: false,
				Approvals: 2, Mergeable: forge.MergeClean,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeRouting(t, tt.routes)

			// Act
			found, ok, err := client.FindPullRequest(t.Context(), tt.repo, featureBranch)

			// Assert
			if err != nil || !ok || found != tt.want {
				t.Errorf("FindPullRequest = %+v, %v, %v, want %+v", found, ok, err, tt.want)
			}

			listed := requestTo(*seen, tt.listPath)
			if listed.method != http.MethodGet {
				t.Errorf("the branch was not listed at %s; requests were %+v", tt.listPath, *seen)
			}

			for _, part := range tt.wantQuery {
				if !strings.Contains(listed.query, part) {
					t.Errorf("query %q is missing %q", listed.query, part)
				}
			}
		})
	}
}

func TestFindPullRequestWithNoneOpen(t *testing.T) {
	t.Parallel()

	for name, repo := range map[string]forge.Repo{github: githubRepo(), gitlab: gitlabRepo()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusOK, `[]`)

			// Act
			_, ok, err := client.FindPullRequest(t.Context(), repo, featureBranch)

			// Assert
			if err != nil || ok {
				t.Errorf("FindPullRequest on %v = %v, %v, want none found and no error", repo.Kind, ok, err)
			}
		})
	}
}

func TestCreatePullRequestOnGitHub(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusCreated,
		`{"number":43,"html_url":"https://github.com/example/repo/pull/43","title":"fix: token","draft":true}`)

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Body: "## Why\n\nBecause.", Head: featureBranch, Base: baseBranch, Draft: true,
	})

	// Assert
	if err != nil || created.Number != 43 || created.URL != "https://github.com/example/repo/pull/43" {
		t.Fatalf("CreatePullRequest = %+v, %v", created, err)
	}

	asked := lastRequest(t, seen)
	if asked.method != http.MethodPost || asked.path != githubPullsPath {
		t.Errorf("asked %s %s, want POST %s", asked.method, asked.path, githubPullsPath)
	}

	want := map[string]any{
		"title": prTitle, "body": "## Why\n\nBecause.", "head": featureBranch, "base": baseBranch, "draft": true,
	}
	for key, value := range want {
		if asked.body[key] != value {
			t.Errorf("sent %s = %v, want %v", key, asked.body[key], value)
		}
	}
}

func TestCreateMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusCreated,
		`{"iid":8,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/8","title":"Draft: fix: token","draft":true}`)

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Body: "Because.", Head: featureBranch, Base: baseBranch, Draft: true,
	})

	// Assert
	if err != nil || created.Number != 8 || !created.Draft {
		t.Fatalf("CreatePullRequest = %+v, %v", created, err)
	}

	asked := lastRequest(t, seen)
	if asked.path != gitlabMergesPath {
		t.Errorf("asked %s, want the project's merge requests", asked.path)
	}

	// GitLab marks a draft by its title, which every version understands.
	want := map[string]any{
		"title": "Draft: fix: token", "description": "Because.", "source_branch": featureBranch, "target_branch": baseBranch,
	}
	for key, value := range want {
		if asked.body[key] != value {
			t.Errorf("sent %s = %v, want %v", key, asked.body[key], value)
		}
	}
}

func TestARefusedPullRequestSaysWhy(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		status int
		body   string
		want   string
	}{
		"github validation": {
			repo: githubRepo(), status: http.StatusUnprocessableEntity,
			body: `{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom",` +
				`"message":"A pull request already exists for example:fix/PROJ-1-token."}]}`,
			want: "A pull request already exists",
		},
		"gitlab conflict": {
			repo: gitlabRepo(), status: http.StatusConflict,
			body: `{"message":["Another open merge request already exists for this source branch: !7"]}`,
			want: "Another open merge request already exists",
		},
		"gitlab error": {
			repo: gitlabRepo(), status: http.StatusBadRequest,
			body: `{"error":"title is missing"}`,
			want: "title is missing",
		},
		"github error without a message": {
			repo: githubRepo(), status: http.StatusUnprocessableEntity,
			body: `{"message":"Validation Failed","errors":[{"code":"missing_field"}]}`,
			want: "Validation Failed",
		},
		"gitlab plain message": {
			repo: gitlabRepo(), status: http.StatusBadRequest,
			body: `{"message":"target_branch is missing"}`,
			want: "target_branch is missing",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, tt.status, tt.body)

			// Act
			_, err := client.CreatePullRequest(t.Context(), tt.repo, forge.NewPullRequest{
				Title: prTitle, Body: "", Head: featureBranch, Base: baseBranch, Draft: false,
			})

			// Assert
			if !errors.Is(err, forge.ErrRejected) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("CreatePullRequest returned %v, want the forge's reason %q", err, tt.want)
			}
		})
	}
}

// unknownForge is a repository on a host that names no forge.
func unknownForge() forge.Repo {
	return forge.Repo{Kind: forge.KindUnknown, Host: "git.example.com", Path: "a/b"}
}

func TestFindPullRequestNeedsAForgeThatIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusOK, `[]`)

	// Act
	_, _, err := client.FindPullRequest(t.Context(), unknownForge(), featureBranch)

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) || seen.Load() != nil {
		t.Errorf("FindPullRequest returned %v and asked %v, want ErrUnknownForge before asking", err, seen.Load())
	}
}

func TestCreatePullRequestNeedsAForgeThatIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusOK, `[]`)

	// Act
	_, err := client.CreatePullRequest(t.Context(), unknownForge(), forge.NewPullRequest{})

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) || seen.Load() != nil {
		t.Errorf("CreatePullRequest returned %v and asked %v, want ErrUnknownForge before asking", err, seen.Load())
	}
}

func TestCheckStatusNeedsAForgeThatIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusOK, `[]`)

	// Act
	_, err := client.CheckStatus(t.Context(), unknownForge(), forge.PullRequest{}, "abc")

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) || seen.Load() != nil {
		t.Errorf("CheckStatus returned %v and asked %v, want ErrUnknownForge before asking", err, seen.Load())
	}
}

func TestARepositoryTheTokenCannotSeeSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// Both forges answer 404, not 403, for a private repository the token has
	// no access to. On /user a 404 means a wrong address; here it does not.
	client, _ := forgeAnswering(t, http.StatusNotFound, `{"message":"Not Found"}`)

	// Act
	_, _, err := client.FindPullRequest(t.Context(), githubRepo(), featureBranch)

	// Assert
	if !errors.Is(err, forge.ErrNoRepository) || errors.Is(err, forge.ErrNoAPI) {
		t.Errorf("FindPullRequest returned %v, want ErrNoRepository", err)
	}
}

func TestOnlyARefusalTheForgeExplainsCarriesItsReason(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		body   string
		want   error
	}{
		// GitHub's 403 body cannot tell a rate limit from a missing User-Agent,
		// so it is not read as a reason.
		"forbidden with a message": {
			status: http.StatusForbidden, body: `{"message":"API rate limit exceeded"}`, want: forge.ErrRefused,
		},
		"unexplained": {
			status: http.StatusUnprocessableEntity, body: `not json`, want: forge.ErrUnexpectedStatus,
		},
		"explained with nothing": {
			status: http.StatusUnprocessableEntity, body: `{"message":""}`, want: forge.ErrUnexpectedStatus,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, tt.status, tt.body)

			// Act
			_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{})

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, forge.ErrRejected) {
				t.Errorf("CreatePullRequest returned %v, want %v and no reason", err, tt.want)
			}
		})
	}
}

func TestARefusalThatBreaksOffKeepsItsStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	dropped := func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(brokenBody{}),
		}, nil
	}

	client := forge.New(dropped, "https://api.example.com", secret)

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{})

	// Assert
	if !errors.Is(err, forge.ErrUnexpectedStatus) {
		t.Errorf("CreatePullRequest returned %v, want the status error when no reason could be read", err)
	}
}
