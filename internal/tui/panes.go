// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// pane is one panel in the rail.
type pane int

const (
	paneIssues pane = iota
	paneBranch
	paneCommits
	paneReview
	paneSlack
)

// paneCount is untyped on purpose: typed as pane, the exhaustive linter would
// count it as a member and demand a case for it in every switch.
const paneCount = 5

// title names a pane. A lookup rather than a switch, because a switch over every
// pane leaves a final arm that can never be false.
func (p pane) title() string {
	titles := [paneCount]string{"Issues", "Branch", "Commits", "Review", "Slack"}

	return titles[p]
}

// label is the title with the number that jumps to it.
func (p pane) label() string {
	return strconv.Itoa(int(p)+1) + " " + p.title()
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
	handle func(m Model, msg tea.KeyMsg) (Model, tea.Cmd)
	// pick selects the row of the pane's list drawn on a line; nil for a pane
	// with no list to pick from. inRail says which drawing was clicked.
	pick func(m Model, line, rows int, inRail bool) (Model, tea.Cmd)
}

// behaviorOf is a pane's behavior.
func behaviorOf(target pane) behavior {
	return map[pane]behavior{
		paneIssues: {
			rail: Model.issuesRail, detail: Model.issueDetailView, narrow: Model.issuesRail,
			keys: Model.issuesKeys, handle: Model.handleIssuesKey, pick: Model.pickIssue,
		},
		paneBranch: {
			rail: Model.branchRail, detail: Model.branchDetail, narrow: nil,
			keys: Model.branchKeys, handle: Model.handleBranchKey, pick: nil,
		},
		paneCommits: {
			rail: Model.commitsRail, detail: Model.commitsDetail, narrow: nil,
			keys: Model.commitsKeys, handle: Model.handleCommitsKey, pick: Model.pickChange,
		},
		paneReview: {
			rail: Model.reviewRail, detail: Model.reviewDetail, narrow: nil,
			keys: Model.reviewKeys, handle: Model.handleReviewKey, pick: nil,
		},
		paneSlack: {
			rail: Model.slackRail, detail: Model.slackDetail, narrow: nil,
			keys: Model.slackKeys, handle: Model.handleSlackKey, pick: nil,
		},
	}[target]
}
