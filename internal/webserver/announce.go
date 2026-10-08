// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetAnnouncement composes the announcement for the checked-out branch's pull
// request without posting it, for a preview — to the channel asked, or the
// configured one — saying whether it can wait for the pull request's CI. It
// is a 409 when there is no pull request to announce; a branch or a pull
// request that cannot be read is classified by fault.
func (s *server) GetAnnouncement(
	_ context.Context, request api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	announcement, pull, err := s.announcement()
	if errors.Is(err, loop.ErrNoPullRequest) {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	if err != nil {
		return problemAnswer[api.GetAnnouncementdefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	channel := orZero(request.Params.Channel)
	preview := announcementDTO(announcement.Text(), s.channelOr(channel))
	preview.Tagging = s.tagging(announcement.Moment, channel)

	if s.canWaitForCI(announcement.Moment, pull) {
		preview.CanWaitForCi = new(true)
	}

	return api.GetAnnouncement200JSONResponse(preview), nil
}

// Why an announcement is not posted as asked.
var (
	// errEditWithoutPreview is edited text with no word of the announcement
	// it was edited from, which could have changed since.
	errEditWithoutPreview = errors.New("an edited announcement needs the text it was edited from; preview it again")
	// errChangedSincePreview is an announcement composed now that reads
	// differently from the one previewed.
	errChangedSincePreview = errors.New("the announcement changed since it was previewed; preview it again")
)

// announcePost is an announcement ready to go: what it marks, the pull
// request it announces, the delivery and what posting it remembers.
type announcePost struct {
	moment   messaging.Moment
	pull     forge.PullRequest
	delivery loop.Delivery
	memory   loop.AnnounceMemory
}

// Announce posts the composed announcement to the configured service — to the
// requested channel, or the configured one when none is given — or holds it
// until the pull request's CI passes when asked to. It is a 409 when there is
// no pull request to announce, and when the request carries the text a
// preview showed and the announcement composed now reads differently, so what
// is posted is only ever what was shown, or an edit of it. Given mentions, it
// tags the linked user owners and the groups named, each one the announcement
// offers, on a line after the text — and is a 409 too when the linked owners
// are not the ones the preview showed. A read that fails while composing it,
// and a post that fails, are classified by fault, whose details never carry
// the error's own text, which can name the forge or the webhook.
func (s *server) Announce(_ context.Context, request api.AnnounceRequestObject) (api.AnnounceResponseObject, error) {
	if s.deps.Post == nil {
		return announceUnprocessable("announcing is not available"), nil
	}

	post, refusal := s.postFor(*request.Body)
	if refusal != nil {
		return refusal, nil
	}

	if orZero(request.Body.When) == api.AnnounceRequestWhenCiPasses {
		return s.announceWhenGreen(post), nil
	}

	return s.announceNow(post), nil
}

// postFor is the announcement the request asks to post — composed now, in the
// text it was edited to, if it was — or the answer refusing it.
func (s *server) postFor(body api.AnnounceRequest) (announcePost, api.AnnounceResponseObject) {
	announcement, pull, err := s.announcement()
	if errors.Is(err, loop.ErrNoPullRequest) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce())
	}

	if err != nil {
		return announcePost{}, s.announceFault(err)
	}

	text, err := postedText(body, announcement)
	if errors.Is(err, errChangedSincePreview) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeConflict, err.Error()))
	}

	if err != nil {
		return announcePost{}, announceUnprocessable(err.Error())
	}

	made := loop.Announced{Pull: pull.Number, Moment: announcement.Moment}
	if s.announcedAlready(made) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(s.announcedBefore(pull.Number))
	}

	mentions, memory, refusal := s.postMentions(body.Mentions, announcement.Moment)
	if refusal != nil {
		return announcePost{}, refusal
	}

	memory.Recorded, memory.Record = s.recordedAnnouncements, s.recordAnnouncement

	return announcePost{
		moment: announcement.Moment, pull: pull, memory: memory,
		delivery: loop.Delivery{
			Channel: s.channelOr(body.Channel), Text: text, Made: made, Mentions: mentions,
		},
	}, nil
}

// postedText is the text a post sends: the edit, when the request carries
// one, of an announcement that reads now as the preview it began from did, and
// the announcement composed now otherwise.
func postedText(body api.AnnounceRequest, announcement messaging.Announcement) (string, error) {
	switch {
	case body.EditedText != nil && body.Text == nil:
		return "", errEditWithoutPreview
	case changedSincePreview(body.Text, announcement):
		return "", errChangedSincePreview
	case body.EditedText == nil:
		return announcement.Text(), nil
	case strings.TrimSpace(*body.EditedText) == "":
		return "", loop.ErrEmptyAnnouncement
	default:
		return *body.EditedText, nil
	}
}

// postMentions is whom a post tags and what it remembers of the choice, or
// the answer refusing mentions that are not the ones previewed or offered.
func (s *server) postMentions(
	asked *api.AnnounceMentions, moment messaging.Moment,
) (messaging.Mentions, loop.AnnounceMemory, api.AnnounceResponseObject) {
	mentions, memory, err := s.mentions(asked, moment)
	if errors.Is(err, errTagsChanged) {
		return messaging.Mentions{}, loop.AnnounceMemory{},
			api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error()))
	}

	if err != nil {
		return messaging.Mentions{}, loop.AnnounceMemory{}, announceUnprocessable(err.Error())
	}

	return mentions, memory, nil
}

// announceNow posts the announcement at once, dropping any held for CI, which
// would otherwise follow it once CI passed, and the channel would read it
// twice. It is a 409 while the held one is being posted, and when the
// announcement was made meanwhile.
func (s *server) announceNow(post announcePost) api.AnnounceResponseObject {
	_, err := s.dropHeld()
	if err != nil {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error()))
	}

	err = s.deliver(post)
	if errors.Is(err, errAnnouncedAlready) {
		return api.Announce409ApplicationProblemPlusJSONResponse(s.announcedBefore(post.pull.Number))
	}

	if err != nil {
		return s.announceFault(err)
	}

	return api.Announce200JSONResponse(announcementDTO(post.delivery.Text, post.delivery.Channel))
}

// errAnnouncedAlready is an announcement made already at its moment, from
// here or from a terminal.
var errAnnouncedAlready = errors.New("announced already at this moment")

// deliver posts post, and records what it made, unless that was made already.
// The check, the post and the record are one step under delivering, so two
// asks at once — two tabs, or a held announcement and one made now — post it
// once, and the second learns it was made.
func (s *server) deliver(post announcePost) error {
	s.delivering.Lock()
	defer s.delivering.Unlock()

	if s.announcedAlready(post.delivery.Made) {
		return errAnnouncedAlready
	}

	return loop.Deliver(s.deps.Post, post.memory, post.delivery)
}

// announcedBefore refuses to announce pull at a moment it was announced at
// already.
func (s *server) announcedBefore(pull int) api.Problem {
	return problem(api.ProblemCodeConflict,
		s.pullName(pull)+" was already announced at this moment, here or from a terminal")
}

// announcement is the announcement for the checked-out branch's pull request,
// composed from the pull request, the branch's issue, and the configured
// template, and the pull request it announces. It fails with
// loop.ErrNoPullRequest when there is no pull request to announce, and with
// the read's own error when the branch or the pull request cannot be read.
func (s *server) announcement() (messaging.Announcement, forge.PullRequest, error) {
	cfg := s.config()

	return loop.ComposeAnnouncement(loop.AnnounceSeams{
		Branch:    s.deps.Branch,
		FindPull:  s.deps.FindPull,
		Author:    s.authorSeam(),
		Issue:     s.deps.Issue,
		BrowseURL: s.deps.BrowseURL,
		CheckCI:   s.deps.CheckCI,
	}, cfg.Messaging, cfg.Jira.Project, s.forgeKindNow())
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

// announcementDTO maps an announcement's text and its channel onto the wire.
func announcementDTO(text, channel string) api.Announcement {
	return api.Announcement{Text: text, Channel: channel}
}

// announceFault answers an announcement that failed, in reading what it
// announces or in posting it, through fault.
func (s *server) announceFault(err error) api.AnnounceResponseObject {
	return problemAnswer[api.AnnouncedefaultApplicationProblemPlusJSONResponse](s.fault(err))
}

// announceUnprocessable is the 422 response for an announcement the server will
// not post.
func announceUnprocessable(message string) api.Announce422ApplicationProblemPlusJSONResponse {
	return api.Announce422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, message))
}

// nothingToAnnounce refuses an announcement with no pull request to announce,
// named in the forge's own noun.
func (s *server) nothingToAnnounce() api.Problem {
	return problem(api.ProblemCodeConflict, "there is no "+s.noun()+" to announce")
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

	workspace, err := loop.TagWorkspace(s.deps.Workspace)
	if errors.Is(err, messaging.ErrNoCredential) {
		return nil
	}

	if err != nil {
		return untagged(err)
	}

	tags, available := s.proposedTags(moment, workspace)

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
// is kept in workspace, and whether it tags anyone at all: only one ready for
// review does, and only with the store that keeps who is whom. The tags are a
// proposal, so a kept read that fails proposes fewer rather than holding the
// announcement back.
func (s *server) proposedTags(moment messaging.Moment, workspace string) (loop.Tags, bool) {
	if moment != messaging.MomentReady || s.deps.OwnerLinks == nil || s.deps.RepoGroups == nil {
		return loop.Tags{}, false
	}

	links, _ := s.deps.OwnerLinks(workspace)
	repoGroups, _ := s.deps.RepoGroups(workspace)

	var (
		last   []string
		chosen bool
	)

	if s.deps.LastGroups != nil {
		last, chosen = s.deps.LastGroups(workspace)
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

	tags, workspace, err := s.tagsAsPreviewed(asked.Users, moment)
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
			return s.keptWrite(func() error { return s.deps.RecordGroups(workspace, ids) })
		}
	}

	return mentions, memory, nil
}

// tagsAsPreviewed is whom an announcement at moment proposes to tag, and the
// Slack workspace they are kept in, refused when it tags no one now — not
// ready for review, no Slack user token in the configuration in effect, or no
// workspace to read its links in — and when the user owners it links are not
// previewed, the ones its preview showed.
func (s *server) tagsAsPreviewed(previewed []string, moment messaging.Moment) (loop.Tags, string, error) {
	if !s.canReadDirectory("") {
		return loop.Tags{}, "", errNoTags
	}

	workspace, err := loop.TagWorkspace(s.deps.Workspace)
	if err != nil {
		return loop.Tags{}, "", errNoTags
	}

	tags, available := s.proposedTags(moment, workspace)
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
