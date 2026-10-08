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
	paneSummary
	paneRepositories
)

// paneFresh is how long after a pane began loading a switch to it leaves it as
// it is: long enough that flicking between panes spends no requests, short
// enough that a pane come back to after a while is current.
const paneFresh = 30 * time.Second

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 9

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false. The messaging pane is named
// for the service in use — Slack, Teams, Discord or Webhook.
func (p pane) title(messaging string) string {
	titles := [paneCount]string{
		"Issues", "Branch", "Commits", "Review", messaging, "Reviews", "Tasks", "Summary", "Repositories",
	}

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
	// loading reports a refresh, or the pane's first read, begun and not yet
	// answered, which the pane's title marks in flight.
	loading func(m Model) bool
	// move steps the cursor of the pane's list delta rows, down for a positive
	// delta, stopping at either end; nil for a pane with no list, whose ends
	// are its detail's.
	move func(m Model, delta int) (Model, tea.Cmd)
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
	// readsBranch marks a pane whose refresh reads the branch again, which
	// goes on to find its pull request and read CI: one such refresh leaves
	// every pane it feeds fresh.
	readsBranch bool
}

// behaviorOf is a pane's behavior.
func behaviorOf(target pane) behavior {
	return map[pane]behavior{
		paneIssues: {
			rail: Model.issuesRail, detail: Model.issueDetailView, narrow: Model.issuesNarrow,
			keys: Model.issuesKeys, handle: Model.handleIssuesKey, pick: Model.pickIssue, move: Model.moveIssue,
			refresh: Model.refreshIssues, loading: func(m Model) bool { return m.issues.loading },
			scroll: func(m *Model) *int { return &m.detail.scroll },
		},
		paneBranch: {
			rail: Model.branchRail, detail: Model.branchDetail, narrow: nil,
			keys: Model.branchKeys, handle: Model.handleBranchKey, pick: nil,
			refresh: Model.refreshBranch, loading: func(m Model) bool { return m.branch.loading },
			scroll: func(m *Model) *int { return &m.branch.scroll }, readsBranch: true,
		},
		paneCommits: {
			rail: Model.commitsRail, detail: Model.commitsDetail, narrow: nil,
			keys: Model.commitsKeys, handle: Model.handleCommitsKey, pick: Model.pickChange, move: Model.moveChangeBy,
			refresh: Model.refreshCommits, loading: func(m Model) bool { return m.changes.loading },
			scroll: func(m *Model) *int { return &m.changes.scroll }, listInDetail: true, readsBranch: true,
		},
		paneReview: {
			rail: Model.reviewRail, detail: Model.reviewDetail, narrow: nil,
			keys: Model.reviewKeys, handle: Model.handleReviewKey, pick: nil,
			refresh: Model.refreshReview, loading: func(m Model) bool { return m.review.loading },
			scroll: func(m *Model) *int { return &m.review.scroll }, readsBranch: true,
		},
		paneMessaging: {
			rail: Model.messagingRail, detail: Model.messagingDetail, narrow: nil,
			keys: Model.messagingKeys, handle: Model.handleMessagingKey, pick: nil,
			refresh: Model.refreshMessaging, loading: func(m Model) bool { return m.messaging.loading },
			scroll: func(m *Model) *int { return &m.messaging.scroll }, readsBranch: true,
		},
		paneReviews: {
			rail: Model.reviewQueueRail, detail: Model.reviewQueueDetail, narrow: nil,
			keys: Model.reviewQueueKeys, handle: Model.handleReviewQueueKey, pick: Model.pickReview,
			move: commandless(Model.moveReviewBy), refresh: Model.refreshReviewQueue,
			loading: func(m Model) bool { return m.reviewQueue.loading },
			scroll:  func(m *Model) *int { return &m.reviewQueue.scroll }, listInDetail: true,
		},
		paneTasks: {
			rail: Model.tasksRail, detail: Model.tasksDetail, narrow: nil,
			keys: Model.tasksKeys, handle: Model.handleTasksKey, pick: Model.pickTask, move: commandless(Model.moveTaskBy),
			refresh: Model.refreshTasks, loading: func(m Model) bool { return m.tasks.loading },
			scroll: func(m *Model) *int { return &m.tasks.scroll }, listInDetail: true,
		},
		paneSummary: {
			rail: Model.summaryRail, detail: Model.summaryDetail, narrow: nil,
			keys: Model.summaryKeys, handle: Model.handleSummaryKey, pick: nil, move: commandless(Model.moveSummaryBy),
			refresh: Model.refreshSummary, loading: Model.summaryLoading,
			scroll: func(m *Model) *int { return &m.summary.scroll }, listInDetail: true,
		},
		paneRepositories: {
			rail: Model.repositoriesRail, detail: Model.repositoriesDetail, narrow: nil,
			keys: Model.repositoriesKeys, handle: Model.handleRepositoriesKey, pick: nil,
			move: commandless(Model.moveRepositoryBy), refresh: Model.refreshRepositories,
			loading: func(m Model) bool { return m.repositories.loading },
			scroll:  func(m *Model) *int { return &m.repositories.scroll }, listInDetail: true,
		},
	}[target]
}

// commandless is a list's move that asks for nothing once it has moved, as a
// pane's move, which may.
func commandless(move func(Model, int) Model) func(Model, int) (Model, tea.Cmd) {
	return func(m Model, delta int) (Model, tea.Cmd) { return move(m, delta), nil }
}

// refreshPane loads target again, noting when, whatever its age, for it and
// for every pane its load also refreshed.
func (m Model) refreshPane(target pane) (Model, tea.Cmd) {
	for _, loaded := range alsoRefreshed(target) {
		m.refreshed[loaded] = m.deps.now()
	}

	return behaviorOf(target).refresh(m)
}

// alsoRefreshed is target and the panes a refresh of it loads with it: every
// pane fed by the branch read, when target reads the branch.
func alsoRefreshed(target pane) []pane {
	if !behaviorOf(target).readsBranch {
		return []pane{target}
	}

	var fed []pane

	for candidate := range pane(paneCount) {
		if behaviorOf(candidate).readsBranch {
			fed = append(fed, candidate)
		}
	}

	return fed
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

// offer is one of a pane's keys: the binding its footer names, whether it acts
// right now, what it does when it does, and, for a key that explains itself,
// why it cannot when it cannot. A pane lists its offers once, and its footer
// and its key handler both read that list, so a key the footer leaves out
// never acts.
type offer struct {
	binding key.Binding
	can     bool
	act     func() (Model, tea.Cmd)
	refusal error
}

// liveKeys is the bindings of the offers that act right now, in order, for a
// footer.
func liveKeys(offers []offer) []key.Binding {
	var keys []key.Binding

	for _, each := range offers {
		if each.can {
			keys = append(keys, each.binding)
		}
	}

	return keys
}

// answer acts on the first offer msg presses that acts right now, or, when
// none does, says why the first it presses that explains itself cannot. A key
// no offer takes does nothing.
func (m Model) answer(offers []offer, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	for _, each := range offers {
		if each.can && key.Matches(msg, each.binding) {
			return each.act()
		}
	}

	for _, each := range offers {
		if each.refusal != nil && key.Matches(msg, each.binding) {
			return m.noticedGuidance(each.refusal), nil
		}
	}

	return m, nil
}
