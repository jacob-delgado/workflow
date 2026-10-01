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
	"github.com/jacob-delgado/workflow/internal/config"
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

// GetSlackMembers lists the people in a channel — the configured one when
// none is named — that a code owner who is a user can be linked to.
func (s *server) GetSlackMembers(
	_ context.Context, request api.GetSlackMembersRequestObject,
) (api.GetSlackMembersResponseObject, error) {
	members, err := s.channelMembers(orZero(request.Params.Channel))

	directory, err := directoryAnswer(members, err)
	if err != nil {
		prob, code := s.peopleFault(err)

		return api.GetSlackMembersdefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
	}

	return api.GetSlackMembers200JSONResponse(directory), nil
}

// GetSlackGroups lists the workspace's user groups, to tag or to link a team
// to.
func (s *server) GetSlackGroups(
	_ context.Context, _ api.GetSlackGroupsRequestObject,
) (api.GetSlackGroupsResponseObject, error) {
	groups, err := s.userGroups()

	directory, err := directoryAnswer(groups, err)
	if err != nil {
		prob, code := s.peopleFault(err)

		return api.GetSlackGroupsdefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
	}

	return api.GetSlackGroups200JSONResponse(directory), nil
}

// GetPeople lists every owner decided on this forge host, then the branch's
// owners not decided yet.
func (s *server) GetPeople(_ context.Context, _ api.GetPeopleRequestObject) (api.GetPeopleResponseObject, error) {
	people, err := s.people()
	if err != nil {
		prob, code := s.peopleFault(err)

		return api.GetPeopledefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
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

	prob, code := s.peopleFault(err)

	return api.LinkPersondefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
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

	prob, code := s.peopleFault(err)

	return api.ForgetPersondefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
}

// GetRepoGroups lists the user groups this repository's announcements may
// tag.
func (s *server) GetRepoGroups(
	_ context.Context, _ api.GetRepoGroupsRequestObject,
) (api.GetRepoGroupsResponseObject, error) {
	groups, err := s.repoGroups()
	if err != nil {
		prob, code := s.peopleFault(err)

		return api.GetRepoGroupsdefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
	}

	return api.GetRepoGroups200JSONResponse(groups), nil
}

// SetRepoGroups replaces this repository's user groups with the ones named,
// each labeled as the workspace's directory has it.
func (s *server) SetRepoGroups(
	_ context.Context, request api.SetRepoGroupsRequestObject,
) (api.SetRepoGroupsResponseObject, error) {
	err := s.setRepoGroups(request.Body.Ids)
	if err == nil {
		var groups api.RepoGroups

		groups, err = s.repoGroups()
		if err == nil {
			return api.SetRepoGroups200JSONResponse(groups), nil
		}
	}

	prob, code := s.peopleFault(err)

	return api.SetRepoGroupsdefaultApplicationProblemPlusJSONResponse{Body: prob, StatusCode: code}, nil
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

	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.Owner)
	}

	branchOwners := s.branchOwners()
	for _, owner := range slices.Concat(branchOwners.Users, branchOwners.Teams) {
		if !slices.ContainsFunc(names, func(name string) bool { return loop.SameOwner(name, owner) }) {
			names = append(names, owner)
		}
	}

	users, teams := loop.SplitReviewers(names)
	tags := loop.ProposeTags(codeowners.Owners{Users: users, Teams: teams}, links, nil, nil, false,
		messaging.MomentReady)

	return api.People{Owners: ownerTagsDTO(tags.Owners)}, nil
}

// branchOwners is who owns the branch's changes, or nobody when they cannot
// be read: People lists them only to be decided ahead of an announcement.
func (s *server) branchOwners() codeowners.Owners {
	if s.deps.Branch == nil {
		return codeowners.Owners{}
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return codeowners.Owners{}
	}

	owners, err := loop.OwnersOf(s.ownerSeams(), branch.Base)
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

	return s.inWorkspace(func(workspace string) error {
		if link.NotOnSlack {
			return s.keptWrite(func() error { return s.deps.LinkOwner(workspace, link.Owner, nil) })
		}

		target, err := s.slackTarget(*link.SlackID, strings.Contains(link.Owner, "/"), orZero(link.Channel))
		if err != nil {
			return err
		}

		return s.keptWrite(func() error { return s.deps.LinkOwner(workspace, link.Owner, &target) })
	})
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

// keptWrite runs write, a write to the kept associations or a clean of the
// local data, after any other under way.
func (s *server) keptWrite(write func() error) error {
	s.keptWrites.Lock()
	defer s.keptWrites.Unlock()

	return write()
}

// slackTarget is slackID as Slack's directory labels it: a user group for a team,
// a member of channel otherwise.
func (s *server) slackTarget(slackID string, team bool, channel string) (loop.SlackTarget, error) {
	if team {
		return s.slackGroup(slackID)
	}

	_, err := messaging.ParseSlackUser(slackID)
	if err != nil {
		return loop.SlackTarget{}, errWrongKind
	}

	return lookUp(func() ([]loop.SlackTarget, error) { return s.channelMembers(channel) }, slackID)
}

// slackGroup is the user group the workspace lists under slackID.
func (s *server) slackGroup(slackID string) (loop.SlackTarget, error) {
	_, err := messaging.ParseSlackGroup(slackID)
	if err != nil {
		return loop.SlackTarget{}, errWrongKind
	}

	return lookUp(s.userGroups, slackID)
}

// taggingLive reports whether the configuration in effect posts with a Slack
// user token, which tagging and the directory it links from need: a save in
// Settings can switch to a webhook while the server runs.
func (s *server) taggingLive() bool {
	return s.config().Messaging.Mode() == config.MessagingUser
}

// channelMembers is the people in channel, the configured one when empty.
func (s *server) channelMembers(channel string) ([]loop.SlackTarget, error) {
	if !s.taggingLive() || s.deps.ChannelMembers == nil {
		return nil, errNoSlackDirectory
	}

	return directoryRead(s.deps.ChannelMembers(s.channelOr(channel)))
}

// userGroups is the workspace's user groups; a workspace that has none, or
// does not let the token read them, lists none.
func (s *server) userGroups() ([]loop.SlackTarget, error) {
	if !s.taggingLive() || s.deps.UserGroups == nil {
		return nil, errNoSlackDirectory
	}

	groups, err := directoryRead(s.deps.UserGroups())
	if errors.Is(err, messaging.ErrNoUserGroups) {
		return nil, nil
	}

	return groups, err
}

// directoryRead is a directory read as People and groups takes it: one with
// no credential to read with is no directory.
func directoryRead(entries []loop.SlackTarget, err error) ([]loop.SlackTarget, error) {
	if errors.Is(err, messaging.ErrNoCredential) {
		return nil, errNoSlackDirectory
	}

	return entries, err
}

// lookUp is the entry read lists under slackID.
func lookUp(read func() ([]loop.SlackTarget, error), slackID string) (loop.SlackTarget, error) {
	entries, err := read()
	if err != nil {
		return loop.SlackTarget{}, err
	}

	index := slices.IndexFunc(entries, hasID(slackID))
	if index < 0 {
		return loop.SlackTarget{}, errNotInDirectory
	}

	return entries[index], nil
}

// repoGroups is this repository's user groups, with its name.
func (s *server) repoGroups() (api.RepoGroups, error) {
	if s.deps.RepoGroups == nil {
		return api.RepoGroups{}, errNoPeopleStore
	}

	var groups []loop.SlackTarget

	err := s.inWorkspace(func(workspace string) error {
		var err error

		groups, err = s.deps.RepoGroups(workspace)

		return err
	})
	if err != nil {
		return api.RepoGroups{}, err
	}

	return api.RepoGroups{Repository: s.info.Repository, Groups: slackTargetsDTO(groups)}, nil
}

// setRepoGroups keeps ids, each once, as this repository's user groups.
func (s *server) setRepoGroups(ids []string) error {
	if s.deps.SetRepoGroups == nil || s.deps.RepoGroups == nil {
		return errNoPeopleStore
	}

	return s.inWorkspace(func(workspace string) error {
		return s.keptWrite(func() error {
			saved, err := s.deps.RepoGroups(workspace)
			if err != nil {
				return err
			}

			groups, err := s.labelGroups(ids, saved)
			if err != nil {
				return err
			}

			return s.deps.SetRepoGroups(workspace, groups)
		})
	})
}

// labelGroups is ids each once, labeled as saved when one already is — so a
// group Slack no longer lists, or a token that cannot read them, keeps it —
// and from the workspace's directory when it is new. Only a new group needs
// the directory.
func (s *server) labelGroups(ids []string, saved []loop.SlackTarget) ([]loop.SlackTarget, error) {
	groups := make([]loop.SlackTarget, 0, len(ids))

	for _, groupID := range ids {
		if slices.ContainsFunc(groups, hasID(groupID)) {
			continue
		}

		if index := slices.IndexFunc(saved, hasID(groupID)); index >= 0 {
			groups = append(groups, saved[index])

			continue
		}

		group, err := s.slackGroup(groupID)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// hasID reports whether a Slack user or group is the one id names.
func hasID(id string) func(loop.SlackTarget) bool {
	return func(target loop.SlackTarget) bool { return target.ID == id }
}

// channelOr is channel, or the configured one when channel is empty.
func (s *server) channelOr(channel string) string {
	if channel == "" {
		return s.config().Messaging.Channel
	}

	return channel
}

// directoryAnswer is a directory read as the answer carries it: a token
// without the scope the read needs answers no entries and names the scope.
func directoryAnswer(entries []loop.SlackTarget, err error) (api.SlackDirectory, error) {
	if scope, missing := missingScope(err); missing {
		return api.SlackDirectory{Entries: []api.SlackTarget{}, MissingScope: &scope}, nil
	}

	if err != nil {
		return api.SlackDirectory{}, err
	}

	return api.SlackDirectory{Entries: slackTargetsDTO(entries), MissingScope: nil}, nil
}

// missingScope is the scope err says the Slack token lacks, if it says so.
func missingScope(err error) (string, bool) {
	var missing *messaging.MissingScopeError
	if !errors.As(err, &missing) {
		return "", false
	}

	return missing.Needed, true
}

// peopleFault is the problem a People and groups request is refused with:
// what the request or the server's setup cannot carry out is unprocessable,
// and anything else is classified by fault.
func (s *server) peopleFault(err error) (api.Problem, int) {
	if errors.Is(err, loop.ErrUnknownWorkspace) {
		prob := problem(api.Unprocessable, workspaceRefusal(err))

		return prob, prob.Status
	}

	if scope, missing := missingScope(err); missing {
		prob := problem(api.Unprocessable, "the Slack token lacks the "+scope+
			" scope; add it to the Slack app, then sign in again with workflow slack login")

		return prob, prob.Status
	}

	for _, refusal := range peopleRefusals() {
		if errors.Is(err, refusal) {
			prob := problem(api.Unprocessable, refusal.Error())

			return prob, prob.Status
		}
	}

	return s.fault(err)
}

// peopleRefusals are the failures People and groups answers in their own
// words: its own refusals, the kept store's and Slack's directory's, none of
// which names a host or a path.
func peopleRefusals() []error {
	return []error{
		errNoSlackDirectory, errNoPeopleStore, errOneOrTheOther, errWrongKind, errNotInDirectory,
		store.ErrKeptFromNewerBuild, store.ErrInvalidOwner, store.ErrInvalidSlackID,
		messaging.ErrChannelNotFound, messaging.ErrLookupRefused, messaging.ErrDirectoryTooLarge,
	}
}

// ownerTagsDTO maps code owners as an announcement tags them onto the wire.
func ownerTagsDTO(owners []loop.OwnerTag) []api.OwnerTag {
	states := map[loop.OwnerState]api.OwnerTagState{
		loop.OwnerUnlinked:   api.Unlinked,
		loop.OwnerLinked:     api.Linked,
		loop.OwnerNotOnSlack: api.NotOnSlack,
	}
	kinds := map[bool]api.OwnerTagKind{false: api.User, true: api.Team}

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

// slackTargetsDTO maps Slack users or groups onto the wire.
func slackTargetsDTO(targets []loop.SlackTarget) []api.SlackTarget {
	mapped := make([]api.SlackTarget, 0, len(targets))
	for _, target := range targets {
		mapped = append(mapped, api.SlackTarget(target))
	}

	return mapped
}
