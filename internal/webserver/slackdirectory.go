// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetSlackMembers lists the people in a channel — the configured one when
// none is named — that a code owner who is a user can be linked to.
func (s *server) GetSlackMembers(
	_ context.Context, request api.GetSlackMembersRequestObject,
) (api.GetSlackMembersResponseObject, error) {
	members, err := s.channelMembers(orZero(request.Params.Channel))

	directory, err := directoryAnswer(members, err)
	if err != nil {
		return problemAnswer[api.GetSlackMembersdefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
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
		return problemAnswer[api.GetSlackGroupsdefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
	}

	return api.GetSlackGroups200JSONResponse(directory), nil
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
	if !s.taggingLive() || s.deps.Messaging.ChannelMembers == nil {
		return nil, errNoSlackDirectory
	}

	return directoryRead(s.deps.Messaging.ChannelMembers(s.channelOr(channel)))
}

// userGroups is the workspace's user groups; a workspace that has none, or
// does not let the token read them, lists none.
func (s *server) userGroups() ([]loop.SlackTarget, error) {
	if !s.taggingLive() || s.deps.Messaging.UserGroups == nil {
		return nil, errNoSlackDirectory
	}

	groups, err := directoryRead(s.deps.Messaging.UserGroups())
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

// slackTargetsDTO maps Slack users or groups onto the wire.
func slackTargetsDTO(targets []loop.SlackTarget) []api.SlackTarget {
	mapped := make([]api.SlackTarget, 0, len(targets))
	for _, target := range targets {
		mapped = append(mapped, api.SlackTarget(target))
	}

	return mapped
}
