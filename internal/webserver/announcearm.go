// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"cmp"
	"context"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// draftNotArmed is why a draft just opened has no announcement armed.
const draftNotArmed = "a draft is not announced; announce it once it is ready"

// armOnOpen arms the announcement of pull, just opened from branch, to post
// once its CI passes, as asking to announce when CI passes holds one, and
// answers it as held, or why a draft was not armed. Nothing is armed, and
// nothing said, with messaging.auto_announce off in the configuration in
// effect, nothing to post with, for a pull request announced already, or
// while a held announcement is being posted. Whom it tags is read as workflow
// announce --yes proposes them, the linked owners of the changes against the
// base pull merges into and the groups that start checked, under ctx.
func (s *server) armOnOpen(
	ctx context.Context, branch gitrepo.Branch, pull forge.PullRequest,
) (*api.QueuedAnnouncement, *string) {
	cfg := s.config()
	if !cfg.Messaging.AutoAnnounce || cfg.Messaging.Mode() == config.MessagingNone || s.deps.Messaging.Post == nil {
		return nil, nil
	}

	if pull.Draft {
		return nil, new(draftNotArmed)
	}

	composed := loop.Composed{
		Announcement: loop.ComposeAnnouncementOf(ctx, loop.AnnounceSeams{
			Author: s.authorSeam(), Issue: s.deps.Jira.Issue, BrowseURL: s.deps.Jira.BrowseURL,
		}, branch, pull, cfg.Messaging, cfg.Jira.Project, s.forgeKindNow()),
		Pull: pull, Branch: branch,
	}
	if s.announcedAlready(ctx, composed.Made()) {
		return nil, nil
	}

	mentions := s.unaskedMentions(ctx, cmp.Or(pull.Base, branch.Base))
	post := announcePost{
		delivery: composed.Delivery(s.channelOr(""), composed.Announcement.Text(), mentions),
		memory:   loop.AnnounceMemory{Recorded: s.recordedAnnouncements, Record: s.recordAnnouncement},
		branch:   branch.Name, automatic: true,
	}

	queued, err := s.hold(post, branch.Name, ciReading{ci: forge.CI{}, read: false})
	if err != nil {
		return nil, nil
	}

	return &queued, nil
}

// unaskedMentions is whom an announcement armed with no preview to ask tags:
// the linked owners of the changes against base, the branch its pull request
// merges into, and the groups that start checked, read under ctx, or no one
// where the Slack directory cannot be read.
func (s *server) unaskedMentions(ctx context.Context, base string) messaging.Mentions {
	if !s.canReadDirectory(ctx, "") {
		return messaging.Mentions{}
	}

	workspace, err := loop.TagWorkspace(ctx, s.deps.Messaging.Workspace)
	if err != nil {
		return messaging.Mentions{}
	}

	owners := func(ctx context.Context) codeowners.Owners { return s.ownersAgainst(ctx, base) }
	tags, _ := s.proposedTags(ctx, messaging.MomentReady, workspace, owners)
	_, mentions := tags.Proposed()

	return mentions
}
