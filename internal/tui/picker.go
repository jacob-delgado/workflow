// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// doneGlyph marks something finished. Like failedGlyph, it says so by shape.
const doneGlyph = "●"

// pickerTitle titles the detail pane while the picker is open.
const pickerTitle = "Change status"

// outcomeRows is the room kept under the list for how applying is going.
const outcomeRows = 2

// transitionsListed carries Jira's transitions for one issue back into the update
// loop.
type transitionsListed struct {
	issueKey string
	found    []jira.Transition
	err      error
}

// transitionApplied reports how applying a transition went.
type transitionApplied struct {
	issueKey string
	to       jira.Transition
	err      error
}

// statusPicker is the change-status picker: the transitions Jira offers the issue
// it was opened on, and how applying one is going.
//
// The zero value is closed. An open picker always names its issue, which is what
// lets a late listing for some other issue be recognized and dropped.
type statusPicker struct {
	open     bool
	issue    jira.Issue
	found    []jira.Transition
	listErr  error
	applyErr error
	settled  bool
	sending  bool
	selected int
}

// settle records a listing, unless it answers a question this picker did not ask
// — another issue's, or its own asked twice by closing and reopening.
func (p statusPicker) settle(listing transitionsListed) statusPicker {
	if p.settled || listing.issueKey != p.issue.Key {
		return p
	}

	p.found, p.listErr, p.settled = listing.found, listing.err, true

	return p
}

// move shifts the selection, stopping at either end.
func (p statusPicker) move(step int) statusPicker {
	p.selected = max(0, min(p.selected+step, len(p.found)-1))

	return p
}

// chosen is the selected transition, if there is one to apply.
func (p statusPicker) chosen() (jira.Transition, bool) {
	if len(p.found) == 0 {
		return jira.Transition{}, false
	}

	return p.found[p.selected], true
}

// render draws the picker in as many rows as fit.
func (p statusPicker) render(rows int) string {
	lines := []string{p.issue.Key + " " + p.issue.Summary, "status  " + p.issue.Status, ""}

	switch {
	case !p.settled:
		lines = append(lines, "loading transitions…")
	case p.listErr != nil:
		lines = append(lines, failedGlyph+" "+p.listErr.Error())
	case len(p.found) == 0:
		lines = append(lines, "Jira offers no status change for "+p.issue.Key)
	default:
		lines = append(lines, p.rows(rows-len(lines)-outcomeRows)...)
		lines = append(lines, p.outcome()...)
	}

	return strings.Join(lines, "\n")
}

// rows draws as many transitions as fit, scrolled so the selection stays on
// screen. At least one always shows, so a cramped terminal still says which.
func (p statusPicker) rows(space int) []string {
	space = max(1, space)
	first := max(0, p.selected-space+1)
	last := min(len(p.found), first+space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		marker := "  "
		if index == p.selected {
			marker = "▸ "
		}

		move := p.found[index]
		lines = append(lines, marker+statusGlyph(move.ToStatusCategory)+" "+transitionLabel(move))
	}

	return lines
}

// outcome says how applying the chosen transition is going, if it was tried.
func (p statusPicker) outcome() []string {
	switch {
	case p.sending:
		chosen, _ := p.chosen()

		return []string{"", "moving " + p.issue.Key + " to " + chosen.ToStatus + "…"}
	case p.applyErr != nil:
		return []string{"", failedGlyph + " " + p.applyErr.Error()}
	default:
		return nil
	}
}

// transitionLabel names a transition by its verb and where it leads. A
// transition named for its own status says it once.
func transitionLabel(move jira.Transition) string {
	if move.Name == move.ToStatus {
		return move.Name
	}

	return move.Name + " → " + move.ToStatus
}

// openStatusPicker opens the picker on the selected issue and starts listing its
// transitions.
func (m Model) openStatusPicker() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok {
		return m, nil
	}

	m.picker = statusPicker{open: true, issue: selected}
	list := m.deps.ListTransitions

	return m, func() tea.Msg {
		found, err := list(selected.Key)

		return transitionsListed{issueKey: selected.Key, found: found, err: err}
	}
}

// handlePickerKey answers a key while the picker has the keyboard.
func (m Model) handlePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case m.picker.sending:
		// Closing now would hide the answer, and a refused change must never go
		// unseen. The request carries a deadline, so this cannot last.
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.picker = statusPicker{}
	case key.Matches(msg, m.keys.down):
		m.picker = m.picker.move(1)
	case key.Matches(msg, m.keys.up):
		m.picker = m.picker.move(-1)
	case key.Matches(msg, m.keys.confirm):
		return m.applyChosen()
	}

	return m, nil
}

// applyChosen sends the selected transition.
func (m Model) applyChosen() (Model, tea.Cmd) {
	chosen, ok := m.picker.chosen()
	if !ok {
		return m, nil
	}

	m.picker.sending, m.picker.applyErr = true, nil
	apply, issueKey := m.deps.ApplyTransition, m.picker.issue.Key

	return m, func() tea.Msg {
		return transitionApplied{issueKey: issueKey, to: chosen, err: apply(issueKey, chosen)}
	}
}

// finishTransition reports a transition's outcome. A refusal keeps the picker
// open with Jira's reason, to choose again; a move that worked closes it and
// refreshes the list, since the issue's status — and perhaps its place in the
// list — just changed.
func (m Model) finishTransition(result transitionApplied) (Model, tea.Cmd) {
	if result.err != nil {
		m.picker.sending, m.picker.applyErr = false, result.err

		return m, nil
	}

	m.picker = statusPicker{}
	m.notice = doneGlyph + " " + result.issueKey + " moved to " + result.to.ToStatus

	return m, m.searchIssues()
}
