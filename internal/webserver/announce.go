// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/slack"
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
		return api.GetAnnouncement409JSONResponse{Code: api.Conflict, Message: errNoPullRequest.Error()}, nil
	}

	return api.GetAnnouncement200JSONResponse(announcementDTO(announcement, s.config().Slack.Channel)), nil
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
		return api.Announce409JSONResponse{Code: api.Conflict, Message: errNoPullRequest.Error()}, nil
	}

	channel := request.Body.Channel
	if channel == "" {
		channel = s.config().Slack.Channel
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
func (s *server) composeAnnouncement() (slack.Announcement, bool) {
	if s.deps.Branch == nil || s.deps.FindPull == nil {
		return slack.Announcement{}, false
	}

	branch, err := s.deps.Branch()
	if err != nil {
		return slack.Announcement{}, false
	}

	pull, found, err := s.deps.FindPull(branch.Name)
	if err != nil || !found {
		return slack.Announcement{}, false
	}

	key, _ := convention.IssueKey(branch.Name, s.config().Jira.Project)

	return slack.Announcement{
		Author:           s.author(),
		PullRequestURL:   pull.URL,
		PullRequestTitle: pull.Title,
		IssueKey:         key,
		IssueSummary:     s.issueSummary(jira.Key(key)),
		IssueURL:         s.issueURL(jira.Key(key)),
		Noun:             noun(s.config().Forge.Kind),
		Template:         s.config().Slack.Announcement,
	}, true
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

// noun is what the forge calls a change, from its kind, for the announcement.
func noun(kind string) string {
	if kind == "gitlab" {
		return "merge request"
	}

	return "pull request"
}

// announcementDTO maps the composed announcement and its channel onto the wire.
func announcementDTO(announcement slack.Announcement, channel string) api.Announcement {
	return api.Announcement{Text: announcement.Text(), Channel: channel}
}

// announceUnprocessable is the 422 response for an announcement the server will
// not post.
func announceUnprocessable(message string) api.Announce422JSONResponse {
	return api.Announce422JSONResponse{Code: api.Unprocessable, Message: message}
}
