// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The owners and the channel the tagging tests use.
const (
	ownerAna         = "ana"
	ownerDan         = "dan"
	controlPlaneTeam = "acme/control-plane"
	bareGroup        = "acme"
	devChannel       = "#dev"
)

// slackAna is whom Ana is on Slack.
func slackAna() loop.SlackTarget { return loop.SlackTarget{ID: "U0ANA", Label: "Ana"} }

// controlPlanePod is the user group the control-plane team is on Slack.
func controlPlanePod() loop.SlackTarget {
	return loop.SlackTarget{ID: "S0CP", Label: "control-plane-pod"}
}

// apiReviewers is a user group the repository may tag, linked to no team.
func apiReviewers() loop.SlackTarget { return loop.SlackTarget{ID: "S0API", Label: "api-reviewers"} }

// codeOwnersOfThePR() are the owners of the changed paths: three people, then
// two teams.
func codeOwnersOfThePR() codeowners.Owners {
	return codeowners.Owners{Users: []string{ownerAna, "ben", ownerDan}, Teams: []string{controlPlaneTeam, "acme/api"}}
}

// decided is what was decided for the owners: Ana and the control-plane team
// are on Slack, and Dan is not. Ben has never been asked.
func decided() []loop.OwnerLink {
	return []loop.OwnerLink{
		{Owner: ownerAna, OnSlack: true, Slack: slackAna()},
		{Owner: ownerDan, OnSlack: false, Slack: loop.SlackTarget{}},
		{Owner: controlPlaneTeam, OnSlack: true, Slack: controlPlanePod()},
	}
}

func TestProposeTagsGivesEachOwnerWhatWasDecidedForThem(t *testing.T) {
	t.Parallel()

	// Act
	tags := loop.ProposeTags(codeOwnersOfThePR(), decided(), nil, nil, false, messaging.MomentReady)

	// Assert
	want := []loop.OwnerTag{
		{Owner: ownerAna, Team: false, State: loop.OwnerLinked, Slack: slackAna()},
		{Owner: "ben", Team: false, State: loop.OwnerUnlinked, Slack: loop.SlackTarget{}},
		{Owner: ownerDan, Team: false, State: loop.OwnerNotOnSlack, Slack: loop.SlackTarget{}},
		{Owner: controlPlaneTeam, Team: true, State: loop.OwnerLinked, Slack: controlPlanePod()},
		{Owner: "acme/api", Team: true, State: loop.OwnerUnlinked, Slack: loop.SlackTarget{}},
	}
	if !reflect.DeepEqual(tags.Owners, want) {
		t.Errorf("Owners = %+v, want %+v", tags.Owners, want)
	}
}

func TestALinkOfTheWrongKindReadsAsUnlinked(t *testing.T) {
	t.Parallel()

	// Arrange
	owners := codeowners.Owners{Users: []string{ownerAna}, Teams: []string{controlPlaneTeam}}
	links := []loop.OwnerLink{
		{Owner: ownerAna, OnSlack: true, Slack: controlPlanePod()},
		{Owner: controlPlaneTeam, OnSlack: true, Slack: slackAna()},
	}

	// Act
	tags := loop.ProposeTags(owners, links, nil, nil, false, messaging.MomentReady)

	// Assert
	for _, owner := range tags.Owners {
		if owner.State != loop.OwnerUnlinked {
			t.Errorf("%s is %v, want unlinked: a user cannot be a group, nor a team a user", owner.Owner, owner.State)
		}
	}
}

func TestAnOwnerIsMatchedToTheirLinkWhateverTheCase(t *testing.T) {
	t.Parallel()

	// Arrange
	owners := codeowners.Owners{Users: []string{"Ana"}, Teams: []string{"Acme/Control-Plane"}}

	// Act
	tags := loop.ProposeTags(owners, decided(), nil, nil, false, messaging.MomentReady)

	// Assert
	for _, owner := range tags.Owners {
		if owner.State != loop.OwnerLinked {
			t.Errorf("%s is %v, want linked: forge names are not case-sensitive", owner.Owner, owner.State)
		}
	}
}

func TestProposeTagsPreChecksTheLastChoiceAndTheOwningTeamsGroups(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repoGroups []loop.SlackTarget
		last       []string
		lastChosen bool
		want       []loop.GroupTag
	}{
		"with no last choice, only the owning team's group": {
			repoGroups: []loop.SlackTarget{apiReviewers(), controlPlanePod()},
			want: []loop.GroupTag{
				{Slack: apiReviewers(), Checked: false, FromOwners: false},
				{Slack: controlPlanePod(), Checked: true, FromOwners: true},
			},
		},
		"the last choice, with the owning team's group": {
			repoGroups: []loop.SlackTarget{apiReviewers(), controlPlanePod()},
			last:       []string{apiReviewers().ID},
			lastChosen: true,
			want: []loop.GroupTag{
				{Slack: apiReviewers(), Checked: true, FromOwners: false},
				{Slack: controlPlanePod(), Checked: true, FromOwners: true},
			},
		},
		"a last choice of none still checks the owning team's group": {
			repoGroups: []loop.SlackTarget{apiReviewers()},
			lastChosen: true,
			want: []loop.GroupTag{
				{Slack: apiReviewers(), Checked: false, FromOwners: false},
				{Slack: controlPlanePod(), Checked: true, FromOwners: true},
			},
		},
		"a group last chosen but no longer the repository's is not offered": {
			repoGroups: nil,
			last:       []string{apiReviewers().ID},
			lastChosen: true,
			want:       []loop.GroupTag{{Slack: controlPlanePod(), Checked: true, FromOwners: true}},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			tags := loop.ProposeTags(
				codeOwnersOfThePR(), decided(), tt.repoGroups, tt.last, tt.lastChosen, messaging.MomentReady)

			// Assert
			if !reflect.DeepEqual(tags.Groups, tt.want) {
				t.Errorf("Groups = %+v, want %+v", tags.Groups, tt.want)
			}
		})
	}
}

func TestOnlyTheReadyForReviewAnnouncementTagsAnyone(t *testing.T) {
	t.Parallel()

	moments := map[string]messaging.Moment{"merged": messaging.MomentMerged, "CI red": messaging.MomentCIRed}

	for name, moment := range moments {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			tags := loop.ProposeTags(codeOwnersOfThePR(), decided(), []loop.SlackTarget{apiReviewers()},
				[]string{apiReviewers().ID}, true, moment)

			// Assert
			if len(tags.Owners) != 0 || len(tags.Groups) != 0 {
				t.Errorf("ProposeTags at %v = %+v, want no one", moment, tags)
			}
		})
	}
}

func TestMentionsTagLinkedPeopleAndTheCheckedGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	tags := loop.ProposeTags(codeOwnersOfThePR(), decided(), []loop.SlackTarget{apiReviewers()}, nil, false,
		messaging.MomentReady)

	// Act
	mentions, err := tags.Mentions([]string{controlPlanePod().ID})

	// Assert
	if line := mentions.Line(); err != nil || line != "cc <@U0ANA> <!subteam^S0CP>" {
		t.Errorf("Mentions = %q, %v; want Ana and the control-plane group, and not Dan", line, err)
	}
}

func TestMentionsTagOnlyTheOwnersTheCallerPassed(t *testing.T) {
	t.Parallel()

	// Arrange
	// The author is linked, but the caller has already left them out of the owners.
	author := loop.OwnerLink{Owner: "me", OnSlack: true, Slack: loop.SlackTarget{ID: "U0ME", Label: "Me"}}
	links := append(decided(), author)
	tags := loop.ProposeTags(codeowners.Owners{Users: []string{ownerAna}, Teams: nil}, links, nil, nil, false,
		messaging.MomentReady)

	// Act
	mentions, err := tags.Mentions(nil)

	// Assert
	if line := mentions.Line(); err != nil || line != "cc <@U0ANA>" {
		t.Errorf("Mentions = %q, %v; want Ana alone", line, err)
	}
}

func TestMentionsRefuseAGroupNotOffered(t *testing.T) {
	t.Parallel()

	// Arrange
	tags := loop.ProposeTags(codeOwnersOfThePR(), decided(), nil, nil, false, messaging.MomentReady)

	// Act
	_, err := tags.Mentions([]string{apiReviewers().ID})

	// Assert
	if !errors.Is(err, loop.ErrGroupNotOffered) {
		t.Errorf("Mentions = %v, want %v", err, loop.ErrGroupNotOffered)
	}
}

// tagged is a ready-for-review delivery tagging Ana and the control-plane group.
func tagged(t *testing.T) loop.Delivery {
	t.Helper()

	mentions, err := messaging.NewMentions([]string{slackAna().ID}, []string{controlPlanePod().ID})
	if err != nil {
		t.Fatalf("NewMentions: %v", err)
	}

	return loop.Delivery{
		Channel: devChannel, Text: "ready", Made: loop.Announced{Pull: 9, Moment: messaging.MomentReady}, Mentions: mentions,
	}
}

func TestDeliverPostsTheMentionsAndRecordsTheGroupsChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		sent   deliveries
		chosen []string
	)

	memory := sent.memory()
	memory.RecordGroups = func(ids []string) error {
		chosen = ids

		return nil
	}

	// Act
	err := loop.Deliver(sent.post(nil), memory, tagged(t))

	// Assert
	if err != nil || !slices.Equal(sent.posted, []string{devChannel + " ready\ncc <@U0ANA> <!subteam^S0CP>"}) {
		t.Errorf("Deliver = %v, posted %q; want the text, then the tags on a line of their own", err, sent.posted)
	}

	if !slices.Equal(chosen, []string{controlPlanePod().ID}) {
		t.Errorf("recorded groups %v, want the control-plane group", chosen)
	}
}

func TestAFailedPostRecordsNoGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		sent     deliveries
		recorded bool
	)

	memory := sent.memory()
	memory.RecordGroups = func([]string) error {
		recorded = true

		return nil
	}

	// Act
	err := loop.Deliver(sent.post(errSeam), memory, tagged(t))

	// Assert
	if !errors.Is(err, errSeam) || recorded {
		t.Errorf("Deliver = %v, recorded groups %t; want the post's error and nothing recorded", err, recorded)
	}
}

func TestABareNameDecidedAsAPersonIsAskedAgainOnceTheForgeKnowsItAsAGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	// Before the forge told them apart, every bare name was decided as a
	// person, here as one not on Slack; GitLab now knows acme as a group.
	owners := codeowners.Owners{Users: nil, Teams: []string{bareGroup}}
	links := []loop.OwnerLink{{Owner: bareGroup, Team: false, OnSlack: false, Slack: loop.SlackTarget{}}}

	// Act
	tags := loop.ProposeTags(owners, links, nil, nil, false, messaging.MomentReady)

	// Assert
	want := []loop.OwnerTag{{Owner: bareGroup, Team: true, State: loop.OwnerUnlinked, Slack: loop.SlackTarget{}}}
	if !reflect.DeepEqual(tags.Owners, want) {
		t.Errorf("Owners = %+v, want acme a team to be asked about again", tags.Owners)
	}
}

func TestABareNameDecidedAsATeamStaysOneWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab could not be asked, so acme reads as a person, as it is spelled.
	owners := codeowners.Owners{Users: []string{bareGroup}, Teams: nil}
	links := []loop.OwnerLink{{Owner: bareGroup, Team: true, OnSlack: true, Slack: controlPlanePod()}}

	// Act
	tags := loop.ProposeTags(owners, links, nil, nil, false, messaging.MomentReady)

	// Assert
	want := []loop.OwnerTag{{Owner: bareGroup, Team: true, State: loop.OwnerLinked, Slack: controlPlanePod()}}
	if !reflect.DeepEqual(tags.Owners, want) || len(tags.Groups) != 1 || !tags.Groups[0].Checked {
		t.Errorf("Tags = %+v, want acme the team linked to its group, offered checked", tags)
	}
}
