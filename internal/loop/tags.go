// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// SlackTarget is a Slack user or user group, by its ID and the label it is
// shown by.
type SlackTarget struct {
	ID    string
	Label string
}

// OwnerLink is what was decided for one forge owner: whether they are on Slack,
// and if so, as whom — a user for a user owner, a user group for a team.
type OwnerLink struct {
	Owner   string
	OnSlack bool
	Slack   SlackTarget
}

// ErrGroupNotOffered refuses to tag a user group the announcement did not
// offer.
var ErrGroupNotOffered = errors.New("that user group is not offered for this announcement")

// OwnerState is what is known of a code owner on Slack.
type OwnerState int

const (
	// OwnerUnlinked is an owner never asked about, or whose link is not one an
	// owner of their kind can have.
	OwnerUnlinked OwnerState = iota
	// OwnerLinked is an owner known on Slack: a user as a user, a team as a
	// user group.
	OwnerLinked
	// OwnerNotOnSlack is an owner decided not to be on Slack, who is never
	// tagged and not asked again.
	OwnerNotOnSlack
)

// OwnerTag is one code owner as an announcement may tag them: a user, or a
// team (org/team, GitLab's group/subgroup), what is known of them on Slack,
// and whom they are there when linked.
type OwnerTag struct {
	Owner string
	Team  bool
	State OwnerState
	Slack SlackTarget
}

// GroupTag is a user group an announcement offers: whether it starts checked,
// and whether it is offered because a team owning the changed paths is
// linked to it.
type GroupTag struct {
	Slack      SlackTarget
	Checked    bool
	FromOwners bool
}

// Tags is who an announcement proposes to tag: the code owners, users then
// teams, and the user groups it offers.
type Tags struct {
	Owners []OwnerTag
	Groups []GroupTag
}

// ProposeTags is who the announcement at moment proposes to tag. Only the
// ready-for-review announcement tags anyone; any other moment proposes no
// one. owners are the changed paths' owners, already without the author;
// links are what was decided for owners on this forge host; repoGroups are
// the user groups this repository may tag; and last is the groups chosen
// the last time, when lastChosen says there was a last time.
//
// The groups offered are the repository's, then any a linked owning team
// adds. The groups linked to owning teams start checked, and so does the last
// choice; with no last choice, only the teams' groups do.
func ProposeTags(
	owners codeowners.Owners, links []OwnerLink, repoGroups []SlackTarget, last []string, lastChosen bool,
	moment messaging.Moment,
) Tags {
	if moment != messaging.MomentReady {
		return Tags{}
	}

	ownerTags := make([]OwnerTag, 0, len(owners.Users)+len(owners.Teams))
	for _, user := range owners.Users {
		ownerTags = append(ownerTags, ownerTag(user, false, links))
	}

	for _, team := range owners.Teams {
		ownerTags = append(ownerTags, ownerTag(team, true, links))
	}

	if !lastChosen {
		last = nil
	}

	return Tags{Owners: ownerTags, Groups: offeredGroups(ownerTags, repoGroups, last)}
}

// Mentions is the tags of the linked user owners and of checkedGroupIDs.
// A team owner is tagged through its group, when that group is checked. A
// group the tags did not offer is refused with ErrGroupNotOffered.
func (t Tags) Mentions(checkedGroupIDs []string) (messaging.Mentions, error) {
	for _, id := range checkedGroupIDs {
		if !slices.ContainsFunc(t.Groups, func(group GroupTag) bool { return group.Slack.ID == id }) {
			return messaging.Mentions{}, ErrGroupNotOffered
		}
	}

	users := make([]string, 0, len(t.Owners))

	for _, owner := range t.Owners {
		if !owner.Team && owner.State == OwnerLinked {
			users = append(users, owner.Slack.ID)
		}
	}

	return messaging.NewMentions(users, checkedGroupIDs)
}

// ownerTag is owner as links decided them. A link to the wrong kind of Slack
// target — a user owner to a group, a team to a user — reads as unlinked, so
// it is asked again rather than tagged.
func ownerTag(owner string, team bool, links []OwnerLink) OwnerTag {
	tag := OwnerTag{Owner: owner, Team: team, State: OwnerUnlinked, Slack: SlackTarget{}}

	index := slices.IndexFunc(links, func(link OwnerLink) bool { return link.Owner == owner })
	if index < 0 {
		return tag
	}

	link := links[index]

	switch {
	case !link.OnSlack:
		tag.State = OwnerNotOnSlack
	case fitsOwner(link.Slack.ID, team):
		tag.State, tag.Slack = OwnerLinked, link.Slack
	}

	return tag
}

// fitsOwner reports whether slackID is the kind an owner links to: a user
// group for a team, a user otherwise.
func fitsOwner(slackID string, team bool) bool {
	if team {
		_, err := messaging.ParseSlackGroup(slackID)

		return err == nil
	}

	_, err := messaging.ParseSlackUser(slackID)

	return err == nil
}

// offeredGroups is repoGroups then the groups linked teams among owners add,
// each once, checked when a team links it or last chose it.
func offeredGroups(owners []OwnerTag, repoGroups []SlackTarget, last []string) []GroupTag {
	var teamGroups []SlackTarget

	for _, owner := range owners {
		if owner.Team && owner.State == OwnerLinked {
			teamGroups = append(teamGroups, owner.Slack)
		}
	}

	groups := make([]GroupTag, 0, len(repoGroups)+len(teamGroups))

	for _, group := range slices.Concat(repoGroups, teamGroups) {
		sameGroup := func(offered GroupTag) bool { return offered.Slack.ID == group.ID }
		if slices.ContainsFunc(groups, sameGroup) {
			continue
		}

		fromOwners := slices.ContainsFunc(teamGroups, func(team SlackTarget) bool { return team.ID == group.ID })
		checked := fromOwners || slices.Contains(last, group.ID)
		groups = append(groups, GroupTag{Slack: group, Checked: checked, FromOwners: fromOwners})
	}

	return groups
}
