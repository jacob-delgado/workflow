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
// request without posting it, for a preview — to the channel asked, or the
// configured one. It is a 409 when there is no pull request to announce; a
// branch or a pull request that cannot be read is classified by fault.
func (s *server) GetAnnouncement(
	_ context.Context, request api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	announcement, err := s.announcement()
	if errors.Is(err, loop.ErrNoPullRequest) {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	if err != nil {
		body, code := s.fault(err)

		return api.GetAnnouncementdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	channel := orZero(request.Params.Channel)
	preview := announcementDTO(announcement, s.channelOr(channel))
	preview.Tagging = s.tagging(announcement.Moment, channel)

	return api.GetAnnouncement200JSONResponse(preview), nil
}

// Announce posts the composed announcement to the configured service — to the
// requested channel, or the configured one when none is given. It is a 409 when
// there is no pull request to announce, and when the request carries the text a
// preview showed and the announcement composed now reads differently, so what
// is posted is only ever what was shown. Given mentions, it tags the linked
// user owners and the groups named, each one the announcement offers, on a
// line after the text — and is a 409 too when the linked owners are not the
// ones the preview showed. A read that fails while composing it,
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
	if errors.Is(err, errTagsChanged) {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.Conflict, err.Error())), nil
	}

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

// What a post's mentions are refused with.
var (
	errNoTags = errors.New("this announcement tags no one: only a ready-for-review announcement " +
		"with a Slack user token does; preview it again")
	errTagsChanged = errors.New("whom the announcement tags changed since it was previewed; preview it again")
)

// tagging is whom an announcement at moment proposes to tag, for its preview
// to channel, and a scope the Slack token lacks to link an owner not yet
// linked. It is nil without a Slack user token in the configuration in
// effect, or one Slack has no credential for, which tagging needs.
func (s *server) tagging(moment messaging.Moment, channel string) *api.AnnouncementTagging {
	if !s.canReadDirectory(channel) {
		return nil
	}

	tags, available := s.proposedTags(moment)

	scope, err := s.scopeToLink(tags.Owners, channel)
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
// preview's to show.
func (s *server) canReadDirectory(channel string) bool {
	if !s.taggingLive() || s.deps.ChannelMembers == nil {
		return false
	}

	_, err := s.deps.ChannelMembers(s.channelOr(channel))

	return !errors.Is(err, messaging.ErrNoCredential)
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
// could be linked to — channel's members for a person, the user groups for a
// team — or "" when it lacks none; and errNoSlackDirectory when there is no
// directory to read. Any other failure is the picker's to show when it reads.
func (s *server) scopeToLink(owners []loop.OwnerTag, channel string) (string, error) {
	reads := map[bool]func() ([]loop.SlackTarget, error){
		false: func() ([]loop.SlackTarget, error) { return s.channelMembers(channel) },
		true:  s.userGroups,
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
// choose. No mentions asked for tags no one.
func (s *server) mentions(asked *api.AnnounceMentions, moment messaging.Moment) (
	messaging.Mentions, loop.AnnounceMemory, error,
) {
	if asked == nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, nil
	}

	tags, err := s.tagsAsPreviewed(asked.Users, moment)
	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, err
	}

	mentions, err := tags.Mentions(asked.Groups)
	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, err
	}

	memory := loop.AnnounceMemory{Recorded: nil, Record: nil, RecordGroups: nil}
	if len(tags.Groups) > 0 && s.deps.RecordGroups != nil {
		memory.RecordGroups = func(ids []string) error {
			return s.keptWrite(func() error { return s.deps.RecordGroups(ids) })
		}
	}

	return mentions, memory, nil
}

// tagsAsPreviewed is whom an announcement at moment proposes to tag, refused
// when it tags no one now — not ready for review, or no Slack user token in
// the configuration in effect — and when the user owners it links are not
// previewed, the ones its preview showed.
func (s *server) tagsAsPreviewed(previewed []string, moment messaging.Moment) (loop.Tags, error) {
	if !s.canReadDirectory("") {
		return loop.Tags{}, errNoTags
	}

	tags, available := s.proposedTags(moment)
	if !available {
		return loop.Tags{}, errNoTags
	}

	if !slices.Equal(idSet(previewed), idSet(linkedUsers(tags))) {
		return loop.Tags{}, errTagsChanged
	}

	return tags, nil
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
