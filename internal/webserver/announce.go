// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetAnnouncement composes the announcement for the checked-out branch's pull
// request without posting it, for a preview. It is a 409 when there is no pull
// request to announce.
func (s *server) GetAnnouncement(
	_ context.Context, _ api.GetAnnouncementRequestObject,
) (api.GetAnnouncementResponseObject, error) {
	announcement, ok := s.announcement()
	if !ok {
		return api.GetAnnouncement409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	return api.GetAnnouncement200JSONResponse(announcementDTO(announcement, s.config().Messaging.Channel)), nil
}

// Announce posts the composed announcement to the configured service — to the
// requested channel, or the configured one when none is given. It is a 409 when
// there is no pull request to announce; a post that fails is classified by
// fault, whose details never carry the error's own text, which can name the
// webhook.
func (s *server) Announce(_ context.Context, request api.AnnounceRequestObject) (api.AnnounceResponseObject, error) {
	if request.Body == nil {
		return announceUnprocessable("a request body is required"), nil
	}

	if s.deps.Post == nil {
		return announceUnprocessable("announcing is not available"), nil
	}

	announcement, ok := s.announcement()
	if !ok {
		return api.Announce409ApplicationProblemPlusJSONResponse(s.nothingToAnnounce()), nil
	}

	channel := request.Body.Channel
	if channel == "" {
		channel = s.config().Messaging.Channel
	}

	err := s.deps.Post(channel, announcement.Text())
	if err != nil {
		body, code := fault(err)

		return api.AnnouncedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.Announce200JSONResponse(announcementDTO(announcement, channel)), nil
}

// announcement is the announcement for the checked-out branch's pull request,
// composed from the pull request, the branch's issue, and the configured
// template. It reports false when there is no pull request to announce, or the
// branch or the pull request cannot be read — each answered with the same 409.
func (s *server) announcement() (messaging.Announcement, bool) {
	cfg := s.config()

	announcement, _, err := loop.ComposeAnnouncement(loop.AnnounceSeams{
		Branch:    s.deps.Branch,
		FindPull:  s.deps.FindPull,
		Author:    s.deps.Author,
		Issue:     s.deps.Issue,
		BrowseURL: s.deps.BrowseURL,
		CheckCI:   s.deps.CheckCI,
	}, cfg.Messaging, cfg.Jira.Project, s.info.ForgeKind)

	return announcement, err == nil
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

// nothingToAnnounce refuses an announcement with no pull request to announce,
// named in the forge's own noun.
func (s *server) nothingToAnnounce() api.Problem {
	return problem(api.Conflict, "there is no "+s.noun()+" to announce")
}
