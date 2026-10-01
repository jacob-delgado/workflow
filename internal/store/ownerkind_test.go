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

// acme is a bare name the forge may know as a group or as a person, and
// acmePod a team CODEOWNERS spells with a slash.
const (
	acme    = "acme"
	acmePod = "acme/pod"
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
