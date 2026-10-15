// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// What a post's mentions are refused with.
var (
	errNoTags = errors.New("this announcement tags no one: only a ready-for-review announcement " +
		"with a Slack user token does; preview it again")
	errTagsChanged = errors.New("whom the announcement tags changed since it was previewed; preview it again")
)

// tagging is whom an announcement at moment proposes to tag, for its preview
// to channel, and a scope the Slack token lacks to link an owner not yet
// linked. It is nil without a Slack user token in the configuration in
// effect, or one Slack has no credential for, which tagging needs. Slack is
// asked under ctx.
func (s *server) tagging(ctx context.Context, moment messaging.Moment, channel string) *api.AnnouncementTagging {
	if !s.canReadDirectory(ctx, channel) {
		return nil
	}

	workspace, err := loop.TagWorkspace(ctx, s.deps.Messaging.Workspace)
	if errors.Is(err, messaging.ErrNoCredential) {
		return nil
	}

	if err != nil {
		return untagged(err)
	}

	tags, available := s.proposedTags(ctx, moment, workspace, s.branchOwners)

	scope, err := s.scopeToLink(ctx, tags.Owners, channel)
	if err != nil {
		return nil
	}

	tagging := api.AnnouncementTagging{
		Available: available, MissingScope: nil, Owners: ownerTagsDTO(tags.Owners), Groups: groupTagsDTO(tags.Groups),
	}
	if scope != "" {
		tagging.MissingScope = &scope
	}

	return &tagging
}

// canReadDirectory reports a Slack directory to tag from: a Slack user token
// in the configuration in effect, which Settings may change while the server
// runs, and a directory that does not answer ErrNoCredential, as it does when
// Slack holds no credential for that token. Any other failure is the
// preview's to show. The directory is read under ctx.
func (s *server) canReadDirectory(ctx context.Context, channel string) bool {
	if !s.taggingLive() || s.deps.Messaging.ChannelMembers == nil {
		return false
	}

	_, err := s.deps.Messaging.ChannelMembers(ctx, s.channelOr(channel))

	return !errors.Is(err, messaging.ErrNoCredential)
}

// untagged is the tagging of an announcement that tags no one because the
// Slack workspace could not be read, and why.
func untagged(err error) *api.AnnouncementTagging {
	reason := workspaceRefusal(err)

	return &api.AnnouncementTagging{
		Available: false, MissingScope: nil, UnavailableReason: &reason,
		Owners: []api.OwnerTag{}, Groups: []api.GroupTag{},
	}
}

// workspaceRefusal says why the Slack workspace could not be read: in fault's
// words where it classifies the cause, since an unreachable Slack's own words
// can carry an address, and as no more than that otherwise.
func workspaceRefusal(err error) string {
	prob, classified := faultProblem(err)
	if !classified {
		return loop.ErrUnknownWorkspace.Error()
	}

	return loop.ErrUnknownWorkspace.Error() + ": " + prob.Detail
}

// proposedTags is whom an announcement at moment proposes to tag from what
// is kept in workspace, the owners those owners reads, and whether it tags
// anyone at all: only one ready for review does, and only with the store that
// keeps who is whom. The tags are a proposal, so a kept read that fails
// proposes fewer rather than holding the announcement back. What is kept is
// read under ctx.
func (s *server) proposedTags(
	ctx context.Context, moment messaging.Moment, workspace string, owners func(context.Context) codeowners.Owners,
) (loop.Tags, bool) {
	if moment != messaging.MomentReady || s.deps.Store.OwnerLinks == nil || s.deps.Store.RepoGroups == nil {
		return loop.Tags{}, false
	}

	links, _ := s.deps.Store.OwnerLinks(ctx, workspace)
	repoGroups, _ := s.deps.Store.RepoGroups(ctx, workspace)

	var (
		last   []string
		chosen bool
	)

	if s.deps.Store.LastGroups != nil {
		last, chosen = s.deps.Store.LastGroups(ctx, workspace)
	}

	return loop.ProposeTags(owners(ctx), links, repoGroups, last, chosen, moment), true
}

// scopeToLink is a scope the Slack token lacks to read whom an unlinked owner
// could be linked to — channel's members for a person, the user groups for a
// team — or "" when it lacks none; and errNoSlackDirectory when there is no
// directory to read, read under ctx. Any other failure is the picker's to
// show when it reads.
func (s *server) scopeToLink(ctx context.Context, owners []loop.OwnerTag, channel string) (string, error) {
	reads := map[bool]func() ([]loop.SlackTarget, error){
		false: func() ([]loop.SlackTarget, error) { return s.channelMembers(ctx, channel) },
		true:  func() ([]loop.SlackTarget, error) { return s.userGroups(ctx) },
	}

	for _, team := range []bool{false, true} {
		unlinked := func(owner loop.OwnerTag) bool { return owner.Team == team && owner.State == loop.OwnerUnlinked }
		if !slices.ContainsFunc(owners, unlinked) {
			continue
		}

		_, err := reads[team]()
		if errors.Is(err, errNoSlackDirectory) {
			return "", err
		}

		if scope, missing := missingScope(err); missing {
			return scope, nil
		}
	}

	return "", nil
}

// mentions is whom a post tags — the linked user owners, and the groups
// asked for, each one the announcement at moment offers — and what it
// remembers of the choice: the groups chosen, when there were groups to
// choose. No mentions asked for tags no one. Slack is asked under ctx.
func (s *server) mentions(ctx context.Context, asked *api.AnnounceMentions, moment messaging.Moment) (
	messaging.Mentions, loop.AnnounceMemory, error,
) {
	if asked == nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, nil
	}

	tags, workspace, err := s.tagsAsPreviewed(ctx, asked.Users, moment)
	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, err
	}

	mentions, err := tags.Mentions(asked.Groups)
	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, err
	}

	memory := loop.AnnounceMemory{Recorded: nil, Record: nil, RecordGroups: nil}
	if len(tags.Groups) > 0 && s.deps.Store.RecordGroups != nil {
		memory.RecordGroups = func(ctx context.Context, ids []string) error {
			return s.keptWrite(func() error { return s.deps.Store.RecordGroups(ctx, workspace, ids) })
		}
	}

	return mentions, memory, nil
}

// tagsAsPreviewed is whom an announcement at moment proposes to tag, and the
// Slack workspace they are kept in, refused when it tags no one now — not
// ready for review, no Slack user token in the configuration in effect, or no
// workspace to read its links in — and when the user owners it links are not
// previewed, the ones its preview showed. Slack is asked under ctx.
func (s *server) tagsAsPreviewed(
	ctx context.Context, previewed []string, moment messaging.Moment,
) (loop.Tags, string, error) {
	if !s.canReadDirectory(ctx, "") {
		return loop.Tags{}, "", errNoTags
	}

	workspace, err := loop.TagWorkspace(ctx, s.deps.Messaging.Workspace)
	if err != nil {
		return loop.Tags{}, "", errNoTags
	}

	tags, available := s.proposedTags(ctx, moment, workspace, s.branchOwners)
	if !available {
		return loop.Tags{}, "", errNoTags
	}

	if !slices.Equal(idSet(previewed), idSet(linkedUsers(tags))) {
		return loop.Tags{}, "", errTagsChanged
	}

	return tags, workspace, nil
}

// linkedUsers are the Slack users the user owners among tags are linked to.
func linkedUsers(tags loop.Tags) []string {
	var users []string

	for _, owner := range tags.Owners {
		if !owner.Team && owner.State == loop.OwnerLinked {
			users = append(users, owner.Slack.ID)
		}
	}

	return users
}

// idSet is ids sorted, each once, to compare as a set.
func idSet(ids []string) []string {
	set := slices.Clone(ids)
	slices.Sort(set)

	return slices.Compact(set)
}

// groupTagsDTO maps the user groups an announcement offers onto the wire.
func groupTagsDTO(groups []loop.GroupTag) []api.GroupTag {
	mapped := make([]api.GroupTag, 0, len(groups))
	for _, group := range groups {
		mapped = append(mapped, api.GroupTag{
			Slack: api.SlackTarget(group.Slack), Checked: group.Checked, FromOwners: group.FromOwners,
		})
	}

	return mapped
}
