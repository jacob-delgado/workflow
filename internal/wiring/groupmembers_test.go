// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
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

func TestThereIsNoGroupMembersSeamOnGitHub(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})
	cfg, where := githubCLIWorkspace(t)

	// Act
	forgeSeams := wired(t, cfg, where, nil).Forge

	// Assert
	if forgeSeams.GroupMembers != nil {
		t.Error("GroupMembers is bound on GitHub, whose teams review as teams")
	}
}
