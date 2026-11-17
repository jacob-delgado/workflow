// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// request is the pull request the composer describes. A reviewer named
// org/team is a team, which the forge is asked for apart from the people.
func (c prComposer) request() forge.NewPullRequest {
	users, teams := loop.SplitReviewers(splitList(c.answers.reviewers))

	return forge.NewPullRequest{
		Title: strings.TrimSpace(c.answers.title), Body: c.body, Head: forge.Head{Branch: c.head},
		Base: strings.TrimSpace(c.answers.base), Draft: c.answers.draft,
		Reviewers: users, TeamReviewers: teams,
		Assignees: splitList(c.answers.assignees),
		Labels:    splitList(c.answers.labels),
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

// open opens the pull request — pushing the branch first when its push remote
// does not have every commit, and opening only if the push succeeded.
func (c prComposer) open(m Model) (Model, tea.Cmd) {
	request := c.request()

	switch {
	case request.Title == "":
		c.problem, c.send = errNoTitle, c.send.Cleared()
	case request.Base == "":
		c.problem, c.send = errNoBase, c.send.Cleared()
	case m.dryRun:
		return m.closeOverlay().noticed(c.dryRunNotice(request, m.branch.checkedOut().Pushed(), m.pullOffers())), nil
	case m.branch.checkedOut().Pushed():
		return c.create(m)
	default:
		m.prDrafts = m.prDrafts.keeping(c.snapshot())

		return m.startPushThen(func(pushed Model) (Model, tea.Cmd) {
			pushed, reload := pushed.readingBranch()
			pushed, create := c.create(pushed)

			return pushed, tea.Batch(reload, create)
		})
	}

	m.overlay = c

	return m, nil
}

// dryRunNotice says what opening the pull request would do, including the push
// that enter does first when its push remote does not have every commit, and
// the offers the open goes on to make: offers, not writes, as the live path
// makes them.
func (c prComposer) dryRunNotice(request forge.NewPullRequest, pushed bool, offers []string) string {
	open := "open \"" + request.Title + "\" from " + request.Head.Branch + " into " + request.Base
	if len(offers) > 0 {
		open += ", then offer " + strings.Join(offers, " and ")
	}

	if pushed {
		return "dry run: would " + open
	}

	return "dry run: would push " + request.Head.Branch + ", then " + open
}

// create asks the forge to open the pull request, put back on top as it
// is sent, after a push among them, so its answer finds it by the round it
// opened in.
func (c prComposer) create(m Model) (Model, tea.Cmd) {
	c.problem = nil
	c.send, _ = c.send.Sent()
	m.overlay = c
	ctx, create, request, round := m.session, m.deps.Forge.CreatePullRequest, c.request(), c.opened

	return m, func() tea.Msg {
		created, err := create(ctx, request)

		return pullCreated{opened: round, head: request.Head.Branch, created: created, err: err}
	}
}

// pullCreated reports how opening a pull request went, for the composer
// opened in round opened: err is why it did not open, and created the pull
// request opened, with what fell short of the request once it did.
type pullCreated struct {
	opened int
	// head is the branch it was opened from, whose kept draft it drops.
	head    string
	created forge.Created
	err     error
}

var _ applier = pullCreated{}

// apply shows the new pull request and starts on its CI, or keeps the composer
// open with the forge's reason, pinned until a key its form takes. A pull that
// opened but whose reviewers could not be added is shown all the same, with a
// note, rather than lost. A late answer, its composer no longer the one open,
// changes nothing; none arrives, since the composer takes no key but ctrl+c,
// which quits, while its create is out.
func (msg pullCreated) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[prComposer](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	pull := msg.created.Pull

	opened := m.marks.done + " opened " + m.vocab.sigil + strconv.Itoa(pull.Number) + " " + pull.URL
	if msg.created.Shortfall != nil {
		opened += "; could not add every reviewer, assignee or label: " + briefly(msg.created.Shortfall)
	}

	m = m.beginReview(m.review.find.Told(pullFind{pull: pull, found: true, madeAfter: nil}))
	m.review.opened = m.deps.now()
	m.prDrafts = m.prDrafts.without(msg.head)

	m, armed, readHold := m.armOnOpen(pull)
	m = m.noticed(opened + armed)

	// The pull request opened is told the review, superseding a find a
	// Review refresh began before it opened, so only this one's CI read is
	// left for the refresh's mark to wait on.
	m, check := m.checkCI()
	cmds := tea.Batch(check, m.messaging.loadAuthor(m.session, m.deps), readHold)

	// With a Jira issue, offer to link the pull request on it — so the team that
	// watches Jira learns of it — and then to move it to the review status. When
	// Jira cannot take the link, go straight to the status offer. Noting it on the
	// issue's task is offered once those are answered.
	issueKey, named := m.branch.jiraIssue(m.cfg.Jira.Project)
	if named {
		m.followUp = offeredOnOpening(func(m Model) waitingOffer {
			return m.offerAnnotate(issueKey, pull)
		})
	}

	switch {
	case named && m.deps.Jira.LinkPullRequest != nil:
		return m.openIssueLinker(issueKey, pull), cmds
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
	branch, title, base, body              string
	reviewers, assignees, labels           string
	template                               int
	draft, edited, typed, reviewersSettled bool
	// draftedOnto, subjects and reviewersProposed are what the composer
	// proposed from, so a draft reopened onto a base chosen in it proposes
	// from that base still.
	draftedOnto, reviewersProposed string
	subjects                       []string
}

// prDrafts are the pull request drafts kept for the session, at most one per
// branch, so leaving the composer on one branch never touches another
// branch's draft.
type prDrafts []prDraft

// on is the draft kept for a branch, and whether one is.
func (d prDrafts) on(branch string) (prDraft, bool) {
	index := slices.IndexFunc(d, func(draft prDraft) bool { return draft.branch == branch })
	if index < 0 {
		return prDraft{}, false
	}

	return d[index], true
}

// keeping is the drafts with the one for draft's branch replaced by it. It
// copies rather than change the drafts it was called on.
func (d prDrafts) keeping(draft prDraft) prDrafts {
	return append(d.without(draft.branch), draft)
}

// without is the drafts with a branch's dropped, copied rather than changed.
func (d prDrafts) without(branch string) prDrafts {
	return slices.DeleteFunc(slices.Clone(d), func(draft prDraft) bool { return draft.branch == branch })
}

// written reports whether any draft holds what was typed or edited, more
// than the composer proposed.
func (d prDrafts) written() bool {
	return slices.ContainsFunc(d, func(draft prDraft) bool { return draft.edited || draft.typed })
}

// snapshot is the composer's editable state, to reopen on.
func (c prComposer) snapshot() prDraft {
	return prDraft{
		branch: c.head, title: c.answers.title, base: c.answers.base, body: c.body,
		reviewers: c.answers.reviewers, assignees: c.answers.assignees, labels: c.answers.labels,
		template: c.answers.template, draft: c.answers.draft, edited: c.edited, typed: c.typed,
		reviewersSettled: c.reviewersSettled, draftedOnto: c.draftedOnto, reviewersProposed: c.reviewersProposed,
		subjects: c.proposedFrom.Subjects,
	}
}

// restore fills the composer from a kept draft.
func (c prComposer) restore(draft prDraft) prComposer {
	c.answers = &prAnswers{
		title: draft.title, base: draft.base, reviewers: draft.reviewers, assignees: draft.assignees,
		labels: draft.labels, template: draft.template, draft: draft.draft,
	}
	c.body, c.edited, c.typed = draft.body, draft.edited, draft.typed
	c.reviewersSettled = draft.reviewersSettled
	c.draftedOnto, c.reviewersProposed = draft.draftedOnto, draft.reviewersProposed
	c.proposedFrom.Subjects = draft.subjects

	return c
}
