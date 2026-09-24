// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// pane is one panel in the rail.
type pane int

const (
	paneIssues pane = iota
	paneBranch
	paneCommits
	paneReview
	paneMessaging
	paneReviews
)

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 6

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false. The messaging pane is named
// for the service in use — Slack, Teams, Discord or Webhook.
func (p pane) title(messaging string) string {
	titles := [paneCount]string{"Issues", "Branch", "Commits", "Review", messaging, "Reviews"}

	return titles[p]
}

// label is the title with the number that jumps to it.
func (p pane) label(messaging string) string {
	return strconv.Itoa(int(p)+1) + " " + p.title(messaging)
}

// paneNumbers are the digit keys that jump to each pane, "1" through the last,
// derived from paneCount so a new pane is reachable without a second edit.
func paneNumbers() []string {
	numbers := make([]string, paneCount)
	for index := range numbers {
		numbers[index] = strconv.Itoa(index + 1)
	}

	return numbers
}

// behavior is what one pane shows and does. Each pane is one entry in a table
// rather than a case in a switch every new pane would have to edit.
type behavior struct {
	// rail is the pane's content in the rail, in as many rows as fit.
	rail func(m Model, rows int) string
	// detail is everything the pane has to say in the detail pane, scrolled
	// and clipped by whatever draws it.
	detail func(m Model, width int) string
	// narrow is what the pane shows when the rail is gone and the detail is
	// the whole screen; nil means its detail.
	narrow func(m Model, rows int) string
	// keys is what the pane offers in the footer, right now.
	keys func(m Model) []key.Binding
	// handle answers a key the rest of the interface did not claim.
	handle func(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd)
	// pick selects the row of the pane's list drawn on a line; nil for a pane
	// with no list to pick from. inRail says which drawing was clicked.
	pick func(m Model, line, rows int, inRail bool) (Model, tea.Cmd)
	// scroll is where the pane keeps how far its detail is scrolled, on its own
	// state, so leaving the pane and coming back finds it where it was.
	scroll func(m *Model) *int
	// listInDetail marks a pane whose selectable list lives in the detail rather
	// than the rail, so the heavy focus border belongs on the detail, where the
	// cursor is, not on the rail's summary.
	listInDetail bool
}

// behaviorOf is a pane's behavior.
func behaviorOf(target pane) behavior {
	return map[pane]behavior{
		paneIssues: {
			rail: Model.issuesRail, detail: Model.issueDetailView, narrow: Model.issuesNarrow,
			keys: Model.issuesKeys, handle: Model.handleIssuesKey, pick: Model.pickIssue,
			scroll: func(m *Model) *int { return &m.detail.scroll },
		},
		paneBranch: {
			rail: Model.branchRail, detail: Model.branchDetail, narrow: nil,
			keys: Model.branchKeys, handle: Model.handleBranchKey, pick: nil,
			scroll: func(m *Model) *int { return &m.branch.scroll },
		},
		paneCommits: {
			rail: Model.commitsRail, detail: Model.commitsDetail, narrow: nil,
			keys: Model.commitsKeys, handle: Model.handleCommitsKey, pick: Model.pickChange,
			scroll: func(m *Model) *int { return &m.changes.scroll }, listInDetail: true,
		},
		paneReview: {
			rail: Model.reviewRail, detail: Model.reviewDetail, narrow: nil,
			keys: Model.reviewKeys, handle: Model.handleReviewKey, pick: nil,
			scroll: func(m *Model) *int { return &m.review.scroll },
		},
		paneMessaging: {
			rail: Model.messagingRail, detail: Model.messagingDetail, narrow: nil,
			keys: Model.messagingKeys, handle: Model.handleMessagingKey, pick: nil,
			scroll: func(m *Model) *int { return &m.messaging.scroll },
		},
		paneReviews: {
			rail: Model.reviewQueueRail, detail: Model.reviewQueueDetail, narrow: nil,
			keys: Model.reviewQueueKeys, handle: Model.handleReviewQueueKey, pick: Model.pickReview,
			scroll: func(m *Model) *int { return &m.reviewQueue.scroll }, listInDetail: true,
		},
	}[target]
}
