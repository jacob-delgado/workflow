// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// readPullRequestStarts is the command that reads, for the composer opened as
// opened, the repository's templates and the remote branches its base can
// complete to: files and git, read off the update loop. The composer's form
// waits for them, so nothing they read changes a field under the cursor.
func readPullRequestStarts(ctx context.Context, deps Deps, opened int) tea.Cmd {
	templates, remoteBranches := deps.Forge.Templates, deps.Git.RemoteBranches
	if templates == nil && remoteBranches == nil {
		return nil
	}

	return func() tea.Msg {
		read := pullRequestStartsRead{opened: opened, templatesRead: templates != nil}
		if templates != nil {
			read.templates = templates()
		}

		if remoteBranches != nil {
			// A failure to list is no reason to refuse the composer: its base
			// is simply left without completions.
			read.branches, _ = remoteBranches(ctx)
		}

		return read
	}
}

// pullRequestStartsRead is the templates and the remote branches read for the
// pull request composer opened as opened.
type pullRequestStartsRead struct {
	opened        int
	templates     []forge.Template
	templatesRead bool
	branches      []string
}

var _ applier = pullRequestStartsRead{}

// apply offers the remote branches as the base's completions, and the
// templates: the body is proposed again from the first while it is still the
// one proposed. Nothing changes once the composer they were read for has
// closed, even if another has opened since.
func (read pullRequestStartsRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened {
		return m, nil
	}

	composer.awaiting.starts, composer.branches = false, read.branches

	if read.templatesRead {
		composer.templates, composer.templatesRead = read.templates, true
		composer.answers.template = min(composer.answers.template, max(0, len(read.templates)-1))
	}

	if read.templatesRead && !composer.edited {
		composer = composer.withTemplate(composer.answers.template)
	}

	return composer.settled(m)
}

// readTitleIssue reads the composer's issue for its title, when the title is to
// come from the issue. A failed read leaves the summary empty, so the title
// stays the one proposed, as the command line's does. The tracker is asked under
// ctx.
func readTitleIssue(ctx context.Context, deps Deps, composer prComposer) tea.Cmd {
	read, from := deps.Jira.Issue, composer.proposedFrom
	if from.TitleSource != convention.TitleFromIssue || from.IssueKey == "" || read == nil {
		return nil
	}

	opened, proposed := composer.opened, composer.answers.title

	return func() tea.Msg {
		from.IssueSummary = ""

		detail, err := read(ctx, from.IssueKey)
		if err == nil {
			from.IssueSummary = detail.Issue.Summary
		}

		fromIssue, _ := loop.Draft(from)

		return titleIssueRead{opened: opened, proposed: proposed, fromIssue: fromIssue}
	}
}

// titleIssueRead is the title the branch's issue gives the pull request, read
// for the composer opened as opened, which proposed the title without it.
type titleIssueRead struct {
	opened              int
	proposed, fromIssue string
}

var _ applier = titleIssueRead{}

// apply puts the issue's title in the composer while its title is still the
// one proposed, which a kept draft's may not be. Nothing changes once the
// composer it was read for has closed.
func (read titleIssueRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened {
		return m, nil
	}

	composer.awaiting.title = false
	if composer.answers.title == read.proposed {
		composer.answers.title = read.fromIssue
	}

	return composer.settled(m)
}

// readReviewers reads the code owners of the branch's changes against the
// composer's base, to propose them as its reviewers. The read diffs and reads
// git, and asks the forge who the author is, off the update loop; the
// composer's form waits for it, as it does for the other reads.
func readReviewers(ctx context.Context, deps Deps, composer prComposer) tea.Cmd {
	owners := loop.OwnerSeams{
		ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
	}
	if owners.ChangedPaths == nil || owners.CodeOwnersAt == nil {
		return nil
	}

	opened, base := composer.opened, composer.draftedOnto

	return func() tea.Msg {
		return reviewersRead{opened: opened, proposed: loop.ProposedReviewers(ctx, owners, base)}
	}
}

// reviewersRead is who the code owners of the branch's changes propose as the
// pull request's reviewers, read for the composer opened as opened.
type reviewersRead struct {
	opened   int
	proposed []string
}

var _ applier = reviewersRead{}

// apply fills the composer's reviewers with the owners, unless a kept draft
// already says who reviews. Nothing changes once the composer they were read
// for has closed, even if another has opened since.
func (read reviewersRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened {
		return m, nil
	}

	composer.awaiting.reviewers = false
	if !composer.reviewersSettled {
		composer.reviewersProposed = strings.Join(read.proposed, ", ")
		composer.answers.reviewers, composer.reviewersSettled = composer.reviewersProposed, true
	}

	return composer.settled(m)
}

// browseURL links an issue, when there is an issue and a way to link it.
func browseURL(deps Deps, issueKey jira.Key) string {
	if issueKey == "" || deps.Jira.BrowseURL == nil {
		return ""
	}

	return deps.Jira.BrowseURL(issueKey)
}
