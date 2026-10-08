// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// readPullRequestStarts is the command that reads, for the composer opened as
// opened, the repository's templates and the remote branches its base can
// complete to: files and git, read off the update loop so the composer never
// waits on them.
func readPullRequestStarts(deps Deps, opened int) tea.Cmd {
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
			read.branches, _ = remoteBranches()
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
// one proposed, unless the composer they were read for has closed — even if
// another has opened since — or is being sent.
func (read pullRequestStartsRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened || composer.send.sending {
		return m, nil
	}

	composer = composer.withBaseSuggestions(read.branches)

	if read.templatesRead {
		composer.templates, composer.templatesRead = read.templates, true
		composer.template = min(composer.template, max(0, len(read.templates)-1))
	}

	if read.templatesRead && !composer.edited {
		composer = composer.withTemplate(composer.template)
	}

	m.overlay = composer

	return m, nil
}

// readTitleIssue reads the composer's issue for its title, when the title is to
// come from the issue. A failed read leaves the summary empty, so the title
// stays the one proposed, as the command line's does.
func readTitleIssue(deps Deps, composer prComposer) tea.Cmd {
	read, from := deps.Jira.Issue, composer.proposedFrom
	if from.TitleSource != convention.TitleFromIssue || from.IssueKey == "" || read == nil {
		return nil
	}

	head, proposed := composer.head, composer.title.Value()

	return func() tea.Msg {
		from.IssueSummary = ""

		detail, err := read(from.IssueKey)
		if err == nil {
			from.IssueSummary = detail.Issue.Summary
		}

		fromIssue, _ := loop.Draft(from)

		return titleIssueRead{head: head, proposed: proposed, fromIssue: fromIssue}
	}
}

// titleIssueRead is the title the branch's issue gives the pull request, read
// after its composer opened on the title proposed without it.
type titleIssueRead struct {
	head, proposed, fromIssue string
}

var _ applier = titleIssueRead{}

// apply puts the issue's title in the composer, unless the composer has closed,
// or is for another branch, or its title is no longer the one proposed: typed
// over, or already being sent.
func (read titleIssueRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.head != read.head || composer.send.sending || composer.title.Value() != read.proposed {
		return m, nil
	}

	composer.title.SetValue(read.fromIssue)
	m.overlay = composer

	return m, nil
}

// readReviewers reads the code owners of the branch's changes against the
// composer's base, to propose them as its reviewers. The read diffs and reads
// git, and asks the forge who the author is, so it does not hold the composer
// back.
func readReviewers(deps Deps, composer prComposer) tea.Cmd {
	owners := loop.OwnerSeams{
		ChangedPaths: deps.Git.ChangedPaths, CodeOwnersAt: deps.Git.CodeOwnersAt, Author: deps.Forge.Author,
	}
	if owners.ChangedPaths == nil || owners.CodeOwnersAt == nil {
		return nil
	}

	opened, base := composer.opened, composer.base.Value()

	return func() tea.Msg {
		return reviewersRead{opened: opened, proposed: loop.ProposedReviewers(owners, base)}
	}
}

// reviewersRead is who the code owners propose as the pull request's reviewers,
// read after its composer opened.
type reviewersRead struct {
	opened   int
	proposed []string
}

var _ applier = reviewersRead{}

// apply fills the composer's reviewers with the owners, unless the composer
// they were read for has closed — even if another has opened since — or is
// being sent, or reviewers were typed in the meantime.
func (read reviewersRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened || composer.send.sending || composer.reviewersSettled {
		return m, nil
	}

	composer.reviewers.SetValue(strings.Join(read.proposed, ", "))
	composer.reviewersSettled = true
	m.overlay = composer

	return m, nil
}

// withBaseSuggestions offers the remote branches as completions for the base
// field; with none listed the field is left plain.
func (c prComposer) withBaseSuggestions(branches []string) prComposer {
	if len(branches) == 0 {
		return c
	}

	c.base.SetSuggestions(branches)
	c.base.ShowSuggestions = true

	return c
}

// browseURL links an issue, when there is an issue and a way to link it.
func browseURL(deps Deps, issueKey jira.Key) string {
	if issueKey == "" || deps.Jira.BrowseURL == nil {
		return ""
	}

	return deps.Jira.BrowseURL(issueKey)
}
