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
		members = append(members, fmt.Sprintf(`{"username":"%s%d","state":"active"}`, prefix, index))
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
	// ana is named and is in the team too, so she is asked once.
	client, seen := scriptedForge(t, gitlabKnowing(
		map[string]string{userAna: "7", userBen: "9"},
		map[string]string{membersPath: `[{"username":"ben","state":"active"},{"username":"ana","state":"active"},` +
			`{"username":"gone","state":"blocked"}]`},
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
	if created.Number != 8 || !errors.Is(err, forge.ErrSomeReviewersNotAdded) ||
		!strings.Contains(err.Error(), groupControlPlane) {
		t.Fatalf("CreatePullRequest = %+v, %v; want it opened, naming the team left off", created, err)
	}

	opened := requestTo(*seen, gitlabMergesPath)
	if !reflect.DeepEqual(opened.body["reviewer_ids"], []any{float64(7)}) {
		t.Errorf("reviewer_ids = %v, want ana's alone", opened.body["reviewer_ids"])
	}
}
