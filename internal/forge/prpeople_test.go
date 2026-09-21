// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The issue-side paths GitHub adds assignees and labels through, and the pull's
// reviewers path, named so a repeated literal does not read as coincidence.
const (
	githubPRIssuePath   = "/repos/example/repo/issues/43"
	githubReviewersPath = githubPullsPath + "/43/requested_reviewers"
	gitlabUsersPath     = "/users"

	userAna  = "ana"
	userBen  = "ben"
	userCass = "cass"

	labelBug    = "bug"
	labelReview = "review"
)

// forgeConversation serves each path its mapped answer and records every
// request with its decoded JSON body. A path in fails answers 403 instead, to
// try a step an under-scoped token cannot make.
func forgeConversation(t *testing.T, answers map[string]string, fails map[string]bool) (forge.Client, *[]recorded) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []recorded
	)

	note := func(request *http.Request, body map[string]any) {
		lock.Lock()
		defer lock.Unlock()

		seen = append(seen, recorded{
			method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery, body: body,
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any

		_ = json.NewDecoder(request.Body).Decode(&body)
		note(request, body)

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")

		if fails[request.URL.EscapedPath()] {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`{"message":"Resource not accessible by personal access token"}`))

			return
		}

		answer, ok := answers[request.URL.EscapedPath()]
		if !ok {
			answer = "{}"
		}

		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret), &seen
}

func TestCreatePullRequestOnGitHubAddsReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeConversation(t, map[string]string{
		githubPullsPath: `{"number":43,"html_url":"https://github.com/example/repo/pull/43","title":"fix: token"}`,
	}, nil)

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna, userBen}, Assignees: []string{userCass}, Labels: []string{labelBug, labelReview},
	})

	// Assert
	if err != nil || created.Number != 43 {
		t.Fatalf("CreatePullRequest = %+v, %v", created, err)
	}

	reviewers := requestTo(*seen, githubReviewersPath)
	if reviewers.method != http.MethodPost || !reflect.DeepEqual(reviewers.body["reviewers"], []any{userAna, userBen}) {
		t.Errorf("reviewers request = %+v, want a POST of ana and ben", reviewers)
	}

	assignees := requestTo(*seen, githubPRIssuePath+"/assignees")
	if !reflect.DeepEqual(assignees.body["assignees"], []any{userCass}) {
		t.Errorf("assignees body = %+v, want cass", assignees.body)
	}

	labels := requestTo(*seen, githubPRIssuePath+"/labels")
	if !reflect.DeepEqual(labels.body["labels"], []any{labelBug, labelReview}) {
		t.Errorf("labels body = %+v, want bug and review", labels.body)
	}
}

func TestCreatePullRequestOnGitHubAsksForNoOneWhenNoneAreNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeConversation(t, map[string]string{
		githubPullsPath: `{"number":43,"html_url":"https://github.com/example/repo/pull/43"}`,
	}, nil)

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
	})
	// Assert
	// With nobody named, only the pull is opened — no empty reviewer request.
	if err != nil {
		t.Fatalf("CreatePullRequest returned %v", err)
	}

	if got := requestTo(*seen, githubReviewersPath); got.method != "" {
		t.Errorf("asked for reviewers with none named: %+v", got)
	}
}

func TestCreatePullRequestOnGitHubKeepsThePullWhenReviewersAreRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token can open the pull but not request reviewers, the way an
	// under-scoped credential answers.
	client, _ := forgeConversation(t,
		map[string]string{githubPullsPath: `{"number":43,"html_url":"https://github.com/example/repo/pull/43"}`},
		map[string]bool{githubReviewersPath: true})

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{userAna},
	})

	// Assert
	// The pull is opened and returned; the refusal is reported, not swallowed.
	if !created.Opened() || created.Number != 43 {
		t.Fatalf("CreatePullRequest lost the pull: %+v", created)
	}

	if !errors.Is(err, forge.ErrRefused) {
		t.Errorf("error = %v, want ErrRefused for the reviewers", err)
	}
}

func TestCreateMergeRequestOnGitLabSetsReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab keys reviewers and assignees by id, so each username is looked up
	// first; this instance answers every lookup with the same user.
	client, seen := forgeConversation(t, map[string]string{
		gitlabUsersPath:  `[{"id":7}]`,
		gitlabMergesPath: `{"iid":8,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/8"}`,
	}, nil)

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna}, Assignees: []string{userCass}, Labels: []string{labelBug, labelReview},
	})

	// Assert
	if err != nil || created.Number != 8 {
		t.Fatalf("CreatePullRequest = %+v, %v", created, err)
	}

	if lookup := requestTo(*seen, gitlabUsersPath); lookup.query != "username=ana" {
		t.Errorf("first user lookup query = %q, want username=ana", lookup.query)
	}

	opened := requestTo(*seen, gitlabMergesPath)
	if !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, want the resolved id", opened.body["reviewer_ids"])
	}

	if !reflect.DeepEqual(opened.body["assignee_ids"], []any{float64(7)}) {
		t.Errorf("assignee_ids = %v, want the resolved id", opened.body["assignee_ids"])
	}

	if opened.body["labels"] != "bug,review" {
		t.Errorf("labels = %v, want the comma-joined names", opened.body["labels"])
	}
}

func TestCreateMergeRequestOnGitLabRefusesAnUnknownReviewer(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab's user lookup finds nobody, so there is no id to set and no merge
	// request is opened.
	client, seen := forgeConversation(t, map[string]string{gitlabUsersPath: `[]`}, nil)

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{"ghost"},
	})

	// Assert
	if !errors.Is(err, forge.ErrNoUser) {
		t.Errorf("error = %v, want ErrNoUser", err)
	}

	if created.Opened() {
		t.Errorf("opened a merge request despite the unknown reviewer: %+v", created)
	}

	if got := requestTo(*seen, gitlabMergesPath); got.method != "" {
		t.Errorf("posted a merge request anyway: %+v", got)
	}
}
