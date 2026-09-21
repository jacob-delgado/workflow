// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// pickerTitle titles the detail pane while the picker is open.
const pickerTitle = "Change status"

// outcomeRows is the room kept under the list for how applying is going.
const outcomeRows = 2

// transitionsListed carries Jira's transitions for one issue back into the update
// loop.
type transitionsListed struct {
	issueKey jira.Key
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

	if picker.preferInProgress {
		picker.selected = firstInProgress(msg.found)
	}

	m.overlay = picker

	return m, nil
}

// firstInProgress is the index of the first transition that leads to an
// in-progress status, or zero when none does — the status the loop implies once
// a branch exists, chosen by category rather than by a localized name.
func firstInProgress(moves []jira.Transition) int {
	for index, move := range moves {
		if move.ToStatusCategory == jira.CategoryIndeterminate {
			return index
		}
	}

	return 0
}

// transitionApplied reports how applying a transition went.
type transitionApplied struct {
	issueKey jira.Key
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

	m = m.closeOverlay().noticed(m.marks.done + " " + string(msg.issueKey) + " is now " + msg.to.ToStatus)

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
	// preferInProgress pre-selects the first in-progress transition once the
	// list arrives, for the offer made right after branching.
	preferInProgress bool
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
	if !ok {
		return m, nil
	}

	return m.pickStatusFor(selected, false)
}

// pickStatusFor opens the picker on an issue and starts listing its transitions.
// preferInProgress pre-selects the first in-progress transition once they
// arrive, for the offer made after branching rather than the picker opened by
// hand.
func (m Model) pickStatusFor(issue jira.Issue, preferInProgress bool) (Model, tea.Cmd) {
	if m.deps.Jira.Transitions == nil {
		return m, nil
	}

	m.overlay = statusPicker{marks: m.marks, styles: m.styles, issue: issue, preferInProgress: preferInProgress}
	list := m.deps.Jira.Transitions

	return m, func() tea.Msg {
		found, err := list(issue.Key)

		return transitionsListed{issueKey: issue.Key, found: found, err: err}
	}
}

// header is the rows above the picker's list: the issue, its status, and a
// blank. The view draws it and the click measures it, so a change to one cannot
// silently break the other's row math.
func (p statusPicker) header() []string {
	return []string{string(p.issue.Key) + " " + p.issue.Summary, "status  " + p.issue.Status, ""}
}

// view draws the picker in as many rows as fit.
func (p statusPicker) view(width, rows int) (string, string) {
	lines := p.header()

	switch {
	case !p.settled:
		lines = append(lines, "loading statuses"+p.marks.ellipsis)
	case p.listErr != nil:
		lines = append(lines, failedGlyph(p.styles, p.marks)+" "+p.listErr.Error())
	case len(p.found) == 0:
		lines = append(lines, "Jira offers no status change for "+string(p.issue.Key))
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

		return []string{"", "changing " + string(p.issue.Key) + " to " + chosen.ToStatus + p.marks.ellipsis}
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
func (p statusPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
	header := len(p.header())
	rows := m.detailRows() - header - outcomeRows
	first, last := window(p.selected, len(p.found), rows)

	index := first + line - header
	if p.send.sending || p.form.open() || line < header || index >= last {
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
		return m.closeOverlay().noticed("dry run: would change " + string(issueKey) + " to " + chosen.ToStatus), nil
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

// fixupTitle titles the pane while the fixup picker is open.
const fixupTitle = "Fix up a commit"

// fixupPicker chooses which unpushed commit to record a fixup! of.
type fixupPicker struct {
	marks    glyphs
	styles   styles
	commits  []gitrepo.Commit
	selected int
}

var _ overlay = fixupPicker{}

// openFixupPicker offers the branch's unpushed commits, the most recent first so
// the likeliest target is the default selection.
func (m Model) openFixupPicker() (Model, tea.Cmd) {
	if m.deps.Git.Fixup == nil || !m.canFoldStaged() {
		return m, nil
	}

	newestFirst := slices.Clone(m.branch.branch.Unpushed())
	slices.Reverse(newestFirst)
	m.overlay = fixupPicker{marks: m.marks, styles: m.styles, commits: newestFirst}

	return m, nil
}

// header is the rows above the fixup picker's list: a prompt and a blank.
func (p fixupPicker) header() []string {
	return []string{"Fold the staged changes into which commit?", ""}
}

// view draws the commits to choose from, in as many rows as fit.
func (p fixupPicker) view(_, rows int) (string, string) {
	lines := p.header()
	lines = append(lines, p.rows(rows-len(lines))...)

	return fixupTitle, strings.Join(lines, "\n")
}

// rows draws as many commits as fit, scrolled so the selection stays on screen.
func (p fixupPicker) rows(space int) []string {
	first, last := window(p.selected, len(p.commits), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		commit := p.commits[index]
		lines = append(lines, p.marks.marker(index == p.selected)+
			p.styles.label.Render(commit.Hash)+" "+commit.Subject)
	}

	return lines
}

// footer offers moving, choosing and leaving.
func (p fixupPicker) footer(keys keyMap) []key.Binding {
	return keys.listKeys()
}

// handleKey moves the selection, chooses a commit to fix up, or leaves.
func (p fixupPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.down):
		p.selected = min(p.selected+1, len(p.commits)-1)
	case key.Matches(msg, m.keys.up):
		p.selected = max(0, p.selected-1)
	case key.Matches(msg, m.keys.confirm):
		chosen := p.commits[p.selected]

		return m.applyFixup(chosen.Hash, chosen.Subject)
	}

	m.overlay = p

	return m, nil
}
