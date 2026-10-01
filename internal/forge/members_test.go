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
)

// The control-plane group, as CODEOWNERS names it and as GitLab's API reads
// its members.
const (
	groupControlPlane = "acme/control-plane"
	membersPath       = "/groups/acme%2Fcontrol-plane/members"
)

// memberPage is a page of GitLab members: count active ones named
// <prefix><n>, then one blocked user.
func memberPage(prefix string, count int) string {
	members := make([]string, 0, count+1)
	for index := range count {
		members = append(members,
			fmt.Sprintf(`{"username":"%s%d","state":"active","access_level":30}`, prefix, index))
	}

	members = append(members, `{"username":"gone","state":"blocked"}`)

	return "[" + strings.Join(members, ",") + "]"
}

func TestGroupMembersReadsEveryPageOfAGitLabGroupsActiveMembers(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first page is full — 99 active members and a blocked one — so the
	// second is read too.
	client, seen := scriptedForge(t, func(asked recorded) (int, string) {
		if strings.Contains(asked.query, "page=1&") {
			return http.StatusOK, memberPage("first", 99)
		}

		return http.StatusOK, memberPage("second", 1)
	})

	// Act
	members, err := client.On(forge.KindGitLab).GroupMembers(t.Context(), groupControlPlane)

	// Assert
	if err != nil || len(members) != 100 || members[0] != "first0" || members[99] != "second0" {
		t.Fatalf("GroupMembers = %d members (%v), %v; want the 100 active ones from both pages",
			len(members), members, err)
	}

	if len(*seen) != 2 || (*seen)[0].path != membersPath {
		t.Errorf("requests = %+v, want two pages of %s", *seen, membersPath)
	}
}

func TestGroupMembersLeavesOutMembersWhoCannotApprove(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Guest (10), a Planner (15) and a Reporter (20) cannot approve a merge
	// request; a Developer (30) and above can.
	client, _ := scriptedForge(t, func(recorded) (int, string) {
		return http.StatusOK, `[{"username":"guest","state":"active","access_level":10},` +
			`{"username":"planner","state":"active","access_level":15},` +
			`{"username":"reporter","state":"active","access_level":20},` +
			`{"username":"dev","state":"active","access_level":30},` +
			`{"username":"owner","state":"active","access_level":50}]`
	})

	// Act
	members, err := client.On(forge.KindGitLab).GroupMembers(t.Context(), groupControlPlane)

	// Assert
	if err != nil || !reflect.DeepEqual(members, []string{"dev", "owner"}) {
		t.Errorf("GroupMembers = %v, %v; want dev and owner, who can approve", members, err)
	}
}

func TestGroupMembersIsNotOfferedOnGitHub(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := scriptedForge(t, func(recorded) (int, string) { return http.StatusOK, "[]" })

	// Act
	_, err := client.On(forge.KindGitHub).GroupMembers(t.Context(), groupControlPlane)

	// Assert
	if !errors.Is(err, forge.ErrNotSupported) || len(*seen) != 0 {
		t.Errorf("GroupMembers = %v after %d requests, want ErrNotSupported and none", err, len(*seen))
	}
}

func TestCreateMergeRequestOnGitLabAsksATeamsActiveMembersToReview(t *testing.T) {
	t.Parallel()

	// Arrange
	// ana is named and is in the team too, so she is asked once. Ben is known
	// by the id the members listing gives, so he is never looked up.
	client, seen := scriptedForge(t, gitlabKnowing(
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
	client, seen := scriptedForge(t, func(asked recorded) (int, string) {
		if asked.path == gitlabUserPath {
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
	client, seen := scriptedForge(t, gitlabKnowing(nil, map[string]string{
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
	client, seen := scriptedForge(t, gitlabKnowing(map[string]string{userAna: "7"}, nil))

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

// requestsTo counts the requests made to path.
func requestsTo(seen []recorded, path string) int {
	count := 0

	for _, asked := range seen {
		if asked.path == path {
			count++
		}
	}

	return count
}
