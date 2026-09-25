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

// pickList is a list to choose one row from: the rows, and which is chosen.
// Every picker that windows its list keeps it in one, so moving, the window
// that keeps the choice in sight, and a click all count rows the same way.
type pickList[T any] struct {
	items    []T
	selected int
}

// moved is the list with the choice moved by step rows, held within the list.
func (l pickList[T]) moved(step int) pickList[T] {
	l.selected = max(0, min(l.selected+step, len(l.items)-1))

	return l
}

// chosen is the chosen row, or false when the list has none.
func (l pickList[T]) chosen() (T, bool) {
	if len(l.items) == 0 {
		var none T

		return none, false
	}

	return l.items[l.selected], true
}

// rows draws as many rows as fit in space, scrolled so the choice stays in
// sight, each labeled behind the selection marker.
func (l pickList[T]) rows(marks glyphs, space int, label func(T) string) []string {
	first, last := window(l.selected, len(l.items), space)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		lines = append(lines, marks.marker(index == l.selected)+label(l.items[index]))
	}

	return lines
}

// clicked is the list with the row rows drew on line chosen, line counted from
// the first row it drew; a line that drew no row changes nothing.
func (l pickList[T]) clicked(line, space int) pickList[T] {
	first, last := window(l.selected, len(l.items), space)
	if index := first + line; line >= 0 && index < last {
		l.selected = index
	}

	return l
}

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

	if msg.err == nil && picker.offer.absent(msg.found) {
		// A listing without the named status makes no offer, rather than pre-select
		// an unrelated move; a failed listing stays open so its failure is shown.
		return m.closeOverlay(), nil
	}

	picker.transitions = pickList[jira.Transition]{items: msg.found, selected: picker.offer.pick(msg.found)}
	picker.listErr, picker.settled = msg.err, true

	m.overlay = picker

	return m, nil
}

// statusOffer decides which transition the picker pre-selects when it opens as
// an offer: the one leading to a named status (after a pull request), the first
// in-progress one (after branching), or none (opened by hand).
type statusOffer struct {
	inProgress bool
	status     string
}

// pick is the index of the transition the offer pre-selects, or zero.
func (o statusOffer) pick(moves []jira.Transition) int {
	switch {
	case o.status != "":
		index, _ := jira.FindTransition(moves, o.status)

		return index
	case o.inProgress:
		return firstInProgress(moves)
	default:
		return 0
	}
}

// absent reports a named-status offer whose status is not among the moves, so
// there is nothing to pre-select and no offer worth opening the picker for.
func (o statusOffer) absent(moves []jira.Transition) bool {
	if o.status == "" {
		return false
	}

	_, found := jira.FindTransition(moves, o.status)

	return !found
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
		return keepOpenWith[statusPicker](m, writeRefusal(msg.err)), nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + string(msg.issueKey) + " is now " + msg.to.ToStatus)
	m, detail := m.reloadDetail(msg.issueKey)

	return m, tea.Batch(m.searchIssues(), detail)
}

// failed is the picker back on its transitions with the reason, the field form
// it was filling in closed, to choose again.
func (p statusPicker) failed(err error) statusPicker {
	p.send, p.form = p.send.failed(err), fieldForm{}

	return p
}

// statusPicker is the change-status picker: the transitions Jira offers the issue
// it was opened on, then the fields the chosen one needs, and how applying it is
// going.
type statusPicker struct {
	marks       glyphs
	styles      styles
	issue       jira.Issue
	transitions pickList[jira.Transition]
	listErr     error
	send        sendState
	settled     bool
	// offer pre-selects a transition once the list arrives, for an offer made
	// after branching or after a pull request; the zero value pre-selects none.
	offer statusOffer
	// form is filling in the chosen transition's fields; it is open when it
	// has any.
	form fieldForm
}

var (
	_ failable[statusPicker] = statusPicker{}
	_ clickable              = statusPicker{}
)

// openStatusPicker opens the picker on the selected issue and starts listing its
// transitions.
func (m Model) openStatusPicker() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok {
		return m, nil
	}

	return m.pickStatusFor(selected, statusOffer{})
}

// pickStatusFor opens the picker on an issue and starts listing its transitions.
// The offer pre-selects a transition once they arrive, for an offer made after
// branching or a pull request rather than the picker opened by hand.
func (m Model) pickStatusFor(issue jira.Issue, offer statusOffer) (Model, tea.Cmd) {
	if m.deps.Jira.Transitions == nil {
		return m, nil
	}

	m.overlay = statusPicker{marks: m.marks, styles: m.styles, issue: issue, offer: offer}
	list := m.deps.Jira.Transitions

	return m, func() tea.Msg {
		found, err := list(issue.Key)

		return transitionsListed{issueKey: issue.Key, found: found, err: err}
	}
}

// offerReviewStatus offers to move an issue to the configured review status once
// its pull request is open, or closes the overlay when none is configured or
// Jira does not offer it. The status is chosen by name because it shares a
// category with "in progress".
func (m Model) offerReviewStatus(issueKey jira.Key) (Model, tea.Cmd) {
	if m.cfg.Jira.ReviewStatus == "" || m.deps.Jira.Transitions == nil {
		return m.closeOverlay(), nil
	}

	return m.pickStatusFor(jira.Issue{Key: issueKey}, statusOffer{status: m.cfg.Jira.ReviewStatus})
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
		lines = append(lines, failureLine(p.styles, p.marks, p.listErr))
	case len(p.transitions.items) == 0:
		lines = append(lines, "Jira offers no status change for "+string(p.issue.Key))
	case p.form.open():
		lines = append(lines, p.form.view(p.marks, p.styles, width, rows-len(lines)-outcomeRows)...)
		lines = append(lines, p.outcome()...)
	default:
		lines = append(lines, p.transitions.rows(p.marks, rows-len(lines)-outcomeRows, p.transitionRow)...)
		lines = append(lines, p.outcome()...)
	}

	return pickerTitle, strings.Join(lines, "\n")
}

// transitionRow names a transition by where it leads, and says which fields it
// needs, if any.
func (p statusPicker) transitionRow(move jira.Transition) string {
	return p.marks.status(move.ToStatusCategory) + " " + transitionLabel(p.marks, move) + needs(p.marks, move)
}

// outcome says how applying the chosen transition is going, if it was tried.
func (p statusPicker) outcome() []string {
	switch {
	case p.send.sending:
		chosen, _ := p.transitions.chosen()

		return []string{"", "changing " + string(p.issue.Key) + " to " + chosen.ToStatus + p.marks.ellipsis}
	case p.send.err != nil:
		return []string{"", failureLine(p.styles, p.marks, p.send.err)}
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
		p.transitions = p.transitions.moved(1)
	case key.Matches(msg, m.keys.up):
		p.transitions = p.transitions.moved(-1)
	case key.Matches(msg, m.keys.confirm):
		return p.choose(m)
	}

	m.overlay = p

	return m, nil
}

// click selects the transition on a clicked line.
func (p statusPicker) click(m Model, line int) (Model, tea.Cmd) {
	if p.send.sending || p.form.open() {
		return m, nil
	}

	header := len(p.header())
	p.transitions = p.transitions.clicked(line-header, m.detailRows()-header-outcomeRows)
	m.overlay = p

	return m, nil
}

// choose goes on with the selected transition: to its fields if it needs any it
// can have filled in here, straight to applying it if it needs none, and nowhere
// if it needs one only Jira's own screen can fill.
func (p statusPicker) choose(m Model) (Model, tea.Cmd) {
	chosen, ok := p.transitions.chosen()
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
	marks   glyphs
	styles  styles
	commits pickList[gitrepo.Commit]
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
	m.overlay = fixupPicker{marks: m.marks, styles: m.styles, commits: pickList[gitrepo.Commit]{items: newestFirst}}

	return m, nil
}

// header is the rows above the fixup picker's list: a prompt and a blank.
func (p fixupPicker) header() []string {
	return []string{"Fold the staged changes into which commit?", ""}
}

// view draws the commits to choose from, in as many rows as fit.
func (p fixupPicker) view(_, rows int) (string, string) {
	lines := p.header()
	lines = append(lines, p.commits.rows(p.marks, rows-len(lines), p.commitRow)...)

	return fixupTitle, strings.Join(lines, "\n")
}

// commitRow names a commit by its hash and subject.
func (p fixupPicker) commitRow(commit gitrepo.Commit) string {
	return p.styles.label.Render(commit.Hash) + " " + commit.Subject
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
		p.commits = p.commits.moved(1)
	case key.Matches(msg, m.keys.up):
		p.commits = p.commits.moved(-1)
	case key.Matches(msg, m.keys.confirm):
		// The picker opens only over unpushed commits, so one is always chosen.
		chosen, _ := p.commits.chosen()

		return m.applyFixup(chosen.Hash, chosen.Subject)
	}

	m.overlay = p

	return m, nil
}
