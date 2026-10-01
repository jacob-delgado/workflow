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

// keptVersion is how many migrations this build's kept file holds; it moves
// with keptMigrations.
const keptVersion = 2

// apiGroup is a second Slack user group a repository may tag.
func apiGroup() store.SlackTarget {
	return store.SlackTarget{ID: apiID, Label: "api-reviewers"}
}

// listBoth gives the repository both groups.
func listBoth(t *testing.T, kept store.Store) {
	t.Helper()

	err := kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{*podGroup(), apiGroup()}, theTime())
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
	groups, err := kept.RepoGroups(t.Context(), repo)

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
	err := kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	groups, err := kept.RepoGroups(t.Context(), repo)

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
	groups, err := kept.RepoGroups(t.Context(), "github.com/example/other")

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
	err := kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{*ana()}, theTime())

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
	ids, chosen, err := kept.LastGroups(t.Context(), repo)

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
			err := kept.RecordGroups(t.Context(), repo, choice, theTime())
			if err != nil {
				t.Fatalf("RecordGroups returned %v, want nil", err)
			}

			ids, chosen, err := kept.LastGroups(t.Context(), repo)

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

	err := kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("listing the repository's group: %v", err)
	}

	// Act
	err = kept.RecordGroups(t.Context(), repo, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("RecordGroups returned %v, want nil", err)
	}

	ids, chosen, err := kept.LastGroups(t.Context(), repo)

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

	err := kept.RecordGroups(t.Context(), repo, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("recording the choice: %v", err)
	}

	// Act
	err = kept.SetRepoGroups(t.Context(), repo, []store.SlackTarget{apiGroup()}, theTime())
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	ids, chosen, err := kept.LastGroups(t.Context(), repo)

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

	err := kept.RecordGroups(t.Context(), repo, []string{apiID, podID}, theTime())
	if err != nil {
		t.Fatalf("recording the choice: %v", err)
	}

	execKept(t, dir, `PRAGMA foreign_keys = OFF`)
	execKept(t, dir, `UPDATE repo_group SET slack_id = 'U0POD123' WHERE slack_id = 'S0POD123'`)
	execKept(t, dir, `UPDATE repo_choice_group SET slack_id = 'U0POD123' WHERE slack_id = 'S0POD123'`)

	// Act
	groups, groupsErr := kept.RepoGroups(t.Context(), repo)
	ids, _, choiceErr := kept.LastGroups(t.Context(), repo)

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

	err := kept.LinkOwner(t.Context(), forgeHost, "acme/pod", podGroup(), theTime())
	if err != nil {
		t.Fatalf("linking the team: %v", err)
	}

	listBoth(t, kept)

	// Act
	err = kept.SetRepoGroups(t.Context(), repo, nil, theTime())
	// Assert
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	if got := keptEntities(t, dir); got != 1 {
		t.Errorf("the kept file holds %d Slack entities, want the team's group alone", got)
	}
}

// atVersionOne turns a current kept file back into what the first migration
// alone made, keeping its rows.
func atVersionOne(t *testing.T, dir string) {
	t.Helper()

	for _, statement := range []string{
		`DROP TABLE repo_choice_group`, `DROP TABLE repo_choice`, `DROP TABLE repo_group`,
		`PRAGMA user_version = 1`,
	} {
		execKept(t, dir, statement)
	}
}

func TestAnOlderKeptFileMigratesAndKeepsItsRows(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)
	atVersionOne(t, dir)

	// Act
	listBoth(t, kept)

	// Assert
	links, err := kept.OwnerLinks(t.Context(), forgeHost)
	if err != nil || len(links) != 1 {
		t.Errorf("OwnerLinks after migrating = %+v, %v; want ana still linked", links, err)
	}

	if got := keptPragma(t, dir, "user_version"); got != keptVersion {
		t.Errorf("the migrated file is at version %d, want %d", got, keptVersion)
	}
}

func TestAReadOnlyStoreReadsAnOlderKeptFileAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	linkAna(t, store.New(dir, false))
	atVersionOne(t, dir)
	readOnly := store.New(dir, false).ReadOnly()

	// Act
	groups, groupsErr := readOnly.RepoGroups(t.Context(), repo)
	links, linksErr := readOnly.OwnerLinks(t.Context(), forgeHost)

	// Assert
	if groupsErr != nil || len(groups) != 0 {
		t.Errorf("RepoGroups from a file not yet migrated = %+v, %v; want none", groups, groupsErr)
	}

	if linksErr != nil || len(links) != 1 {
		t.Errorf("OwnerLinks from a file not yet migrated = %+v, %v; want ana's link", links, linksErr)
	}

	if got := keptPragma(t, dir, "user_version"); got != 1 {
		t.Errorf("the read-only store moved the file to version %d, want it left at 1", got)
	}
}
