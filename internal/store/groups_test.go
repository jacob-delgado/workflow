// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// Each repository keeps the Slack user groups it may tag, and the ones last
// chosen for an announcement, so the next one starts from that choice.

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// apiGroup is a second Slack user group a repository may tag.
func apiGroup() store.SlackTarget {
	return store.SlackTarget{ID: apiID, Label: "api-reviewers"}
}

// listBoth gives the repository both groups.
func listBoth(t *testing.T, kept store.Store) {
	t.Helper()

	err := kept.SetRepoGroups(t.Context(), repo, workspaceA, []store.SlackTarget{*podGroup(), apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("listing the repository's groups: %v", err)
	}
}

func TestTheGroupsOfARepositoryRoundTripByLabel(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	// Act
	groups, err := kept.RepoGroups(t.Context(), repo, workspaceA)

	// Assert
	want := []store.SlackTarget{apiGroup(), *podGroup()}
	if err != nil || !slices.Equal(groups, want) {
		t.Errorf("RepoGroups = %+v, %v; want %+v", groups, err, want)
	}
}

func TestSettingTheGroupsOfARepositoryReplacesThem(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	// Act
	err := kept.SetRepoGroups(t.Context(), repo, workspaceA, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	groups, err := kept.RepoGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || !slices.Equal(groups, []store.SlackTarget{apiGroup()}) {
		t.Errorf("RepoGroups = %+v, %v; want the API group alone", groups, err)
	}
}

func TestRepositoryGroupsAreKeptPerRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	// Act
	groups, err := kept.RepoGroups(t.Context(), "github.com/example/other", workspaceA)

	// Assert
	if err != nil || len(groups) != 0 {
		t.Errorf("RepoGroups for another repository = %+v, %v; want none", groups, err)
	}
}

func TestAUserIsRefusedAsARepositoryGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.SetRepoGroups(t.Context(), repo, workspaceA, []store.SlackTarget{*ana()}, theTime())

	// Assert
	if !errors.Is(err, store.ErrInvalidSlackID) {
		t.Errorf("SetRepoGroups with a user = %v, want ErrInvalidSlackID", err)
	}
}

func TestNoGroupsAreChosenUntilAChoiceIsRecorded(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	// Act
	ids, chosen, err := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || chosen || len(ids) != 0 {
		t.Errorf("LastGroups = %v, %v, %v; want no choice yet", ids, chosen, err)
	}
}

func TestTheLastChoiceOfGroupsRoundTrips(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"one group":             {podID},
		"no group, on purpose":  {},
		"both groups, in order": {apiID, podID},
	}

	for name, choice := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept := store.New(t.TempDir(), false)
			listBoth(t, kept)

			// Act
			err := kept.RecordGroups(t.Context(), repo, workspaceA, choice, theTime())
			if err != nil {
				t.Fatalf("RecordGroups returned %v, want nil", err)
			}

			ids, chosen, err := kept.LastGroups(t.Context(), repo, workspaceA)

			// Assert
			if err != nil || !chosen || !slices.Equal(ids, choice) {
				t.Errorf("LastGroups = %v, %v, %v; want %v chosen", ids, chosen, err, choice)
			}
		})
	}
}

func TestAChosenGroupNotListedIsLeftOutOfTheChoice(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	err := kept.SetRepoGroups(t.Context(), repo, workspaceA, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("listing the repository's group: %v", err)
	}

	// Act
	err = kept.RecordGroups(t.Context(), repo, workspaceA, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("RecordGroups returned %v, want nil", err)
	}

	ids, chosen, err := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || !chosen || !slices.Equal(ids, []string{apiID}) {
		t.Errorf("LastGroups = %v, %v, %v; want only the listed group chosen", ids, chosen, err)
	}
}

func TestAGroupDroppedFromTheListLeavesTheLastChoice(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	err := kept.RecordGroups(t.Context(), repo, workspaceA, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("recording the choice: %v", err)
	}

	// Act
	err = kept.SetRepoGroups(t.Context(), repo, workspaceA, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	ids, chosen, err := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || !chosen || !slices.Equal(ids, []string{apiID}) {
		t.Errorf("LastGroups = %v, %v, %v; want the API group still chosen", ids, chosen, err)
	}
}

func TestAStoredGroupOfTheWrongShapeIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	listBoth(t, kept)

	err := kept.RecordGroups(t.Context(), repo, workspaceA, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("recording the choice: %v", err)
	}

	execKept(t, dir, `PRAGMA foreign_keys = OFF`)
	execKept(t, dir, `UPDATE repo_group SET slack_id = 'U0POD123' WHERE slack_id = 'S0POD123'`)
	execKept(t, dir, `UPDATE repo_choice_group SET slack_id = 'U0POD123' WHERE slack_id = 'S0POD123'`)

	// Act
	groups, groupsErr := kept.RepoGroups(t.Context(), repo, workspaceA)
	ids, _, choiceErr := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if groupsErr != nil || !slices.Equal(groups, []store.SlackTarget{apiGroup()}) {
		t.Errorf("RepoGroups = %+v, %v; want the untampered API group alone", groups, groupsErr)
	}

	if choiceErr != nil || !slices.Equal(ids, []string{apiID}) {
		t.Errorf("LastGroups = %v, %v; want the untampered API group alone", ids, choiceErr)
	}
}

func TestAGroupAnOwnerStillLinksToIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(acmePod, podGroup()), theTime())
	if err != nil {
		t.Fatalf("linking the team: %v", err)
	}

	listBoth(t, kept)

	// Act
	err = kept.SetRepoGroups(t.Context(), repo, workspaceA, nil, theTime())
	// Assert
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	if got := keptEntities(t, dir); got != 1 {
		t.Errorf("the kept file holds %d Slack entities, want the team's group alone", got)
	}
}
