// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// Whether an owner is a person or a team is the caller's to say, kept with
// the decision: CODEOWNERS spells a top-level GitLab group @acme, as it spells
// a user, so the name alone cannot tell, and a team links to a user group.

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// acme is a bare name the forge may know as a group or as a person, acmePod a
// team CODEOWNERS spells with a slash, and otherForgeHost a host whose owners
// are kept apart from forgeHost's.
const (
	acme           = "acme"
	acmePod        = "acme/pod"
	otherForgeHost = "gitlab.example.com"
)

func TestABareNamedTeamLinksToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, team, theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || !slices.Equal(links, []store.OwnerLink{team}) {
		t.Errorf("OwnerLinks = %+v, %v; want acme a team linked to the pod's group", links, err)
	}
}

func TestTheSameBareNameAsAPersonLinksOnlyToAUser(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	person := store.OwnerLink{Owner: acme, Team: false, OnSlack: true, Slack: *podGroup()}

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, person, theTime())

	// Assert
	if !errors.Is(err, store.ErrInvalidSlackID) {
		t.Errorf("linking the person acme to a user group = %v, want ErrInvalidSlackID", err)
	}
}

// linkAcmeAsAnaInA links acme, as a person, to the Slack user Ana in
// workspace A.
func linkAcmeAsAnaInA(t *testing.T, kept store.Store) {
	t.Helper()

	person := store.OwnerLink{Owner: acme, Team: false, OnSlack: true, Slack: *ana()}

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, person, theTime())
	if err != nil {
		t.Fatalf("linking the person acme in workspace A: %v", err)
	}
}

func TestALinkVoidedByAKindChangeElsewhereStaysGoneWhenTheKindChangesBack(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAcmeAsAnaInA(t, kept)

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, team, theTime())
	if err != nil {
		t.Fatalf("linking the team acme in workspace B: %v", err)
	}

	// Act
	err = kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(acme, bea()), theTime())
	if err != nil {
		t.Fatalf("linking the person acme in workspace B: %v", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks in A = %+v, %v; want none, so acme is asked about again there", links, err)
	}
}

func TestALinkVoidedByAKindChangeElsewhereFreesItsSlackUser(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAcmeAsAnaInA(t, kept)

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, workspaceB, team, theTime())
	// Assert
	if err != nil {
		t.Fatalf("linking the team acme in workspace B: %v", err)
	}

	if got := keptEntities(t, dir); got != 1 {
		t.Errorf("the kept file holds %d Slack entities, want 1: the pod's group, with Ana pruned", got)
	}
}

func TestAKindChangeElsewhereAsksAgainWhetherTheOwnerIsOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(acme, nil), theTime())
	if err != nil {
		t.Fatalf("deciding the person acme is not on Slack in workspace A: %v", err)
	}

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err = kept.LinkOwner(t.Context(), forgeHost, workspaceB, team, theTime())
	if err != nil {
		t.Fatalf("linking the team acme in workspace B: %v", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks in A = %+v, %v; want none, so the team acme is asked about again there", links, err)
	}
}

func TestADecisionOfTheSameKindElsewhereKeepsThatTheOwnerIsNotOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(acme, nil), theTime())
	if err != nil {
		t.Fatalf("deciding the person acme is not on Slack in workspace A: %v", err)
	}

	// Act
	err = kept.LinkOwner(t.Context(), forgeHost, workspaceB, decided(acme, bea()), theTime())
	if err != nil {
		t.Fatalf("linking the person acme in workspace B: %v", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: acme, Team: false, OnSlack: false, Slack: store.SlackTarget{}}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks in A = %+v, %v; want %+v, acme still not on Slack there", links, err, want)
	}
}

func TestAKindChangeOnOneForgeHostKeepsTheOwnersLinksOnAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAcmeAsAnaInA(t, kept)

	err := kept.LinkOwner(t.Context(), otherForgeHost, workspaceB, decided(acme, bea()), theTime())
	if err != nil {
		t.Fatalf("linking the person acme on %s in workspace B: %v", otherForgeHost, err)
	}

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err = kept.LinkOwner(t.Context(), otherForgeHost, workspaceB, team, theTime())
	if err != nil {
		t.Fatalf("linking the team acme on %s in workspace B: %v", otherForgeHost, err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: acme, Team: false, OnSlack: true, Slack: *ana()}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks on %s in A = %+v, %v; want %+v", forgeHost, links, err, want)
	}
}

func TestAKindChangeOnOneForgeHostKeepsThatTheOwnerIsNotOnSlackOnAnother(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	err := kept.LinkOwner(t.Context(), forgeHost, workspaceA, decided(acme, nil), theTime())
	if err != nil {
		t.Fatalf("deciding the person acme is not on Slack on %s in workspace A: %v", forgeHost, err)
	}

	err = kept.LinkOwner(t.Context(), otherForgeHost, workspaceB, decided(acme, bea()), theTime())
	if err != nil {
		t.Fatalf("linking the person acme on %s in workspace B: %v", otherForgeHost, err)
	}

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err = kept.LinkOwner(t.Context(), otherForgeHost, workspaceB, team, theTime())
	if err != nil {
		t.Fatalf("linking the team acme on %s in workspace B: %v", otherForgeHost, err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{{Owner: acme, Team: false, OnSlack: false, Slack: store.SlackTarget{}}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks on %s in A = %+v, %v; want %+v", forgeHost, links, err, want)
	}
}

func TestAKindOnAnotherForgeHostKeepsThatTheOwnerIsNotOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAcmeAsAnaInA(t, kept)

	teamNotOnSlack := store.OwnerLink{Owner: acme, Team: true, OnSlack: false, Slack: store.SlackTarget{}}

	err := kept.LinkOwner(t.Context(), otherForgeHost, workspaceA, teamNotOnSlack, theTime())
	if err != nil {
		t.Fatalf("deciding the team acme is not on Slack on %s in workspace A: %v", otherForgeHost, err)
	}

	team := store.OwnerLink{Owner: acme, Team: true, OnSlack: true, Slack: *podGroup()}

	// Act
	err = kept.LinkOwner(t.Context(), otherForgeHost, workspaceB, team, theTime())
	if err != nil {
		t.Fatalf("linking the team acme on %s in workspace B: %v", otherForgeHost, err)
	}

	links, err := kept.OwnerLinks(t.Context(), otherForgeHost, workspaceA)

	// Assert
	if err != nil || !slices.Equal(links, []store.OwnerLink{teamNotOnSlack}) {
		t.Errorf("OwnerLinks on %s in A = %+v, %v; want %+v: acme is a person only on %s, so no kind changed here",
			otherForgeHost, links, err, teamNotOnSlack, forgeHost)
	}
}
