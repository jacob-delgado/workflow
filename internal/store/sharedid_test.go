// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// One Slack ID can be seen from more than one workspace: an Enterprise
// Grid's W user, a Slack Connect member, an org-wide S group. A link, a
// listed group and a chosen group each belong to the workspace they were
// made in, so the same ID kept under another workspace leaves them as they
// were.

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// boOwner is a second forge owner, linked in workspace B.
const boOwner = "bo"

// gridUser is a Slack user every workspace of an Enterprise Grid sees.
func gridUser() *store.SlackTarget {
	return &store.SlackTarget{ID: "W0GRID1", Label: "Grid User"}
}

// orgGroup is an org-wide Slack user group every workspace sees.
func orgGroup() store.SlackTarget {
	return store.SlackTarget{ID: "S0ORG1", Label: "org-reviewers"}
}

// setGroups lists groups for the repository in workspace.
func setGroups(t *testing.T, kept store.Store, workspace string, groups ...store.SlackTarget) {
	t.Helper()

	err := kept.SetRepoGroups(t.Context(), repo, workspace, groups, theTime())
	if err != nil {
		t.Fatalf("listing the groups in %s: %v", workspace, err)
	}
}

// chooseGroups records ids as the repository's last choice in workspace.
func chooseGroups(t *testing.T, kept store.Store, workspace string, ids ...string) {
	t.Helper()

	err := kept.RecordGroups(t.Context(), repo, workspace, ids, theTime())
	if err != nil {
		t.Fatalf("choosing the groups in %s: %v", workspace, err)
	}
}

func TestLinkingASharedIDInAnotherWorkspaceKeepsTheFirstWorkspacesLink(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(anaOwner, gridUser()), theTime())
	if err != nil {
		t.Fatalf("linking ana in A: %v", err)
	}

	// Act
	err = kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(boOwner, gridUser()), theTime())
	if err != nil {
		t.Fatalf("linking bo in B: %v", err)
	}

	inA, errA := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)
	inB, errB := kept.OwnerLinks(t.Context(), forgeHost, workspaceB)

	// Assert
	wantA := []store.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *gridUser()}}
	if errA != nil || !slices.Equal(inA, wantA) {
		t.Errorf("OwnerLinks in A = %+v, %v; want %+v", inA, errA, wantA)
	}

	wantB := []store.OwnerLink{{Owner: boOwner, OnSlack: true, Slack: *gridUser()}}
	if errB != nil || !slices.Equal(inB, wantB) {
		t.Errorf("OwnerLinks in B = %+v, %v; want %+v", inB, errB, wantB)
	}
}

func TestDroppingASharedGroupInAnotherWorkspaceKeepsItListedInTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	setGroups(t, kept, workspaceA, orgGroup())
	setGroups(t, kept, workspaceB, orgGroup())

	// Act
	setGroups(t, kept, workspaceB)

	groups, err := kept.RepoGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || !slices.Equal(groups, []store.SlackTarget{orgGroup()}) {
		t.Errorf("RepoGroups in A = %+v, %v; want the org's group still listed", groups, err)
	}
}

func TestChoosingNoGroupInAnotherWorkspaceKeepsTheFirstWorkspacesChoice(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	setGroups(t, kept, workspaceA, orgGroup())
	chooseGroups(t, kept, workspaceA, orgGroup().ID)
	setGroups(t, kept, workspaceB, orgGroup())

	// Act
	chooseGroups(t, kept, workspaceB)

	ids, chosen, err := kept.LastGroups(t.Context(), repo, workspaceA)

	// Assert
	if err != nil || !chosen || !slices.Equal(ids, []string{orgGroup().ID}) {
		t.Errorf("LastGroups in A = %v, %v, %v; want the org's group still chosen", ids, chosen, err)
	}
}
