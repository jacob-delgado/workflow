// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheGroupMembersSeamListsAGitLabGroupsActiveMembers(t *testing.T) {
	// Arrange
	glab := installForgeCLI(t, "glab", forgeReplies{})
	cfg := config.Config{Forge: config.Forge{CLI: true}}
	where := wiring.Workspace{Root: t.TempDir(), Remote: remoteGitLab}

	forgeSeams := wired(t, cfg, where, nil).Forge

	// Act
	members, err := forgeSeams.GroupMembers("acme/control-plane")

	// Assert
	if err != nil || !reflect.DeepEqual(members, []string{"dan"}) {
		t.Fatalf("GroupMembers = %v, %v; want the active dan alone", members, err)
	}

	if args := glab.args(); !containsAll(args, "groups/acme%2Fcontrol-plane/members?page=1&per_page=100") {
		t.Errorf("glab was called as %v, want the group's first page of members", args)
	}
}

func TestGroupMembersOnGitHubAsksNothing(t *testing.T) {
	// Arrange
	githubCLI := installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)
	forgeSeams := wired(t, cfg, where, nil).Forge

	// Act
	_, err := forgeSeams.GroupMembers("acme")

	// Assert
	if !errors.Is(err, forge.ErrNotSupported) {
		t.Errorf("GroupMembers on GitHub = %v, want %v: its teams review as teams", err, forge.ErrNotSupported)
	}

	if args := githubCLI.args(); len(args) != 0 {
		t.Errorf("gh was called as %v, want GitHub never asked for a group's members", args)
	}
}

func TestIsGroupOnGitHubTakesEveryBareNameForAPersonWithoutAsking(t *testing.T) {
	// Arrange
	githubCLI := installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)
	forgeSeams := wired(t, cfg, where, nil).Forge

	// Act
	group, err := forgeSeams.IsGroup("acme")

	// Assert
	if err != nil || group {
		t.Errorf("IsGroup(acme) on GitHub = %v, %v; want a person, since its teams are spelled org/team", group, err)
	}

	if args := githubCLI.args(); len(args) != 0 {
		t.Errorf("gh was called as %v, want GitHub never asked whether a name is a group", args)
	}
}
