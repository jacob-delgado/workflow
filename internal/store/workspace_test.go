// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// Whom an owner is on Slack, and which groups a repository tags, are kept per
// Slack workspace: a Slack ID means nothing in another workspace, so what was
// linked under one is neither read nor overwritten under another, and is there
// again on switching back. That an owner is not on Slack is about the person,
// so it holds in every workspace.

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// workspaceA and workspaceB are the Slack workspaces the tests switch between.
const (
	workspaceA = "T0AAAAAAA"
	workspaceB = "T0BBBBBBB"
)

// bea is the Slack user ana is in workspace B.
func bea() *store.SlackTarget {
	return &store.SlackTarget{ID: "U0BEA999", Label: "Ana in B"}
}

func TestALinkMadeInOneWorkspaceIsNotReadInAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks in B = %+v, %v; want ana undecided there", links, err)
	}
}

func TestALinkMadeInAnotherWorkspaceLeavesTheFirstOnesLink(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, anaOwner, bea(), theTime())
	if err != nil {
		t.Fatalf("linking ana in B: %v", err)
	}

	inA, errA := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)
	inB, errB := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	wantA := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if errA != nil || !slices.Equal(inA, wantA) {
		t.Errorf("OwnerLinks back in A = %+v, %v; want %+v", inA, errA, wantA)
	}

	wantB := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *bea()}}
	if errB != nil || !slices.Equal(inB, wantB) {
		t.Errorf("OwnerLinks in B = %+v, %v; want %+v", inB, errB, wantB)
	}
}

func TestForgettingAnOwnerInOneWorkspaceKeepsTheirLinkInAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, anaOwner, bea(), theTime())
	if err != nil {
		t.Fatalf("linking ana in B: %v", err)
	}

	// Act
	err = kept.ForgetOwner(t.Context(), forgeHost, workspaceB, anaOwner)
	if err != nil {
		t.Fatalf("ForgetOwner in B returned %v, want nil", err)
	}

	inA, errA := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)
	inB, errB := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	wantA := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if errA != nil || !slices.Equal(inA, wantA) {
		t.Errorf("OwnerLinks in A = %+v, %v; want %+v", inA, errA, wantA)
	}

	if errB != nil || len(inB) != 0 {
		t.Errorf("OwnerLinks in B = %+v, %v; want ana undecided there", inB, errB)
	}
}

func TestAnOwnerNotOnSlackIsNotOnSlackInEveryWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, anaOwner, nil, theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: anaOwner, OnSlack: false, Slack: store.SlackTarget{}}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks in A = %+v, %v; want %+v", links, err, want)
	}
}

func TestTheGroupsOfARepositoryAndItsChoiceAreKeptPerWorkspace(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	listBoth(t, kept)

	err := kept.RecordGroups(t.Context(), repo, workspaceA, []string{podID}, theTime())
	if err != nil {
		t.Fatalf("choosing in A: %v", err)
	}

	group := store.SlackTarget{ID: "S0BGROUP1", Label: "b-reviewers"}

	// Act
	err = kept.SetRepoGroups(t.Context(), repo, workspaceB, []store.SlackTarget{group}, theTime())
	if err != nil {
		t.Fatalf("listing in B: %v", err)
	}

	err = kept.RecordGroups(t.Context(), repo, workspaceB, []string{group.ID}, theTime())
	if err != nil {
		t.Fatalf("choosing in B: %v", err)
	}

	groups, groupsErr := kept.RepoGroups(t.Context(), repo, workspaceA)
	ids, _, choiceErr := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	want := []store.SlackTarget{apiGroup(), *podGroup()}
	if groupsErr != nil || !slices.Equal(groups, want) {
		t.Errorf("RepoGroups back in A = %+v, %v; want %+v", groups, groupsErr, want)
	}

	if choiceErr != nil || !slices.Equal(ids, []string{podID}) {
		t.Errorf("LastGroups back in A = %v, %v; want [%s]", ids, choiceErr, podID)
	}
}

func TestNothingIsKeptOrReadUnderNoWorkspace(t *testing.T) {
	t.Parallel()

	cases := map[string]func(store.Store) error{
		"OwnerLinks": func(kept store.Store) error {
			_, err := kept.OwnerLinks(t.Context(), forgeHost, "")

			return err
		},
		"LinkOwner": func(kept store.Store) error {
			return kept.LinkOwner(t.Context(), forgeHost, "", anaOwner, ana(), theTime())
		},
		"RepoGroups": func(kept store.Store) error {
			_, err := kept.RepoGroups(t.Context(), repo, "")

			return err
		},
		"SetRepoGroups": func(kept store.Store) error {
			return kept.SetRepoGroups(t.Context(), repo, "", []store.SlackTarget{apiGroup()}, theTime())
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept := store.New(t.TempDir(), false)

			// Act
			err := call(kept)

			// Assert
			if !errors.Is(err, store.ErrNoWorkspace) {
				t.Errorf("%s under no workspace = %v, want ErrNoWorkspace", name, err)
			}
		})
	}
}

// keptBeforeWorkspaces leaves in dir the kept file a build before workspaces
// made — both of its migrations' tables — holding ana linked to Ana, dora not
// on Slack, and a repository group listed and chosen.
func keptBeforeWorkspaces(t *testing.T, dir string) {
	t.Helper()

	for _, statement := range []string{
		`CREATE TABLE slack_entity (slack_id TEXT NOT NULL PRIMARY KEY, label TEXT NOT NULL,
			seen_at TEXT NOT NULL) STRICT`,
		`CREATE TABLE owner_decision (forge_host TEXT NOT NULL, owner TEXT NOT NULL, decided_at TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner)) STRICT`,
		`CREATE TABLE owner_slack (forge_host TEXT NOT NULL, owner TEXT NOT NULL, slack_id TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE,
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT) STRICT`,
		`CREATE INDEX owner_slack_by_entity ON owner_slack (slack_id)`,
		`CREATE TABLE repo_group (repo TEXT NOT NULL, slack_id TEXT NOT NULL, added_at TEXT NOT NULL,
			PRIMARY KEY (repo, slack_id),
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT) STRICT`,
		`CREATE INDEX repo_group_by_entity ON repo_group (slack_id)`,
		`CREATE TABLE repo_choice (repo TEXT NOT NULL PRIMARY KEY, chosen_at TEXT NOT NULL) STRICT`,
		`CREATE TABLE repo_choice_group (repo TEXT NOT NULL, slack_id TEXT NOT NULL, PRIMARY KEY (repo, slack_id),
			FOREIGN KEY (repo) REFERENCES repo_choice(repo) ON DELETE CASCADE,
			FOREIGN KEY (repo, slack_id) REFERENCES repo_group(repo, slack_id) ON DELETE CASCADE) STRICT`,
		`INSERT INTO slack_entity VALUES ('U012ABC', 'Ana Lima', '2026-01-02T03:04:05Z'),
			('S0POD123', 'control-plane-pod', '2026-01-02T03:04:05Z')`,
		`INSERT INTO owner_decision VALUES ('github.com', 'ana', '2026-01-02T03:04:05Z'),
			('github.com', 'dora', '2026-01-02T03:04:05Z')`,
		`INSERT INTO owner_slack VALUES ('github.com', 'ana', 'U012ABC')`,
		`INSERT INTO repo_group VALUES ('` + repo + `', 'S0POD123', '2026-01-02T03:04:05Z')`,
		`INSERT INTO repo_choice VALUES ('` + repo + `', '2026-01-02T03:04:05Z')`,
		`INSERT INTO repo_choice_group VALUES ('` + repo + `', 'S0POD123')`,
		`PRAGMA user_version = 2`,
	} {
		execKept(t, dir, statement)
	}
}

func TestAKeptFileFromBeforeWorkspacesAsksAgainWhomEachOwnerIs(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keptBeforeWorkspaces(t, dir)
	kept := store.New(dir, false)

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: "dora", OnSlack: false, Slack: store.SlackTarget{}}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks after migrating = %+v, %v; want ana asked again and dora still not on Slack %+v",
			links, err, want)
	}

	if got := keptPragma(t, dir, "user_version"); got != keptVersion {
		t.Errorf("the migrated file is at version %d, want %d", got, keptVersion)
	}
}

func TestAKeptFileFromBeforeWorkspacesTagsNoGroupItListed(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keptBeforeWorkspaces(t, dir)
	kept := store.New(dir, false)

	// Act
	groups, groupsErr := kept.RepoGroups(t.Context(), repo, workspaceA)
	ids, _, choiceErr := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if groupsErr != nil || len(groups) != 0 {
		t.Errorf("RepoGroups after migrating = %+v, %v; want none until listed again", groups, groupsErr)
	}

	if choiceErr != nil || len(ids) != 0 {
		t.Errorf("LastGroups after migrating = %v, %v; want none", ids, choiceErr)
	}
}

func TestAKeptFileFromBeforeWorkspacesLinksAgainUnderOne(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keptBeforeWorkspaces(t, dir)
	kept := store.New(dir, false)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, anaOwner, bea(), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	want := []store.OwnerLink{
		{Owner: anaOwner, OnSlack: true, Slack: *bea()},
		{Owner: "dora", OnSlack: false, Slack: store.SlackTarget{}},
	}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want %+v", links, err, want)
	}
}

func TestAReadOnlyStoreReadsAKeptFileFromBeforeWorkspacesAsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keptBeforeWorkspaces(t, dir)
	readOnly := store.New(dir, false).ReadOnly()

	// Act
	links, err := readOnly.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks from a file not yet migrated = %+v, %v; want none", links, err)
	}

	if got := keptPragma(t, dir, "user_version"); got != 2 {
		t.Errorf("the read-only store moved the file to version %d, want it left at 2", got)
	}
}
