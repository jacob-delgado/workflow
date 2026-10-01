// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CODEOWNERS spells a top-level GitLab group @acme, as it spells a user, so
// the forge says which a bare name is: a group is tagged as a team, through a
// Slack user group, and a person as a person.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// bareGroup is the bare name CODEOWNERS gives the changes' owner.
const bareGroup = "acme"

// acmeOwned is a pull request only the bare name acme owns, on GitLab, which
// knows acme as a group when group is set.
func acmeOwned(group bool) *world {
	acme := newWorld()
	acme.forgeKind = forge.KindGitLab
	acme.codeOwners = []string{bareGroup}
	acme.slack = newSlackWorld()
	acme.forgeGroups = []string{}

	if group {
		acme.forgeGroups = []string{bareGroup}
	}

	return acme
}

func TestABareNameGitLabKnowsAsAGroupIsLinkedToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	acme := acmeOwned(true)
	preview := typing(t, acme.live(t, 140, 40), "5", "p")

	// Act: open the picker on acme
	picking := typing(t, preview, "a")

	// Assert: it offers the user groups, not the channel's members
	requireScreen(t, picking.View().Content, "Link acme to Slack", "@"+podName, "@"+apiName)
	refuseScreen(t, picking.View().Content, carlaName)

	// Act: choose the pod's group, then post
	linked := typing(t, typing(t, picking, letters("pod")...), keyEnter)
	typing(t, linked, keyEnter)

	// Assert: the link is saved, and the post tags the group
	if calls := acme.asked("link-owner "); len(calls) != 1 || calls[0] != "link-owner acme "+podID {
		t.Errorf("links = %q, want acme linked to the pod's group", calls)
	}

	if got := postedText(t, acme); !strings.HasSuffix(got, "\ncc "+tagPod) {
		t.Errorf("posted %q, want it to tag the pod's group", got)
	}
}

func TestABareNameGitLabKnowsAsAPersonIsLinkedToAUser(t *testing.T) {
	t.Parallel()

	// Arrange
	preview := typing(t, acmeOwned(false).live(t, 140, 40), "5", "p")

	// Act
	picking := typing(t, preview, "a").View().Content

	// Assert
	requireScreen(t, picking, "Link acme to Slack", carlaName, benName)
	refuseScreen(t, picking, "@"+podName)
}

func TestPeopleAsksAgainAboutAGroupDecidedAsAPerson(t *testing.T) {
	t.Parallel()

	// Arrange
	// Before the forge told bare names apart, acme was decided as a person
	// not on Slack; GitLab now knows it as a group.
	acme := acmeOwned(true)
	acme.slack.links = []loop.OwnerLink{{Owner: bareGroup, Team: false, OnSlack: false, Slack: loop.SlackTarget{}}}

	// Act
	picking := typing(t, openPeople(t, acme), keyEnter).View().Content

	// Assert
	requireScreen(t, picking, "Link acme to Slack", "@"+podName)
	refuseScreen(t, picking, carlaName)
}
