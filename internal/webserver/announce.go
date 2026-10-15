// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetAnnouncement composes the announcement for the checked-out branch's pull
// request without posting it, for a preview — to the channel asked, or the
// configured one — saying whether it can wait for the pull request's CI. It
// is a 409 when there is no pull request to announce; a branch or a pull
// request that cannot be read is classified by fault.
func (s *server) GetAnnouncement(
	ctx context.Context, request api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	composed, err := s.announcement(ctx)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	if err != nil {
		return problemAnswer[api.GetAnnouncementdefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	moment := composed.Announcement.Moment
	channel := orZero(request.Params.Channel)
	preview := announcementDTO(composed.Announcement.Text(), s.channelOr(channel))
	preview.Tagging = s.tagging(ctx, moment, channel)

	_, ci, read := s.readCI(ctx, composed.Pull)
	if read {
		preview.Ci = new(ciState(ci.State))
	}

	if canWaitForCI(moment, ci, read) {
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

// announcePost is an announcement ready to go: the delivery, which names the
// pull request it announces and the moment it marks, what posting it
// remembers, the branch it was composed on, and whether it was armed as its
// pull request opened rather than asked for.
type announcePost struct {
	delivery  loop.Delivery
	memory    loop.AnnounceMemory
	branch    string
	automatic bool
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
func (s *server) Announce(ctx context.Context, request api.AnnounceRequestObject) (api.AnnounceResponseObject, error) {
	if s.deps.Messaging.Post == nil {
		return announceUnprocessable("announcing is not available"), nil
	}

	post, refusal := s.postFor(ctx, *request.Body)
	if refusal != nil {
		return refusal, nil
	}

	if orZero(request.Body.When) == api.AnnounceRequestWhenCiPasses {
		return s.announceWhenGreen(ctx, post), nil
	}

	return s.announceNow(ctx, post), nil
}

// postFor is the announcement the request asks to post — composed now, in the
// text it was edited to, if it was — or the answer refusing it. Slack is
// asked under ctx.
func (s *server) postFor(ctx context.Context, body api.AnnounceRequest) (announcePost, api.AnnounceResponseObject) {
	composed, err := s.announcement(ctx)
	if errors.Is(err, loop.ErrNoPullRequest) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce())
	}

	if err != nil {
		return announcePost{}, s.announceFault(err)
	}

	announcement := composed.Announcement

	text, err := postedText(body, announcement)
	if errors.Is(err, errChangedSincePreview) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeConflict, err.Error()))
	}

	if err != nil {
		return announcePost{}, announceUnprocessable(err.Error())
	}

	if s.announcedAlready(ctx, composed.Made()) {
		return announcePost{}, api.Announce409ApplicationProblemPlusJSONResponse(
			s.announcedBefore(composed.Pull.Number))
	}

	mentions, memory, refusal := s.postMentions(ctx, body.Mentions, announcement.Moment)
	if refusal != nil {
		return announcePost{}, refusal
	}

	memory.Recorded, memory.Record = s.recordedAnnouncements, s.recordAnnouncement

	return announcePost{
		delivery: composed.Delivery(s.channelOr(body.Channel), text, mentions), memory: memory,
		branch: composed.Branch.Name,
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
	ctx context.Context, asked *api.AnnounceMentions, moment messaging.Moment,
) (messaging.Mentions, loop.AnnounceMemory, api.AnnounceResponseObject) {
	mentions, memory, err := s.mentions(ctx, asked, moment)
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
// twice; one the gate refuses puts the held one back to wait. It is a 409
// while the held one is being posted, and when the announcement was made
// meanwhile; one posted that the store could not remember is answered as
// posted, with the warning. It is posted under ctx.
func (s *server) announceNow(ctx context.Context, post announcePost) api.AnnounceResponseObject {
	dropped, err := s.dropHeld()
	if err != nil {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error()))
	}

	warning, err := s.deliver(ctx, post)
	if errors.Is(err, loop.ErrCIFailed) || errors.Is(err, loop.ErrCIUnread) {
		s.restoreHeld(dropped)
	}

	if errors.Is(err, errAnnouncedAlready) {
		return api.Announce409ApplicationProblemPlusJSONResponse(s.announcedBefore(post.delivery.Pull.Number))
	}

	if errors.Is(err, loop.ErrCIFailed) {
		return api.Announce409ApplicationProblemPlusJSONResponse(s.ciFailed(post.delivery))
	}

	if err != nil {
		return s.announceFault(err)
	}

	posted := announcementDTO(post.delivery.Text, post.delivery.Channel)
	posted.Warning = optional(warning)

	return api.Announce200JSONResponse(posted)
}

// ciFailed refuses to post delivery now, since a check failed on its pull
// request, saying the way on: a pull request ready for review can wait for a
// run that passes; a merge, the other moment, has no CI to wait for, so its
// failed checks are re-run.
func (s *server) ciFailed(delivery loop.Delivery) api.Problem {
	failed := "a check failed on " + s.pullName(delivery.Pull.Number)
	if delivery.Moment == messaging.MomentReady {
		return problem(api.ProblemCodeCiFailed, failed+
			", so nothing was announced; announce it when CI passes to wait for a fix")
	}

	return problem(api.ProblemCodeCiFailed, failed+
		", so its merge was not announced; re-run the failed checks on the forge, then announce it once they pass")
}

// announcement is the announcement for the checked-out branch's pull request,
// composed from the pull request, the branch's issue, and the configured
// template, with the pull request it announces and the branch it read. It
// fails with loop.ErrNoPullRequest when there is no pull request to announce,
// and with the read's own error when the branch or the pull request cannot be
// read. The tracker is asked under ctx.
func (s *server) announcement(ctx context.Context) (loop.Composed, error) {
	cfg := s.config()

	return loop.ComposeAnnouncement(ctx, loop.AnnounceSeams{
		Branch:    s.deps.Git.Branch,
		FindPull:  s.deps.Forge.FindPullRequest,
		Author:    s.authorSeam(),
		Issue:     s.deps.Jira.Issue,
		BrowseURL: s.deps.Jira.BrowseURL,
	}, cfg.Messaging, cfg.Jira.Project, s.forgeKindNow())
}

// authorSeam is the author read the announcement composes with: the one the
// messaging panel shows, so a preview and its post name the same author even
// when the forge cannot say by the time of the post. It is nil, as the loop
// takes for no author, when no forge is configured.
func (s *server) authorSeam() func(context.Context) (string, error) {
	if s.deps.Forge.Author == nil {
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
