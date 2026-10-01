// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// Whom an owner is on Slack, and which groups a repository tags, are kept per
// Slack workspace: a Slack ID means nothing in another workspace, so what was
// linked under one is neither read nor overwritten under another, and is there
// again on switching back. That an owner is not on Slack is kept per
// workspace too, since the same person may be in one workspace and not
// another; one decided not on Slack before workspaces were is not on Slack in
// any until decided again.

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
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(anaOwner, bea()), theTime())
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

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(anaOwner, bea()), theTime())
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

func TestAnOwnerNotOnSlackInOneWorkspaceKeepsTheirLinkInAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(anaOwner, nil), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	inA, errA := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)
	inB, errB := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	wantA := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if errA != nil || !slices.Equal(inA, wantA) {
		t.Errorf("OwnerLinks in A = %+v, %v; want %+v", inA, errA, wantA)
	}

	wantB := []store.OwnerLink{{Owner: anaOwner, OnSlack: false, Slack: store.SlackTarget{}}}
	if errB != nil || !slices.Equal(inB, wantB) {
		t.Errorf("OwnerLinks in B = %+v, %v; want %+v", inB, errB, wantB)
	}
}

func TestAnOwnerNotOnSlackInOneWorkspaceIsAskedAboutInAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(anaOwner, nil), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks in A = %+v, %v; want ana undecided there", links, err)
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
			return kept.LinkOwner(t.Context(), forgeHost, "", decided(anaOwner, ana()), theTime())
		},
		"LinkOwner not on Slack": func(kept store.Store) error {
			return kept.LinkOwner(t.Context(), forgeHost, "", decided(anaOwner, nil), theTime())
		},
		"RepoGroups": func(kept store.Store) error {
			_, err := kept.RepoGroups(t.Context(), repo, "")

			return err
		},
		"SetRepoGroups": func(kept store.Store) error {
			return kept.SetRepoGroups(t.Context(), repo, "", []store.SlackTarget{apiGroup()}, theTime())
		},
		"ForgetOwner": func(kept store.Store) error { return kept.ForgetOwner(t.Context(), forgeHost, "", anaOwner) },
		"LastGroups": func(kept store.Store) error {
			_, _, err := kept.LastGroups(t.Context(), repo, "")

			return err
		},
		"RecordGroups": func(kept store.Store) error {
			return kept.RecordGroups(t.Context(), repo, "", []string{apiID}, theTime())
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
