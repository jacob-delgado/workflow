// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
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

// githubPull43 is GitHub's answer to opening pull request 43, with nothing
// but its number and address.
const githubPull43 = `{"number":43,"html_url":"https://github.com/example/repo/pull/43"}`

func TestCreatePullRequestOnGitHubAddsReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, conversation(map[string]string{
		githubPullsPath: `{"number":43,"html_url":"https://github.com/example/repo/pull/43","title":"fix: token"}`,
	}, nil))

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
	client, seen := recordingForge(t, conversation(map[string]string{
		githubPullsPath: githubPull43,
	}, nil))

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
	client, _ := recordingForge(t, conversation(
		map[string]string{githubPullsPath: githubPull43},
		map[string]bool{githubReviewersPath: true}))

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
	client, seen := recordingForge(t, conversation(map[string]string{
		gitlabUsersPath:  `[{"id":7}]`,
		gitlabMergesPath: `{"iid":8,"web_url":"https://gitlab.com/group/sub/repo/-/merge_requests/8"}`,
	}, nil))

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

func TestCreatePullRequestOnGitHubStopsAtAssigneesTheForgeRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	// Reviewers are requested, then the token may not assign anyone.
	client, seen := recordingForge(t, conversation(
		map[string]string{githubPullsPath: githubPull43},
		map[string]bool{githubPRIssuePath + "/assignees": true}))

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna}, Assignees: []string{userCass}, Labels: []string{labelBug},
	})

	// Assert
	// The pull is kept and the refusal reported; the labels after it are not
	// sent.
	if created.Number != 43 || !errors.Is(err, forge.ErrRefused) {
		t.Fatalf("CreatePullRequest = %+v, %v; want the pull kept and ErrRefused for the assignees", created, err)
	}

	if got := requestTo(*seen, githubPRIssuePath+"/labels"); got.method != "" {
		t.Errorf("labeled the pull after its assignees were refused: %+v", got)
	}
}

func TestCreatePullRequestOnGitHubRequestsTeamsBySlugAlongsideUsers(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, conversation(map[string]string{githubPullsPath: githubPull43}, nil))

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna}, TeamReviewers: []string{"example/control-plane"},
	})
	// Assert
	if err != nil {
		t.Fatalf("CreatePullRequest returned %v", err)
	}

	asked := requestTo(*seen, githubReviewersPath)
	if !reflect.DeepEqual(asked.body["reviewers"], []any{userAna}) ||
		!reflect.DeepEqual(asked.body["team_reviewers"], []any{"control-plane"}) {
		t.Errorf("reviewers body = %+v, want ana and the control-plane team's slug", asked.body)
	}
}

func TestCreatePullRequestOnGitHubNeverRequestsATeamOfAnotherOrganization(t *testing.T) {
	t.Parallel()

	// Arrange
	// The repository is example's; other-org's reviewers team shares a slug
	// with no team of example's that was meant.
	client, seen := recordingForge(t, conversation(map[string]string{githubPullsPath: githubPull43}, nil))

	// Act
	created, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		TeamReviewers: []string{"other-org/reviewers", "Example/api"},
	})

	// Assert
	if created.Number != 43 || !errors.Is(err, forge.ErrSomePeopleNotAdded) ||
		!strings.Contains(err.Error(), "other-org/reviewers") {
		t.Fatalf("CreatePullRequest = %+v, %v; want it opened, naming other-org's team as left off", created, err)
	}

	asked := requestTo(*seen, githubReviewersPath)
	if !reflect.DeepEqual(asked.body["team_reviewers"], []any{apiSlug}) {
		t.Errorf("team_reviewers = %v, want example's api team alone", asked.body["team_reviewers"])
	}
}

func TestCreatePullRequestOnGitHubRequestsOnlyTeamsWhenNoUserIsNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, conversation(map[string]string{githubPullsPath: githubPull43}, nil))

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, TeamReviewers: []string{"example/api"},
	})

	// Assert
	asked := requestTo(*seen, githubReviewersPath)
	if _, named := asked.body["reviewers"]; err != nil || named || asked.body["team_reviewers"] == nil {
		t.Errorf("reviewers body = %+v, err %v; want only team_reviewers", asked.body, err)
	}
}
