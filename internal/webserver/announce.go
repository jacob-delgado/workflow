// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetAnnouncement composes the announcement for the checked-out branch's pull
// request without posting it, for a preview. It is a 409 when there is no pull
// request to announce; a branch or a pull request that cannot be read is
// classified by fault.
func (s *server) GetAnnouncement(
	_ context.Context, _ api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	announcement, err := s.announcement()
	if errors.Is(err, loop.ErrNoPullRequest) {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	if err != nil {
		body, code := s.fault(err)

		return api.GetAnnouncementdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	preview := announcementDTO(announcement, s.config().Messaging.Channel)
	preview.Tagging = s.tagging(announcement.Moment)

	return api.GetAnnouncement200JSONResponse(preview), nil
}

// Announce posts the composed announcement to the configured service — to the
// requested channel, or the configured one when none is given. It is a 409 when
// there is no pull request to announce, and when the request carries the text a
// preview showed and the announcement composed now reads differently, so what
// is posted is only ever what was shown. Given mentions, it tags the linked
// user owners and the groups named, each one the announcement offers, on a
// line after the text. A read that fails while composing it,
// and a post that fails, are classified by fault, whose details never carry the
// error's own text, which can name the forge or the webhook.
func (s *server) Announce(_ context.Context, request api.AnnounceRequestObject) (api.AnnounceResponseObject, error) {
	if s.deps.Post == nil {
		return announceUnprocessable("announcing is not available"), nil
	}

	announcement, err := s.announcement()
	if errors.Is(err, loop.ErrNoPullRequest) {
		return api.Announce409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	if err != nil {
		return s.announceFault(err), nil
	}

	if changedSincePreview(request.Body.Text, announcement) {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"the announcement changed since it was previewed; preview it again")), nil
	}

	mentions, memory, err := s.mentions(request.Body.Mentions, announcement.Moment)
	if err != nil {
		return announceUnprocessable(err.Error()), nil
	}

	channel := s.channelOr(request.Body.Channel)

	err = loop.Deliver(s.deps.Post, memory, loop.Delivery{
		Channel: channel, Text: announcement.Text(), Made: loop.Announced{}, Mentions: mentions,
	})
	if err != nil {
		return s.announceFault(err), nil
	}

	return api.Announce200JSONResponse(announcementDTO(announcement, channel)), nil
}

// announcement is the announcement for the checked-out branch's pull request,
// composed from the pull request, the branch's issue, and the configured
// template. It fails with loop.ErrNoPullRequest when there is no pull request
// to announce, and with the read's own error when the branch or the pull
// request cannot be read.
func (s *server) announcement() (messaging.Announcement, error) {
	cfg := s.config()

	announcement, _, err := loop.ComposeAnnouncement(loop.AnnounceSeams{
		Branch:    s.deps.Branch,
		FindPull:  s.deps.FindPull,
		Author:    s.authorSeam(),
		Issue:     s.deps.Issue,
		BrowseURL: s.deps.BrowseURL,
		CheckCI:   s.deps.CheckCI,
	}, cfg.Messaging, cfg.Jira.Project, s.forgeKindNow())

	return announcement, err
}

// authorSeam is the author read the announcement composes with: the one the
// messaging panel shows, so a preview and its post name the same author even
// when the forge cannot say by the time of the post. It is nil, as the loop
// takes for no author, when no forge is configured.
func (s *server) authorSeam() func() (string, error) {
	if s.deps.Author == nil {
		return nil
	}

	return s.cachedAuthor
}

// changedSincePreview reports whether a preview's text was given and the
// announcement composed now reads differently from it.
func changedSincePreview(previewed *string, announcement messaging.Announcement) bool {
	return previewed != nil && *previewed != announcement.Text()
}

// announcementDTO maps the composed announcement and its channel onto the wire.
func announcementDTO(announcement messaging.Announcement, channel string) api.Announcement {
	return api.Announcement{Text: announcement.Text(), Channel: channel}
}

// announceFault answers an announcement that failed, in reading what it
// announces or in posting it, through fault.
func (s *server) announceFault(err error) api.AnnounceResponseObject {
	body, code := s.fault(err)

	return api.AnnouncedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
}

// announceUnprocessable is the 422 response for an announcement the server will
// not post.
func announceUnprocessable(message string) api.Announce422ApplicationProblemPlusJSONResponse {
	return api.Announce422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}

// nothingToAnnounce refuses an announcement with no pull request to announce,
// named in the forge's own noun.
func (s *server) nothingToAnnounce() api.Problem {
	return problem(api.Conflict, "there is no "+s.noun()+" to announce")
}

// errNoTags refuses mentions for an announcement that tags no one.
var errNoTags = errors.New("this announcement tags no one: only a ready-for-review announcement " +
	"with a Slack user token does; preview it again")

// tagging is whom an announcement at moment proposes to tag, for its preview,
// and a scope the Slack token lacks to link an owner not yet linked. It is nil
// without a Slack user token, which tagging needs.
func (s *server) tagging(moment messaging.Moment) *api.AnnouncementTagging {
	if s.deps.ChannelMembers == nil {
		return nil
	}

	tags, available := s.proposedTags(moment)
	tagging := api.AnnouncementTagging{
		Available: available, MissingScope: nil, Owners: ownerTagsDTO(tags.Owners), Groups: groupTagsDTO(tags.Groups),
	}

	if scope, missing := s.scopeToLink(tags.Owners); missing {
		tagging.MissingScope = &scope
	}

	return &tagging
}

// proposedTags is whom an announcement at moment proposes to tag, and
// whether it tags anyone at all: only one ready for review does, and only
// with the store that keeps who is whom. The tags are a proposal, so a kept
// read that fails proposes fewer rather than holding the announcement back.
func (s *server) proposedTags(moment messaging.Moment) (loop.Tags, bool) {
	if moment != messaging.MomentReady || s.deps.OwnerLinks == nil || s.deps.RepoGroups == nil {
		return loop.Tags{}, false
	}

	links, _ := s.deps.OwnerLinks()
	repoGroups, _ := s.deps.RepoGroups()

	var (
		last   []string
		chosen bool
	)

	if s.deps.LastGroups != nil {
		last, chosen = s.deps.LastGroups()
	}

	return loop.ProposeTags(s.branchOwners(), links, repoGroups, last, chosen, moment), true
}

// scopeToLink is a scope the Slack token lacks to read whom an unlinked owner
// could be linked to: the configured channel's members for a person, the user
// groups for a team. Any other failure is the picker's to show when it reads.
func (s *server) scopeToLink(owners []loop.OwnerTag) (string, bool) {
	reads := map[bool]func() ([]loop.SlackTarget, error){
		false: func() ([]loop.SlackTarget, error) { return s.deps.ChannelMembers(s.channelOr("")) },
		true:  s.deps.UserGroups,
	}

	for _, team := range []bool{false, true} {
		unlinked := func(owner loop.OwnerTag) bool { return owner.Team == team && owner.State == loop.OwnerUnlinked }
		if reads[team] == nil || !slices.ContainsFunc(owners, unlinked) {
			continue
		}

		_, err := reads[team]()
		if scope, missing := missingScope(err); missing {
			return scope, true
		}
	}

	return "", false
}

// mentions is whom a post tags — the linked user owners, and the groups
// asked for, each one the announcement at moment offers — and what it
// remembers of the choice: the groups chosen, when there were groups to
// choose. No mentions asked for tags no one.
func (s *server) mentions(asked *api.AnnounceMentions, moment messaging.Moment) (
	messaging.Mentions, loop.AnnounceMemory, error,
) {
	if asked == nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, nil
	}

	tags, available := s.proposedTags(moment)
	if !available || s.deps.ChannelMembers == nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, errNoTags
	}

	mentions, err := tags.Mentions(asked.Groups)
	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, err
	}

	memory := loop.AnnounceMemory{Recorded: nil, Record: nil, RecordGroups: nil}
	if len(tags.Groups) > 0 {
		memory.RecordGroups = s.deps.RecordGroups
	}

	return mentions, memory, nil
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
