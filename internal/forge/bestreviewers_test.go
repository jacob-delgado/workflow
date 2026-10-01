// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Names neither forge knows, so asking for them is turned down.
const (
	userGhost  = "ghost"
	teamNobody = "example/nobody"
)

// githubUnprocessable is GitHub's 422 for a reviewer it cannot request.
const githubUnprocessable = `{"message":"Reviews may only be requested from collaborators."}`

// scriptedForge answers each request as answer says, from what was asked, and
// records every request with its decoded JSON body.
func scriptedForge(t *testing.T, answer func(asked recorded) (int, string)) (forge.Client, *[]recorded) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []recorded
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any

		_ = json.NewDecoder(request.Body).Decode(&body)
		asked := recorded{
			method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery, body: body,
		}

		lock.Lock()

		seen = append(seen, asked)

		lock.Unlock()

		status, text := answer(asked)

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(text))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret), &seen
}

// names reads a JSON list of strings out of a recorded body.
func names(body map[string]any, key string) []string {
	listed, _ := body[key].([]any)
	found := make([]string, 0, len(listed))

	for _, each := range listed {
		text, _ := each.(string)
		found = append(found, text)
	}

	return found
}

// githubRefusingStrangers is a GitHub that opens pull 43 and turns down any
// reviewer request naming the ghost user or the nobody team.
func githubRefusingStrangers(asked recorded) (int, string) {
	switch asked.path {
	case githubPullsPath:
		return http.StatusCreated, githubPull43
	case githubReviewersPath:
		if slices.Contains(names(asked.body, "reviewers"), userGhost) ||
			slices.Contains(names(asked.body, "team_reviewers"), "nobody") {
			return http.StatusUnprocessableEntity, githubUnprocessable
		}
	}

	return http.StatusCreated, "{}"
}

func TestCreatePullRequestOnGitHubAddsEveryReviewerItCanWhenOneIsTurnedDown(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := scriptedForge(t, githubRefusingStrangers)

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna, userGhost}, TeamReviewers: []string{"example/api", teamNobody},
		Labels: []string{labelBug},
	})

	// Assert
	if created.Number != 43 || !errors.Is(err, forge.ErrSomeReviewersNotAdded) {
		t.Fatalf("CreatePullRequest = %+v, %v; want pull 43 and ErrSomeReviewersNotAdded", created, err)
	}

	if !strings.Contains(err.Error(), userGhost) || !strings.Contains(err.Error(), teamNobody) ||
		strings.Contains(err.Error(), userAna) {
		t.Errorf("error = %q, want only ghost and %s named", err, teamNobody)
	}

	var accepted [][]string

	for _, asked := range *seen {
		if asked.path == githubReviewersPath {
			accepted = append(accepted, append(names(asked.body, "reviewers"), names(asked.body, "team_reviewers")...))
		}
	}

	want := [][]string{{userAna, userGhost, "api", "nobody"}, {userAna}, {userGhost}, {"api"}, {"nobody"}}
	if !reflect.DeepEqual(accepted, want) {
		t.Errorf("reviewer requests = %v, want the combined one then each alone", accepted)
	}

	if got := requestTo(*seen, githubPRIssuePath+"/labels"); got.method == "" {
		t.Error("labels were not added after a reviewer was turned down")
	}
}

func TestCreatePullRequestOnGitHubDoesNotRetryReviewersTheTokenMayNotRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := scriptedForge(t, func(asked recorded) (int, string) {
		if asked.path == githubReviewersPath {
			return http.StatusForbidden, `{"message":"Resource not accessible by personal access token"}`
		}

		return http.StatusCreated, githubPull43
	})

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{userAna, userBen},
	})

	// Assert
	asks := 0

	for _, asked := range *seen {
		if asked.path == githubReviewersPath {
			asks++
		}
	}

	if asks != 1 || !errors.Is(err, forge.ErrRefused) {
		t.Errorf("asked %d times, error %v; want one ask and ErrRefused", asks, err)
	}
}

// gitlabKnowing is a GitLab that knows the users named in ids, opens merge
// request 8, and answers the members of the groups in members.
func gitlabKnowing(ids, members map[string]string) func(asked recorded) (int, string) {
	return func(asked recorded) (int, string) {
		switch asked.path {
		case gitlabUsersPath:
			_, username, _ := strings.Cut(asked.query, "username=")
			if id, known := ids[username]; known {
				return http.StatusOK, `[{"id":` + id + `}]`
			}

			return http.StatusOK, "[]"
		case gitlabMergesPath:
			return http.StatusCreated, `{"iid":8,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/8"}`
		}

		if answer, ok := members[asked.path]; ok {
			return http.StatusOK, answer
		}

		return http.StatusNotFound, `{"message":"404 Group Not Found"}`
	}
}

func TestCreateMergeRequestOnGitLabOpensWithTheReviewersItKnows(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := scriptedForge(t, gitlabKnowing(map[string]string{userAna: "7"}, nil))

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{userGhost, userAna},
	})

	// Assert
	if created.Number != 8 {
		t.Fatalf("CreatePullRequest = %+v, %v; want merge request 8 opened", created, err)
	}

	if !errors.Is(err, forge.ErrSomeReviewersNotAdded) || !errors.Is(err, forge.ErrNoUser) ||
		!strings.Contains(err.Error(), userGhost) {
		t.Errorf("error = %v, want ErrSomeReviewersNotAdded and ErrNoUser naming ghost", err)
	}

	opened := requestTo(*seen, gitlabMergesPath)
	if !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, want only ana's id", opened.body["reviewer_ids"])
	}
}

func TestCreateMergeRequestOnGitLabStillRefusesAnUnknownAssignee(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := scriptedForge(t, gitlabKnowing(nil, nil))

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Assignees: []string{userGhost},
	})

	// Assert
	if !errors.Is(err, forge.ErrNoUser) || errors.Is(err, forge.ErrSomeReviewersNotAdded) || created.Opened() {
		t.Errorf("CreatePullRequest = %+v, %v; want nothing opened and ErrNoUser", created, err)
	}

	if got := requestTo(*seen, gitlabMergesPath); got.method != "" {
		t.Errorf("posted a merge request without its assignee: %+v", got)
	}
}
