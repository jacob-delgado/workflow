// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// pickerTitle titles the detail pane while the picker is open.
const pickerTitle = "Change status"

// pickerHeader is the rows above the picker's list: the issue, its status, and
// a blank line.
const pickerHeader = 3

// outcomeRows is the room kept under the list for how applying is going.
const outcomeRows = 2

// transitionsListed carries Jira's transitions for one issue back into the update
// loop.
type transitionsListed struct {
	issueKey string
	found    []jira.Transition
	err      error
}

// apply records a listing, unless it answers a question the open picker did not
// ask — another issue's, or its own asked twice by closing and reopening, which
// would replace a list someone is already choosing from.
func (msg transitionsListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(statusPicker)
	if !open || picker.settled || msg.issueKey != picker.issue.Key {
		return m, nil
	}

	picker.found, picker.listErr, picker.settled = msg.found, msg.err, true
	m.overlay = picker

	return m, nil
}

// transitionApplied reports how applying a transition went.
type transitionApplied struct {
	issueKey string
	to       jira.Transition
	err      error
}

// apply reports a transition's outcome. A refusal keeps the picker open with
// Jira's reason, to choose again; a move that worked closes it and refreshes the
// list, since the issue's status — and perhaps its place in the list — just
// changed.
func (msg transitionApplied) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		picker, open := m.overlay.(statusPicker)
		if open {
			picker.send, picker.form = picker.send.failed(msg.err), fieldForm{}
			m.overlay = picker
		}

		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + msg.issueKey + " is now " + msg.to.ToStatus)

	return m, tea.Batch(m.searchIssues(), m.reloadDetail(msg.issueKey))
}

// statusPicker is the change-status picker: the transitions Jira offers the issue
// it was opened on, then the fields the chosen one needs, and how applying it is
// going.
type statusPicker struct {
	marks    glyphs
	styles   styles
	issue    jira.Issue
	found    []jira.Transition
	listErr  error
	send     sendState
	settled  bool
	selected int
	// form is filling in the chosen transition's fields; it is open when it
	// has any.
	form fieldForm
}

var (
	_ overlay   = statusPicker{}
	_ clickable = statusPicker{}
)

// openStatusPicker opens the picker on the selected issue and starts listing its
// transitions.
func (m Model) openStatusPicker() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.Transitions == nil {
		return m, nil
	}

	m.overlay = statusPicker{marks: m.marks, styles: m.styles, issue: selected}
	list := m.deps.Jira.Transitions

	return m, func() tea.Msg {
		found, err := list(selected.Key)

		return transitionsListed{issueKey: selected.Key, found: found, err: err}
	}
}

// view draws the picker in as many rows as fit.
func (p statusPicker) view(width, rows int) (string, string) {
	lines := []string{p.issue.Key + " " + p.issue.Summary, "status  " + p.issue.Status, ""}

	switch {
	case !p.settled:
		lines = append(lines, "loading statuses"+p.marks.ellipsis)
	case p.listErr != nil:
		lines = append(lines, failedGlyph(p.styles, p.marks)+" "+p.listErr.Error())
	case len(p.found) == 0:
		lines = append(lines, "Jira offers no status change for "+p.issue.Key)
	case p.form.open():
		lines = append(lines, p.form.view(p.marks, p.styles, width, rows-len(lines)-outcomeRows)...)
		lines = append(lines, p.outcome()...)
	default:
		lines = append(lines, p.rows(rows-len(lines)-outcomeRows)...)
		lines = append(lines, p.outcome()...)
	}

	return pickerTitle, strings.Join(lines, "\n")
}

// rows draws as many transitions as fit, scrolled so the selection stays on
// screen. A transition that needs fields says which.
func (p statusPicker) rows(space int) []string {
	first, last := window(p.selected, len(p.found), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		move := p.found[index]
		lines = append(lines, p.marks.marker(index == p.selected)+p.marks.status(move.ToStatusCategory)+" "+
			transitionLabel(p.marks, move)+needs(p.marks, move))
	}

	return lines
}

// outcome says how applying the chosen transition is going, if it was tried.
func (p statusPicker) outcome() []string {
	switch {
	case p.send.sending:
		chosen, _ := p.chosen()

		return []string{"", "changing " + p.issue.Key + " to " + chosen.ToStatus + p.marks.ellipsis}
	case p.send.err != nil:
		return []string{"", failedGlyph(p.styles, p.marks) + " " + p.send.err.Error()}
	default:
		return nil
	}
}

// footer offers what works in the picker now. While a move is being sent,
// nothing interrupts it, so only quitting is offered.
func (p statusPicker) footer(keys keyMap) []key.Binding {
	switch {
	case p.send.sending:
		return []key.Binding{keys.interrupt}
	case p.form.open():
		return p.form.footer(keys)
	default:
		return keys.listKeys()
	}
}

// handleKey answers a key while the picker has the keyboard.
func (p statusPicker) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case p.send.sending:
		// Closing now would hide the answer, and a refused change must never go
		// unseen. The request carries a deadline, so this cannot last.
		return m, nil
	case p.form.open():
		return p.handleFormKey(m, msg)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		p.selected = max(0, min(p.selected+1, len(p.found)-1))
	case key.Matches(msg, m.keys.up):
		p.selected = max(0, p.selected-1)
	case key.Matches(msg, m.keys.confirm):
		return p.choose(m)
	}

	m.overlay = p

	return m, nil
}

// click selects the transition on a clicked line.
func (p statusPicker) click(m Model, line int) (Model, tea.Cmd) {
	rows := m.detailRows() - pickerHeader - outcomeRows
	first, last := window(p.selected, len(p.found), rows)

	index := first + line - pickerHeader
	if p.send.sending || p.form.open() || line < pickerHeader || index >= last {
		return m, nil
	}

	p.selected = index
	m.overlay = p

	return m, nil
}

// chosen is the selected transition, if there is one to apply.
func (p statusPicker) chosen() (jira.Transition, bool) {
	if len(p.found) == 0 {
		return jira.Transition{}, false
	}

	return p.found[p.selected], true
}

// choose goes on with the selected transition: to its fields if it needs any it
// can have filled in here, straight to applying it if it needs none, and nowhere
// if it needs one only Jira's own screen can fill.
func (p statusPicker) choose(m Model) (Model, tea.Cmd) {
	chosen, ok := p.chosen()
	if !ok {
		return m, nil
	}

	if blocked, unfillable := unfillableField(chosen); unfillable {
		p.send.err = errNeedsJira(chosen, blocked)
		m.overlay = p

		return m, nil
	}

	if len(chosen.Fields) > 0 {
		p.send.err, p.form = nil, newFieldForm(chosen)
		m.overlay = p

		return m, nil
	}

	return p.apply(m, chosen, nil)
}

// apply sends a transition with its field values.
func (p statusPicker) apply(m Model, chosen jira.Transition, values []jira.FieldValue) (Model, tea.Cmd) {
	issueKey := p.issue.Key

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would change " + issueKey + " to " + chosen.ToStatus), nil
	}

	p.send = starting()
	m.overlay = p
	transition := m.deps.Jira.Transition

	return m, func() tea.Msg {
		return transitionApplied{issueKey: issueKey, to: chosen, err: transition(issueKey, chosen, values)}
	}
}

// transitionLabel names a transition by its verb and where it leads. A
// transition named for its own status says it once.
func transitionLabel(marks glyphs, move jira.Transition) string {
	if move.Name == move.ToStatus {
		return move.Name
	}

	return move.Name + marks.arrow + move.ToStatus
}

// needs names the fields a transition asks for, so it is no surprise.
func needs(marks glyphs, move jira.Transition) string {
	if len(move.Fields) == 0 {
		return ""
	}

	names := make([]string, 0, len(move.Fields))
	for _, field := range move.Fields {
		names = append(names, field.Name)
	}

	return marks.separator + "needs " + strings.Join(names, ", ")
}
