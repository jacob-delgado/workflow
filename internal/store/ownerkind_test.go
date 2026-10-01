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

// keptBeforeOwnerKinds leaves in dir the kept file a build before owner kinds
// made — its three migrations' tables — holding ana linked to Ana and the
// acme/pod team linked to the pod's group, both in workspace A.
func keptBeforeOwnerKinds(t *testing.T, dir string) {
	t.Helper()

	keptBeforeWorkspaces(t, dir)

	for _, statement := range []string{
		`ALTER TABLE slack_entity ADD COLUMN slack_team TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE owner_slack_per_workspace (forge_host TEXT NOT NULL, owner TEXT NOT NULL,
			slack_id TEXT NOT NULL, PRIMARY KEY (forge_host, owner, slack_id),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE,
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT) STRICT`,
		`INSERT INTO owner_slack_per_workspace SELECT forge_host, owner, slack_id FROM owner_slack`,
		`DROP TABLE owner_slack`,
		`ALTER TABLE owner_slack_per_workspace RENAME TO owner_slack`,
		`CREATE INDEX owner_slack_by_entity ON owner_slack (slack_id)`,
		`UPDATE slack_entity SET slack_team = '` + workspaceA + `'`,
		`INSERT INTO owner_decision VALUES ('github.com', 'acme/pod', '2026-01-02T03:04:05Z')`,
		`INSERT INTO owner_slack VALUES ('github.com', 'acme/pod', 'S0POD123')`,
		`PRAGMA user_version = 3`,
	} {
		execKept(t, dir, statement)
	}
}

func TestAKeptFileFromBeforeOwnerKindsKeepsEachOwnersKind(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keptBeforeOwnerKinds(t, dir)

	// Act
	links, err := store.New(dir, false).OwnerLinks(t.Context(), forgeHost, workspaceA)

	// Assert
	want := []store.OwnerLink{
		{Owner: acmePod, Team: true, OnSlack: true, Slack: store.SlackTarget{ID: podID, Label: "control-plane-pod"}},
		{Owner: anaOwner, Team: false, OnSlack: true, Slack: *ana()},
		{Owner: doraOwner, Team: false, OnSlack: false, Slack: store.SlackTarget{}},
	}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks after migrating = %+v, %v; want %+v", links, err, want)
	}
}
