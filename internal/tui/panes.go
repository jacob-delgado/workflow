// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"time"

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
	paneTasks
)

// paneFresh is how long after a pane began loading a switch to it leaves it as
// it is: long enough that flicking between panes spends no requests, short
// enough that a pane come back to after a while is current.
const paneFresh = 30 * time.Second

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 7

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false. The messaging pane is named
// for the service in use — Slack, Teams, Discord or Webhook.
func (p pane) title(messaging string) string {
	titles := [paneCount]string{"Issues", "Branch", "Commits", "Review", messaging, "Reviews", "Tasks"}

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
	// refresh loads the pane again, for r and for a switch to a stale pane.
	refresh func(m Model) (Model, tea.Cmd)
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
			keys: Model.issuesKeys, handle: Model.handleIssuesKey, pick: Model.pickIssue, refresh: Model.refreshIssues,
			scroll: func(m *Model) *int { return &m.detail.scroll },
		},
		paneBranch: {
			rail: Model.branchRail, detail: Model.branchDetail, narrow: nil,
			keys: Model.branchKeys, handle: Model.handleBranchKey, pick: nil,
			refresh: func(m Model) (Model, tea.Cmd) { return m, tea.Batch(m.loadBranch(), m.loadChanges()) },
			scroll:  func(m *Model) *int { return &m.branch.scroll },
		},
		paneCommits: {
			rail: Model.commitsRail, detail: Model.commitsDetail, narrow: nil,
			keys: Model.commitsKeys, handle: Model.handleCommitsKey, pick: Model.pickChange,
			refresh: func(m Model) (Model, tea.Cmd) {
				return m, tea.Batch(m.loadChanges(), m.loadBranch(), m.findHooks())
			},
			scroll: func(m *Model) *int { return &m.changes.scroll }, listInDetail: true,
		},
		paneReview: {
			rail: Model.reviewRail, detail: Model.reviewDetail, narrow: nil,
			keys: Model.reviewKeys, handle: Model.handleReviewKey, pick: nil,
			// The branch, once read, looks for its pull request, and the find reads
			// CI for the one it finds: so a refresh picks up a branch switched in a
			// shell, and never reads CI for a pull request since replaced.
			refresh: func(m Model) (Model, tea.Cmd) { return m, m.loadBranch() },
			scroll:  func(m *Model) *int { return &m.review.scroll },
		},
		paneMessaging: {
			rail: Model.messagingRail, detail: Model.messagingDetail, narrow: nil,
			keys: Model.messagingKeys, handle: Model.handleMessagingKey, pick: nil,
			// The announcement is written from the pull request and its CI, so
			// they are read again with what was announced.
			refresh: func(m Model) (Model, tea.Cmd) { return m, tea.Batch(m.loadAnnounces(), m.loadBranch()) },
			scroll:  func(m *Model) *int { return &m.messaging.scroll },
		},
		paneReviews: {
			rail: Model.reviewQueueRail, detail: Model.reviewQueueDetail, narrow: nil,
			keys: Model.reviewQueueKeys, handle: Model.handleReviewQueueKey, pick: Model.pickReview,
			refresh: func(m Model) (Model, tea.Cmd) { return m, m.loadReviewQueue() },
			scroll:  func(m *Model) *int { return &m.reviewQueue.scroll }, listInDetail: true,
		},
		paneTasks: {
			rail: Model.tasksRail, detail: Model.tasksDetail, narrow: nil,
			keys: Model.tasksKeys, handle: Model.handleTasksKey, pick: Model.pickTask,
			refresh: func(m Model) (Model, tea.Cmd) { return m, m.loadTasks() },
			scroll:  func(m *Model) *int { return &m.tasks.scroll }, listInDetail: true,
		},
	}[target]
}

// refreshPane loads target again, noting when, whatever its age.
func (m Model) refreshPane(target pane) (Model, tea.Cmd) {
	m.refreshed[target] = m.deps.now()

	return behaviorOf(target).refresh(m)
}

// switchTo moves focus to target, as a key or a click asks, loading it again
// when it is stale: it began loading more than paneFresh ago, and is not the
// Issues list holding further pages, which a reload of the first would drop.
func (m Model) switchTo(target pane) (Model, tea.Cmd) {
	if target == m.focus {
		return m, nil
	}

	m = m.focusOn(target)

	fresh := m.deps.now().Sub(m.refreshed[target]) < paneFresh
	if fresh || (target == paneIssues && m.issues.paged) {
		return m, nil
	}

	return m.refreshPane(target)
}
