// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
)

// What People and groups refuses, in words that say what to do.
var (
	errNoSlackDirectory = errors.New("tagging needs a Slack user token; set one up in Settings, " +
		"or run workflow slack login")
	errNoPeopleStore  = errors.New("people and groups are not kept here: the store is off")
	errOneOrTheOther  = errors.New("say whom the owner is on Slack, or that they are not on it — one or the other")
	errWrongKind      = errors.New("a team links to a Slack user group, and a person to a Slack user")
	errNotInDirectory = errors.New("that Slack ID is not among the channel's members or the workspace's user groups")
)

// GetPeople lists every owner decided on this forge host, then the branch's
// owners not decided yet.
func (s *server) GetPeople(_ context.Context, _ api.GetPeopleRequestObject) (api.GetPeopleResponseObject, error) {
	people, err := s.people()
	if err != nil {
		return problemAnswer[api.GetPeopledefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
	}

	return api.GetPeople200JSONResponse(people), nil
}

// LinkPerson records whom an owner is on Slack, with the label Slack's
// directory gives the ID, or that they are not on Slack.
func (s *server) LinkPerson(
	_ context.Context, request api.LinkPersonRequestObject,
) (api.LinkPersonResponseObject, error) {
	err := s.linkPerson(*request.Body)
	if err == nil {
		var people api.People

		people, err = s.people()
		if err == nil {
			return api.LinkPerson200JSONResponse(people), nil
		}
	}

	return problemAnswer[api.LinkPersondefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
}

// ForgetPerson drops what was decided for an owner, so they are asked again.
func (s *server) ForgetPerson(
	_ context.Context, request api.ForgetPersonRequestObject,
) (api.ForgetPersonResponseObject, error) {
	err := errNoPeopleStore
	if s.deps.ForgetOwner != nil {
		err = s.inWorkspace(func(workspace string) error {
			return s.keptWrite(func() error { return s.deps.ForgetOwner(workspace, request.Params.Owner) })
		})
	}

	if err == nil {
		var people api.People

		people, err = s.people()
		if err == nil {
			return api.ForgetPerson200JSONResponse(people), nil
		}
	}

	return problemAnswer[api.ForgetPersondefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
}

// people is every decided owner, then the branch's undecided ones, users
// before teams, each as an announcement would tag them.
func (s *server) people() (api.People, error) {
	if s.deps.OwnerLinks == nil {
		return api.People{}, errNoPeopleStore
	}

	var links []loop.OwnerLink

	err := s.inWorkspace(func(workspace string) error {
		var err error

		links, err = s.deps.OwnerLinks(workspace)

		return err
	})
	if err != nil {
		return api.People{}, err
	}

	tags := loop.ProposeTags(peopleOwners(links, s.branchOwners()), links, nil, nil, false, messaging.MomentReady)

	return api.People{Owners: ownerTagsDTO(tags.Owners)}, nil
}

// peopleOwners is every owner decided, as the kind each was decided as or a
// team when the forge now knows it as a group, then each of the branch's
// owners not decided, as the forge tells them apart.
func peopleOwners(links []loop.OwnerLink, branchOwners codeowners.Owners) codeowners.Owners {
	var owners codeowners.Owners

	add := func(owner string, team bool) {
		if team {
			owners.Teams = append(owners.Teams, owner)
		} else {
			owners.Users = append(owners.Users, owner)
		}
	}

	branchTeam := func(owner string) bool {
		return slices.ContainsFunc(branchOwners.Teams, func(team string) bool { return loop.SameOwner(team, owner) })
	}

	for _, link := range links {
		add(link.Owner, link.Team || branchTeam(link.Owner))
	}

	for _, owner := range slices.Concat(branchOwners.Users, branchOwners.Teams) {
		if !slices.ContainsFunc(links, func(link loop.OwnerLink) bool { return loop.SameOwner(link.Owner, owner) }) {
			add(owner, branchTeam(owner))
		}
	}

	return owners
}

// branchOwners is who owns the branch's changes, a bare name the forge knows
// as a group among the teams, or nobody when they cannot be read: People
// lists them only to be decided ahead of an announcement.
func (s *server) branchOwners() codeowners.Owners {
	if s.deps.Branch == nil {
		return codeowners.Owners{}
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return codeowners.Owners{}
	}

	seams := s.ownerSeams()
	seams.IsGroup = s.deps.IsGroup

	owners, err := loop.OwnersOf(seams, branch.Base)
	if err != nil {
		return codeowners.Owners{}
	}

	return owners
}

// linkPerson checks link and records it.
func (s *server) linkPerson(link api.PersonLink) error {
	if s.deps.LinkOwner == nil {
		return errNoPeopleStore
	}

	if (link.SlackID == nil) != link.NotOnSlack {
		return errOneOrTheOther
	}

	team, err := s.isTeam(link.Owner)
	if err != nil {
		return err
	}

	return s.inWorkspace(func(workspace string) error {
		decision := loop.OwnerLink{Owner: link.Owner, Team: team, OnSlack: false, Slack: loop.SlackTarget{}}
		if !link.NotOnSlack {
			target, err := s.slackTarget(*link.SlackID, team, orZero(link.Channel))
			if err != nil {
				return err
			}

			decision.OnSlack, decision.Slack = true, target
		}

		return s.keptWrite(func() error { return s.deps.LinkOwner(workspace, decision) })
	})
}

// isTeam reports whether owner is a team, as People lists them: as it was
// decided, or as the forge tells the branch's owners apart, and otherwise as
// CODEOWNERS spells a team, with a slash.
func (s *server) isTeam(owner string) (bool, error) {
	people, err := s.people()
	if err != nil {
		return false, err
	}

	for _, listed := range people.Owners {
		if loop.SameOwner(listed.Owner, owner) {
			return listed.Kind == api.OwnerTagKindTeam, nil
		}
	}

	return strings.Contains(owner, "/"), nil
}

// inWorkspace makes use of the kept associations in the Slack workspace the
// token is for, which they are kept per, or says why it cannot be read: with
// no Slack user token, that there is no directory to link from.
func (s *server) inWorkspace(use func(workspace string) error) error {
	if !s.taggingLive() {
		return errNoSlackDirectory
	}

	workspace, err := loop.TagWorkspace(s.deps.Workspace)
	if errors.Is(err, messaging.ErrNoCredential) {
		return errNoSlackDirectory
	}

	if err != nil {
		return err
	}

	return use(workspace)
}

// keptWrite runs write, a write to the kept associations or a removal of the
// local data, after any other under way.
func (s *server) keptWrite(write func() error) error {
	s.keptWrites.Lock()
	defer s.keptWrites.Unlock()

	return write()
}

// peopleFault is the problem a People and groups request is refused with:
// what the request or the server's setup cannot carry out is unprocessable,
// and anything else is classified by fault.
func (s *server) peopleFault(err error) api.Problem {
	if errors.Is(err, loop.ErrUnknownWorkspace) {
		return problem(api.ProblemCodeUnprocessable, workspaceRefusal(err))
	}

	if scope, missing := missingScope(err); missing {
		return problem(api.ProblemCodeUnprocessable, "the Slack token lacks the "+scope+
			" scope; add it to the Slack app, then sign in again with workflow slack login")
	}

	return s.fault(err)
}

// peopleFaults are the failures People and groups answers in their own
// words: its own refusals, the kept store's and Slack's directory's, none of
// which names a host or a path.
func peopleFaults() []faultClass {
	refusals := []error{
		errNoSlackDirectory, errNoPeopleStore, errOneOrTheOther, errWrongKind, errNotInDirectory,
		store.ErrInvalidOwner, store.ErrInvalidSlackID,
		messaging.ErrChannelNotFound, messaging.ErrLookupRefused, messaging.ErrDirectoryTooLarge,
	}

	classes := make([]faultClass, 0, len(refusals))
	for _, refusal := range refusals {
		classes = append(classes,
			faultClass{causes: []error{refusal}, code: api.ProblemCodeUnprocessable, detail: refusal.Error()})
	}

	return classes
}

// ownerTagsDTO maps code owners as an announcement tags them onto the wire.
func ownerTagsDTO(owners []loop.OwnerTag) []api.OwnerTag {
	states := map[loop.OwnerState]api.OwnerTagState{
		loop.OwnerUnlinked:   api.OwnerTagStateUnlinked,
		loop.OwnerLinked:     api.OwnerTagStateLinked,
		loop.OwnerNotOnSlack: api.OwnerTagStateNotOnSlack,
	}
	kinds := map[bool]api.OwnerTagKind{false: api.OwnerTagKindUser, true: api.OwnerTagKindTeam}

	tags := make([]api.OwnerTag, 0, len(owners))

	for _, owner := range owners {
		tag := api.OwnerTag{Owner: owner.Owner, Kind: kinds[owner.Team], State: states[owner.State], Slack: nil}
		if owner.State == loop.OwnerLinked {
			slack := api.SlackTarget(owner.Slack)
			tag.Slack = &slack
		}

		tags = append(tags, tag)
	}

	return tags
}
