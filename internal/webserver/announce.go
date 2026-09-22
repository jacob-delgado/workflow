// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// errNoPullRequest refuses an announcement with nothing to announce.
var errNoPullRequest = errors.New("there is no pull request to announce")

// GetAnnouncement composes the announcement for the checked-out branch's pull
// request without posting it, for a preview. It is a 409 when there is no pull
// request to announce.
func (s *server) GetAnnouncement(
	_ context.Context, _ api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	announcement, ok := s.composeAnnouncement()
	if !ok {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errNoPullRequest.Error())), nil
	}

	return api.GetAnnouncement200JSONResponse(announcementDTO(announcement, s.config().Messaging.Channel)), nil
}

// Announce posts the composed announcement to Slack — to the requested channel,
// or the configured one when none is given. It is a 409 when there is no pull
// request to announce, and a 422 when the post fails.
func (s *server) Announce(_ context.Context, request api.AnnounceRequestObject) (api.AnnounceResponseObject, error) {
	if request.Body == nil {
		return announceUnprocessable("a request body is required"), nil
	}

	if s.deps.Post == nil {
		return announceUnprocessable("announcing is not available"), nil
	}

	announcement, ok := s.composeAnnouncement()
	if !ok {
		return api.Announce409ApplicationProblemPlusJSONResponse(problem(api.Conflict, errNoPullRequest.Error())), nil
	}

	channel := request.Body.Channel
	if channel == "" {
		channel = s.config().Messaging.Channel
	}

	err := s.deps.Post(channel, announcement.Text())
	if err != nil {
		//nolint:nilerr // the failure is answered with a 422; the error may name the webhook, so it is not surfaced
		return announceUnprocessable("the announcement could not be posted"), nil
	}

	return api.Announce200JSONResponse(announcementDTO(announcement, channel)), nil
}

// composeAnnouncement builds the announcement for the checked-out branch's pull
// request from the pull request, the branch's issue, and the configured
// template. It reports false when there is no pull request to announce.
func (s *server) composeAnnouncement() (messaging.Announcement, bool) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return messaging.Announcement{}, false
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return messaging.Announcement{}, false
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil || !found {
		return messaging.Announcement{}, false
	}

	key, _ := convention.IssueKey(branch.Name, s.config().Jira.Project)

	return messaging.Announcement{
		Author:           s.author(),
		PullRequestURL:   pull.URL,
		PullRequestTitle: pull.Title,
		IssueKey:         key,
		IssueSummary:     s.issueSummary(jira.Key(key)),
		IssueURL:         s.issueURL(jira.Key(key)),
		Noun:             noun(s.info.ForgeKind),
		Moment:           s.announceMoment(pull, branch.Head),
		Kind:             s.config().Messaging.Kind,
		Template:         s.config().Messaging.Announcement,
	}, true
}

// announceMoment is the moment the pull request is at: merged, its CI red, or —
// the common case — open and ready for review. A CI read that is unavailable or
// fails leaves the moment at ready rather than failing the announcement.
func (s *server) announceMoment(pull forge.PullRequest, head string) messaging.Moment {
	if pull.State == forge.StateMerged {
		return messaging.MomentMerged
	}

	if s.deps.CheckCI == nil {
		return messaging.MomentReady
	}

	ci, err := s.deps.CheckCI(pull, head)
	if err == nil && ci.State == forge.CIFailed {
		return messaging.MomentCIRed
	}

	return messaging.MomentReady
}

// issueSummary is the branch issue's summary, or empty when the tracker cannot
// say — the announcement reads without it.
func (s *server) issueSummary(key jira.Key) string {
	if s.deps.Issue == nil || key == "" {
		return ""
	}

	detail, err := s.deps.Issue(key)
	if err != nil {
		return ""
	}

	return detail.Issue.Summary
}

// issueURL is a link to the issue, or empty when there is no way to build one.
func (s *server) issueURL(key jira.Key) string {
	if s.deps.BrowseURL == nil || key == "" {
		return ""
	}

	return s.deps.BrowseURL(key)
}

// noun is what the forge calls a change, from its resolved kind, for the
// announcement — "merge request" on GitLab, "pull request" everywhere else.
func noun(kind forge.Kind) string {
	if kind == forge.KindGitLab {
		return "merge request"
	}

	return "pull request"
}

// announcementDTO maps the composed announcement and its channel onto the wire.
func announcementDTO(announcement messaging.Announcement, channel string) api.Announcement {
	return api.Announcement{Text: announcement.Text(), Channel: channel}
}

// announceUnprocessable is the 422 response for an announcement the server will
// not post.
func announceUnprocessable(message string) api.Announce422ApplicationProblemPlusJSONResponse {
	return api.Announce422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, message))
}
