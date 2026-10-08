// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// request is the pull request the composer describes. A reviewer named
// org/team is a team, which the forge is asked for apart from the people.
func (c prComposer) request() forge.NewPullRequest {
	users, teams := loop.SplitReviewers(splitList(c.reviewers.Value()))

	return forge.NewPullRequest{
		Title: strings.TrimSpace(c.title.Value()), Body: c.body, Head: c.head,
		Base: strings.TrimSpace(c.base.Value()), Draft: c.draft,
		Reviewers: users, TeamReviewers: teams,
		Assignees: splitList(c.assignees.Value()),
		Labels:    splitList(c.labels.Value()),
	}
}

// splitList reads a comma-separated field into its trimmed, non-empty entries,
// or nil when nothing was typed.
func splitList(text string) []string {
	var entries []string

	for entry := range strings.SplitSeq(text, ",") {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			entries = append(entries, trimmed)
		}
	}

	return entries
}

// open opens the pull request — pushing the branch first when origin does not
// have every commit, and opening only if the push succeeded.
func (c prComposer) open(m Model) (Model, tea.Cmd) {
	request := c.request()

	switch {
	case request.Title == "":
		c.send.err = errNoTitle
	case request.Base == "":
		c.send.err = errNoBase
	case m.dryRun:
		return m.closeOverlay().noticed(c.dryRunNotice(request, m.branch.branch.Pushed(), m.pullOffers())), nil
	case m.branch.branch.Pushed():
		return c.create(m)
	default:
		m.prDraft = c.snapshot()

		return m.startPush(func(pushed Model) (Model, tea.Cmd) {
			reload := loadBranch(pushed.deps)
			pushed, create := c.create(pushed)

			return pushed, tea.Batch(reload, create)
		})
	}

	m.overlay = c

	return m, nil
}

// dryRunNotice says what opening the pull request would do, including the push
// that enter does first when origin does not have every commit, and the offers
// the open goes on to make: offers, not writes, as the live path makes them.
func (c prComposer) dryRunNotice(request forge.NewPullRequest, pushed bool, offers []string) string {
	open := "open \"" + request.Title + "\" from " + request.Head + " into " + request.Base
	if len(offers) > 0 {
		open += ", then offer " + strings.Join(offers, " and ")
	}

	if pushed {
		return "dry run: would " + open
	}

	return "dry run: would push " + request.Head + ", then " + open
}

// create asks the forge to open the pull request.
func (c prComposer) create(m Model) (Model, tea.Cmd) {
	c.send = starting()
	m.overlay = c
	create, request := m.deps.Forge.CreatePullRequest, c.request()

	return m, func() tea.Msg {
		pull, err := create(request)

		return pullCreated{pull: pull, err: err}
	}
}

// pullCreated reports how opening a pull request went.
type pullCreated struct {
	pull forge.PullRequest
	err  error
}

var _ applier = pullCreated{}

// apply shows the new pull request and starts on its CI, or keeps the composer
// open with the forge's reason. A pull that opened but whose reviewers could
// not be added is shown all the same, with a note, rather than lost.
func (msg pullCreated) apply(m Model) (Model, tea.Cmd) {
	refusal := msg.err
	if refusal != nil && !msg.pull.Opened() {
		return keepOpenWith[prComposer](m, refusal), nil
	}

	notice := m.marks.done + " opened " + m.vocab.sigil + strconv.Itoa(msg.pull.Number) + " " + msg.pull.URL
	if refusal != nil {
		notice += "; could not add every reviewer, assignee or label: " + briefly(refusal)
	}

	m = m.noticed(notice)
	m = m.beginReview(reviewState{pull: msg.pull, found: true, loaded: true})
	m.prDraft = prDraft{}

	cmds := tea.Batch(m.checkCI(), m.loadAuthor())

	// With a Jira issue, offer to link the pull request on it — so the team that
	// watches Jira learns of it — and then to move it to the review status. When
	// Jira cannot take the link, go straight to the status offer. Noting it on the
	// issue's task is offered once those are answered.
	issueKey, named := m.jiraIssue()
	if named {
		m.followUp = m.offerAnnotate(issueKey, msg.pull)
	}

	switch {
	case named && m.deps.Jira.LinkPullRequest != nil:
		m.overlay = issueLinker{vocab: m.vocab, issueKey: issueKey, pull: msg.pull}

		return m, cmds
	case named:
		picker, offer := m.offerReviewStatus(issueKey)

		return picker, tea.Batch(cmds, offer)
	default:
		return m.closeOverlay(), cmds
	}
}

// prDraft is a pull request the composer was filled with, kept for the session
// so a push that fails, or an esc, does not throw the work away.
type prDraft struct {
	branch, title, base, body       string
	reviewers, assignees, labels    string
	template                        int
	draft, edited, reviewersSettled bool
}

// snapshot is the composer's editable state, to reopen on.
func (c prComposer) snapshot() prDraft {
	return prDraft{
		branch: c.head, title: c.title.Value(), base: c.base.Value(), body: c.body,
		reviewers: c.reviewers.Value(), assignees: c.assignees.Value(), labels: c.labels.Value(),
		template: c.template, draft: c.draft, edited: c.edited, reviewersSettled: c.reviewersSettled,
	}
}

// restore fills the composer from a kept draft.
func (c prComposer) restore(draft prDraft) prComposer {
	c.title.SetValue(draft.title)
	c.base.SetValue(draft.base)
	c.reviewers.SetValue(draft.reviewers)
	c.assignees.SetValue(draft.assignees)
	c.labels.SetValue(draft.labels)
	c.body, c.template, c.draft, c.edited = draft.body, draft.template, draft.draft, draft.edited
	c.reviewersSettled = draft.reviewersSettled

	return c
}
