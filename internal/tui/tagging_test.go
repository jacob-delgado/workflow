// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The tags a post carries, as Slack's markup names them.
const (
	tagCarla = "<@" + carlaID + ">"
	tagBen   = "<@" + benID + ">"
	tagPod   = "<!subteam^" + podID + ">"
	tagAPI   = "<!subteam^" + apiID + ">"
)

// taggingWorld is a pull request whose changes ben, carla, dan and the
// control-plane team own: carla is linked, dan is not on Slack, ben was never
// asked, and the team is linked to its user group. The repository may also
// tag the API reviewers.
func taggingWorld() *world {
	tagging := newWorld()
	tagging.codeOwners = []string{ownerBen, ownerCarla, "dan", podTeam}
	tagging.slack = newSlackWorld()
	tagging.slack.links = []loop.OwnerLink{
		{Owner: ownerCarla, OnSlack: true, Slack: loop.SlackTarget{ID: carlaID, Label: carlaName}},
		{Owner: "dan", OnSlack: false, Slack: loop.SlackTarget{}},
		{Owner: podTeam, OnSlack: true, Slack: loop.SlackTarget{ID: podID, Label: podName}},
	}
	tagging.slack.repoGroups = []loop.SlackTarget{{ID: apiID, Label: apiName}}

	return tagging
}

// onlyBen is a pull request only ben owns, and ben was never asked about.
func onlyBen() *world {
	ben := newWorld()
	ben.codeOwners = []string{ownerBen}
	ben.slack = newSlackWorld()

	return ben
}

// postedText is the one post a world made, failing the test when it made
// another number.
func postedText(t *testing.T, w *world) string {
	t.Helper()

	calls := w.asked("post ")
	if len(calls) != 1 {
		t.Fatalf("post calls = %q, want one", calls)
	}

	return calls[0]
}

func TestThePreviewShowsWhomTheAnnouncementTags(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, taggingWorld().live(t, 140, 40), "5", "p").View().Content

	// Assert
	requireScreen(t, view,
		"Code owners", "ben", "? not linked", "carla", "→ Carla Diaz", "dan", "· not on Slack",
		"Groups", "@control-plane-pod", "owns changed paths", "@api-reviewers",
		"tags  @Carla Diaz @control-plane-pod", "a link to Slack", "x not on Slack")
}

func TestPostingTagsTheLinkedOwnersAndTheCheckedGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	typing(t, tagging.live(t, 140, 40), "5", "p", keyEnter)

	// Assert
	if got := postedText(t, tagging); !strings.HasSuffix(got, "\ncc "+tagCarla+" "+tagPod) {
		t.Errorf("posted %q, want it to end tagging Carla and the pod", got)
	}

	if calls := tagging.asked("record-groups "); len(calls) != 1 || calls[0] != "record-groups "+podID {
		t.Errorf("recorded groups %q, want the pod as the choice", calls)
	}
}

func TestSpaceTagsAnotherGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act: move past the four owners to the API reviewers, and tag them
	toggled := typing(t, preview, "down", "down", "down", "down", keySpace)

	// Assert: the summary names them
	requireScreen(t, toggled.View().Content, "tags  @Carla Diaz @api-reviewers @control-plane-pod")

	// Act: post
	typing(t, toggled, keyEnter)

	// Assert: the post tags them too, and they are remembered as chosen
	if got := postedText(t, tagging); !strings.HasSuffix(got, tagPod+" "+tagAPI) {
		t.Errorf("posted %q, want it to tag the API reviewers too", got)
	}

	if calls := tagging.asked("record-groups "); len(calls) != 1 || calls[0] != "record-groups "+podID+","+apiID {
		t.Errorf("recorded groups %q, want the pod and the API reviewers", calls)
	}
}

func TestSpaceUntagsAGroupAndDoesNothingOnAnOwner(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	// space on ben does nothing, nor x on a group; the API reviewers are
	// tagged, then they and the pod untagged
	untagged := typing(t, preview, keySpace, "down", "down", "down", "down", keySpace, "x", keySpace, "down", keySpace)

	// Assert
	requireScreen(t, untagged.View().Content, "tags  @Carla Diaz")
	refuseScreen(t, untagged.View().Content, "tags  @Carla Diaz @")

	if calls := tagging.asked("link-owner "); len(calls) != 0 {
		t.Errorf("links = %q, want none from x on a group", calls)
	}
}

func TestLinkingAnOwnerSavesItAndTagsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	preview := typing(t, ben.live(t, 140, 40), "5", "p")

	// Act: open the picker on ben
	picking := typing(t, preview, "a")

	// Assert: it offers the channel's members and "not on Slack"
	requireScreen(t, picking.View().Content, "Link ben to Slack", carlaName, benName, "Not on Slack")

	// Act: narrow it to Ben by typing, and choose him
	linked := typing(t, typing(t, picking, letters("ortiz")...), keyEnter)

	// Assert: the link is saved at once, and the preview tags him
	if calls := ben.asked("link-owner "); len(calls) != 1 || calls[0] != benLinked {
		t.Fatalf("links = %q, want ben linked to Ben Ortiz", calls)
	}

	requireScreen(t, linked.View().Content, "Announce to Slack", "→ Ben Ortiz", "tags  @Ben Ortiz")

	// Act: post
	typing(t, linked, keyEnter)

	// Assert: it tags Ben
	if got := postedText(t, ben); !strings.HasSuffix(got, "\ncc "+tagBen) {
		t.Errorf("posted %q, want it to tag Ben", got)
	}
}

func TestThePickerFiltersAndGoesBackWithoutLinking(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	picking := typing(t, ben.live(t, 140, 40), "5", "p", "a")

	// Act: type a filter matching no one, take a letter back, and leave
	filtered := typing(t, picking, "q", "q", keyBackspace, "down", "up")

	// Assert: only "not on Slack" is left to choose
	requireScreen(t, filtered.View().Content, "filter  q ", "Not on Slack")
	refuseScreen(t, filtered.View().Content, carlaName)

	// Act: go back
	back := typing(t, filtered, keyEsc)

	// Assert: the preview is open again, and nothing was saved
	requireScreen(t, back.View().Content, "Announce to Slack", "? not linked")

	if calls := ben.asked("link-owner "); len(calls) != 0 {
		t.Errorf("links = %q, want none", calls)
	}
}

func TestChoosingNotOnSlackInThePickerRemembersIt(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	picking := typing(t, ben.live(t, 140, 40), "5", "p", "a")

	// Act
	// the last row is "not on Slack"
	chosen := typing(t, picking, "down", "down", keyEnter)

	// Assert
	if calls := ben.asked("link-owner "); len(calls) != 1 || calls[0] != benNotOnSlack {
		t.Errorf("links = %q, want ben not on Slack", calls)
	}

	requireScreen(t, chosen.View().Content, "· not on Slack", "tags  nobody")
}

func TestXMarksAnOwnerNotOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()

	// Act
	view := typing(t, ben.live(t, 140, 40), "5", "p", "x").View().Content

	// Assert
	if calls := ben.asked("link-owner "); len(calls) != 1 || calls[0] != benNotOnSlack {
		t.Errorf("links = %q, want ben not on Slack", calls)
	}

	requireScreen(t, view, "· not on Slack")
}

func TestATeamIsLinkedToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	team := newWorld()
	team.codeOwners = []string{podTeam}
	team.slack = newSlackWorld()

	// Act: open the picker on the team
	picking := typing(t, team.live(t, 140, 40), "5", "p", "a")

	// Assert: the team picks from the user groups, not the channel
	requireScreen(t, picking.View().Content, "@"+podName, "@"+apiName)
	refuseScreen(t, picking.View().Content, carlaName)

	// Act: choose the pod
	linked := typing(t, picking, keyEnter)

	// Assert: the pod is offered, checked, because the team owns the changes
	requireScreen(t, linked.View().Content, "→ @control-plane-pod", "owns changed paths", "tags  @control-plane-pod")
}

func TestARefusedLinkSaysWhyAndStillPosts(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	ben.slack.linkErr = errDirectoryDown

	// Act: mark ben not on Slack, which is refused
	refused := typing(t, ben.live(t, 140, 40), "5", "p", "x")

	// Assert: the refusal is shown, and ben stays unlinked
	requireScreen(t, refused.View().Content, "slack is down", "? not linked")

	// Act: post anyway
	typing(t, refused, keyEnter)

	// Assert: the post goes, untagged
	if got := postedText(t, ben); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want it untagged", got)
	}
}

func TestAMissingScopeIsNamedAndThePostGoesUntagged(t *testing.T) {
	t.Parallel()

	// Arrange
	scopeless := taggingWorld()
	scopeless.slack.membersErr = &messaging.MissingScopeError{Needed: "users:read"}

	// Act: open the preview
	preview := typing(t, scopeless.live(t, 140, 40), "5", "p")

	// Assert: the scope is named, and no link is offered
	requireScreen(t, preview.View().Content, "tagging needs the users:read scope")
	refuseScreen(t, footerLine(preview.View().Content), "link to Slack")

	// Act: post
	typing(t, preview, keyEnter)

	// Assert: the post goes, untagged, and no choice is remembered
	if got := postedText(t, scopeless); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want it untagged", got)
	}

	if calls := scopeless.asked("record-groups "); len(calls) != 0 {
		t.Errorf("recorded groups %q for an untagged post", calls)
	}
}

func TestAFailedDirectoryReadStillTagsWhoIsKnown(t *testing.T) {
	t.Parallel()

	// Arrange
	down := taggingWorld()
	down.slack.membersErr = errDirectoryDown
	down.slack.groupsErr = errDirectoryDown
	down.slack.linksErr = errDirectoryDown

	// Act: open the preview
	preview := typing(t, down.live(t, 140, 40), "5", "p")

	// Assert: the failure is shown
	requireScreen(t, preview.View().Content, "slack is down")

	// Act: post
	typing(t, preview, keyEnter)

	// Assert: with no links read, only the repository's groups could be
	// offered, and none was checked
	if got := postedText(t, down); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want it untagged", got)
	}
}

func TestThePickerSaysWhenTheDirectoryIsUnreadable(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	ben.slack.membersErr = errDirectoryDown

	// Act
	picking := typing(t, ben.live(t, 140, 40), "5", "p", "a")

	// Assert
	requireScreen(t, picking.View().Content, "slack is down", "Not on Slack")
}

func TestChangingTheChannelReadsItsMembers(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Messaging.Channels = []string{teamChannel}
	ben := onlyBen()
	model := sized(t, tui.New(cfg, nil, ben.deps()), 140, 40)
	model = drain(t, model, model.Init())

	// Act
	picking := typing(t, model, "5", "p", keyRight, "a")

	// Assert
	// #team-b has nobody in it
	if calls := ben.asked("members "); len(calls) != 2 || calls[1] != "members #team-b" {
		t.Errorf("member reads = %q, want the new channel read", calls)
	}

	refuseScreen(t, picking.View().Content, carlaName)
}

func TestAPostWaitingForCICarriesItsTags(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	tagging.ciInterval = time.Millisecond
	tagging.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	// Act
	typing(t, tagging.live(t, 140, 40), "5", "p", "w")

	// Assert
	if got := postedText(t, tagging); !strings.HasSuffix(got, "\ncc "+tagCarla+" "+tagPod) {
		t.Errorf("posted %q, want the queued post to carry its tags", got)
	}

	if calls := tagging.asked("record-groups "); len(calls) != 1 {
		t.Errorf("recorded groups %q, want the choice recorded once", calls)
	}
}

func TestOnlyTheReadyForReviewAnnouncementTags(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := taggingWorld()
	merged.pull.State = forge.StateMerged

	// Act
	view := typing(t, merged.live(t, 140, 40), "5", "p").View().Content

	// Assert
	refuseScreen(t, view, "Code owners", "tags  ")

	if calls := merged.asked("members "); len(calls) != 0 {
		t.Errorf("read the channel's members %q for an announcement that tags no one", calls)
	}
}

func TestADryRunOffersNoLinkingAndSaysWhomItWouldTag(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := taggingWorld()
	model := sized(t, dryInterface(dry), 140, 40)
	model = drain(t, model, model.Init())

	// Act: open the preview
	preview := typing(t, model, "5", "p")

	// Assert: the owners are shown, but cannot be linked
	requireScreen(t, preview.View().Content, "? not linked")
	refuseScreen(t, footerLine(preview.View().Content), "link to Slack")

	// Act: try to link, and announce
	view := typing(t, preview, "a", "x", keyEnter).View().Content

	// Assert: a dry run keeps nothing, so it knows no one to tag
	requireScreen(t, view, "dry run: would announce to "+slackChannel+", tagging nobody")

	if calls := dry.asked("link-owner "); len(calls) != 0 {
		t.Errorf("a dry run linked %q", calls)
	}
}
