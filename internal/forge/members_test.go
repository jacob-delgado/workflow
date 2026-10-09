// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

// The control-plane group, as CODEOWNERS names it and as GitLab's API reads
// its members.
const (
	groupControlPlane = "acme/control-plane"
	membersPath       = "/groups/acme%2Fcontrol-plane/members"
)

// memberPage is a page of GitLab members: count active ones whose ids run on
// from firstID, then one blocked user.
func memberPage(firstID, count int) string {
	members := make([]string, 0, count+1)
	for index := range count {
		members = append(members,
			fmt.Sprintf(`{"id":%d,"username":"member%d","state":"active","access_level":30}`, firstID+index, index))
	}

	members = append(members, `{"id":0,"username":"gone","state":"blocked"}`)

	return "[" + strings.Join(members, ",") + "]"
}

func TestCreateMergeRequestOnGitLabAsksATeamFromEveryPageOfItsMembers(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first page is full — 99 active members and a blocked one — so the
	// second is read too.
	knowing := gitlabKnowing(nil, nil)
	client, seen := recordingForge(t, func(asked recorded) (int, string) {
		switch {
		case asked.path == membersPath && strings.Contains(asked.query, "page=1&"):
			return http.StatusOK, memberPage(1, 99)
		case asked.path == membersPath:
			return http.StatusOK, memberPage(100, 1)
		}

		return knowing(asked)
	})

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	reviewers, _ := requestTo(*seen, gitlabMergesPath).body["reviewer_ids"].([]any)
	if err != nil || len(reviewers) != 100 || reviewers[0] != float64(1) || reviewers[99] != float64(100) {
		t.Fatalf("reviewer_ids = %v, %v; want the 100 active members from both pages", reviewers, err)
	}

	if pages := requestsTo(*seen, membersPath); pages != 2 {
		t.Errorf("read %d pages of %s, want two", pages, membersPath)
	}
}

func TestCreateMergeRequestOnGitLabLeavesOutTeamMembersWhoCannotApprove(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Guest (10), a Planner (15) and a Reporter (20) cannot approve a merge
	// request; a Developer (30) and above can.
	client, seen := recordingForge(t, gitlabKnowing(nil, map[string]string{
		membersPath: `[{"id":1,"username":"guest","state":"active","access_level":10},` +
			`{"id":2,"username":"planner","state":"active","access_level":15},` +
			`{"id":3,"username":"reporter","state":"active","access_level":20},` +
			`{"id":4,"username":"dev","state":"active","access_level":30},` +
			`{"id":5,"username":"owner","state":"active","access_level":50}]`,
	}))

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	opened := requestTo(*seen, gitlabMergesPath)
	if err != nil || !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(4), float64(5)}) {
		t.Errorf("reviewer_ids = %v, %v; want dev's and owner's, who can approve", opened.body["reviewer_ids"], err)
	}
}

func TestCreateMergeRequestOnGitLabLeavesOutMembersWhoseMembershipIsNotActive(t *testing.T) {
	t.Parallel()

	// Arrange
	// An invitation not yet accepted is a membership awaiting; one GitLab
	// says nothing of is taken as standing.
	client, seen := recordingForge(t, gitlabKnowing(nil, map[string]string{
		membersPath: `[{"id":1,"username":"joined","state":"active","membership_state":"active","access_level":30},` +
			`{"id":2,"username":"invited","state":"active","membership_state":"awaiting","access_level":30},` +
			`{"id":3,"username":"unsaid","state":"active","access_level":30}]`,
	}))

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	opened := requestTo(*seen, gitlabMergesPath)
	if err != nil || !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(1), float64(3)}) {
		t.Errorf("reviewer_ids = %v, %v; want the joined member's and the one GitLab said nothing of",
			opened.body["reviewer_ids"], err)
	}
}

func TestCreateMergeRequestOnGitLabAsksATeamsActiveMembersToReview(t *testing.T) {
	t.Parallel()

	// Arrange
	// ana is named and is in the team too, so she is asked once. Ben is known
	// by the id the members listing gives, so he is never looked up.
	client, seen := recordingForge(t, gitlabKnowing(
		map[string]string{userAna: "7"},
		map[string]string{membersPath: `[{"id":9,"username":"ben","state":"active","access_level":30},` +
			`{"id":7,"username":"ana","state":"active","access_level":40},` +
			`{"id":11,"username":"gone","state":"blocked","access_level":30}]`},
	))

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna}, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	if err != nil || created.Number != 8 {
		t.Fatalf("CreatePullRequest = %+v, %v", created, err)
	}

	opened := requestTo(*seen, gitlabMergesPath)
	if !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7), float64(9)}) {
		t.Errorf("reviewer_ids = %v, want ana's then ben's", opened.body["reviewer_ids"])
	}

	if lookups := requestsTo(*seen, gitlabUsersPath); lookups != 1 {
		t.Errorf("looked up %d users, want ana alone: a member's id comes with the listing", lookups)
	}
}

func TestCreateMergeRequestOnGitLabLeavesTheAuthorOutOfATeam(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token is ben's, and ben is in the team he asks to review.
	knowing := gitlabKnowing(nil, map[string]string{
		membersPath: `[{"id":9,"username":"ben","state":"active","access_level":30},` +
			`{"id":7,"username":"ana","state":"active","access_level":30}]`,
	})
	client, seen := recordingForge(t, func(asked recorded) (int, string) {
		if asked.path == userPath {
			return http.StatusOK, `{"id":9,"username":"ben"}`
		}

		return knowing(asked)
	})

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	opened := requestTo(*seen, gitlabMergesPath)
	if err != nil || !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, %v; want ana's alone, not the author's own", opened.body["reviewer_ids"], err)
	}
}

func TestCreateMergeRequestOnGitLabExpandsATopLevelGroupNamedAsAUser(t *testing.T) {
	t.Parallel()

	// Arrange
	// CODEOWNERS spells a top-level group @platform, just as it spells a user;
	// GitLab knows no user by that name, but has the group.
	client, seen := recordingForge(t, gitlabKnowing(nil, map[string]string{
		"/groups/platform/members": `[{"id":7,"username":"ana","state":"active","access_level":30}]`,
	}))

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{"platform"},
	})

	// Assert
	opened := requestTo(*seen, gitlabMergesPath)
	if err != nil || !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, %v; want the platform group's ana", opened.body["reviewer_ids"], err)
	}
}

func TestCreateMergeRequestOnGitLabOpensWithoutATeamItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab has no such group, so it answers 404 for its members.
	client, seen := recordingForge(t, gitlabKnowing(map[string]string{userAna: "7"}, nil))

	// Act
	created, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch,
		Reviewers: []string{userAna}, TeamReviewers: []string{groupControlPlane},
	})

	// Assert
	if created.Number != 8 || !errors.Is(err, forge.ErrSomePeopleNotAdded) ||
		!strings.Contains(err.Error(), groupControlPlane) {
		t.Fatalf("CreatePullRequest = %+v, %v; want it opened, naming the team left off", created, err)
	}

	opened := requestTo(*seen, gitlabMergesPath)
	if !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, want ana's alone", opened.body["reviewer_ids"])
	}
}

func TestCreateMergeRequestOnGitLabKeepsWhyAGroupNamedAsAUserWasNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab knows no user named platform, and asks to wait when its group is
	// read: a reason to try again, not a name mistyped.
	knowing := gitlabKnowing(nil, nil)
	client, _ := recordingForge(t, func(asked recorded) (int, string) {
		if asked.path == "/groups/platform/members" {
			return http.StatusTooManyRequests, `{"message":"Retry later"}`
		}

		return knowing(asked)
	})

	// Act
	_, err := client.CreatePullRequest(t.Context(), gitlabRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{"platform"},
	})

	// Assert
	if !errors.Is(err, forge.ErrSomePeopleNotAdded) || !errors.Is(err, httpx.ErrRateLimited) ||
		errors.Is(err, forge.ErrNoUser) {
		t.Errorf("CreatePullRequest = %v; want platform missed for the rate limit, not as no such user", err)
	}
}
