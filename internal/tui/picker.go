// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
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
// loop, for the picker opened in round opened.
type transitionsListed struct {
	opened int
	found  []jira.Transition
	err    error
}

var _ applier = transitionsListed{}

// apply records a listing in the picker that asked for it, and nothing once
// that picker has closed, even when another has opened since: on another
// issue, or on the same one, whose own listing is on its way.
func (msg transitionsListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := answerFor[statusPicker](m, msg.opened)
	if !open {
		return m, nil
	}

	if msg.err == nil && picker.offer.absent(msg.found) {
		// A listing without the named status makes no offer, rather than pre-select
		// an unrelated move; a failed listing stays open so its failure is shown.
		return m.closeOverlay(), nil
	}

	// The picker reads its moves once, as it opens, before it could begin a
	// round: the listing is round 0's.
	picker.listing, _ = picker.listing.Answered(0, msg.found, msg.err)
	picker.selected = picker.offer.pick(msg.found)

	return m.withAnswered(picker), nil
}

// offerKind is which transition a picker opened as an offer pre-selects.
type offerKind int

const (
	// offerNone pre-selects none: the picker was opened by hand.
	offerNone offerKind = iota
	// offerInProgress pre-selects the first in-progress move, after branching.
	offerInProgress
	// offerNamed pre-selects the move to a status named by the configuration,
	// after a pull request.
	offerNamed
)

// statusOffer decides which transition the picker pre-selects when it opens as
// an offer: the one leading to a named status (after a pull request), the first
// in-progress one (after branching), or none (opened by hand). The zero value
// pre-selects none.
type statusOffer struct {
	kind offerKind
	// status is the status an offerNamed offer leads to.
	status string
}

// pick is the index of the transition the offer pre-selects, or zero.
func (o statusOffer) pick(moves []jira.Transition) int {
	return map[offerKind]func() int{
		offerNone:       func() int { return 0 },
		offerInProgress: func() int { return firstInProgress(moves) },
		offerNamed: func() int {
			index, _ := jira.FindTransition(moves, o.status)

			return index
		},
	}[o.kind]()
}

// absent reports a named-status offer whose status is not among the moves, so
// there is nothing to pre-select and no offer worth opening the picker for.
func (o statusOffer) absent(moves []jira.Transition) bool {
	if o.kind != offerNamed {
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

// transitionApplied reports how applying a transition went, for the picker
// opened in round opened.
type transitionApplied struct {
	opened   int
	issueKey jira.Key
	to       jira.Transition
	err      error
}

var _ applier = transitionApplied{}

// apply reports a transition's outcome to the picker that sent it. A refusal
// keeps the picker open with Jira's reason, to choose again; a move that worked
// closes it and refreshes the list, since the issue's status — and perhaps its
// place in the list — just changed. A move to done offers to complete the
// issue's task. An answer for a picker since closed changes nothing.
func (msg transitionApplied) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[statusPicker](m, msg.opened, msg.err)
	if !done {
		return m, nil
	}

	m = m.closeOverlay().noticed(m.marks.done + " " + shownKey(msg.issueKey) + " is now " + msg.to.ToStatus)
	m.followUp = m.orMarkDone(msg.issueKey, msg.to)
	m, list := m.readingIssues()
	m, detail := m.reloadDetail(msg.issueKey)

	return m, tea.Batch(list, detail)
}

// statusPicker is the change-status picker: the transitions Jira offers the issue
// it was opened on, then the fields the chosen one needs, and how applying it is
// going.
type statusPicker struct {
	issue jira.Issue
	// listing is the transitions Jira offers the issue, read once as the
	// picker opens; selected is which of them is chosen.
	listing  latest.Read[[]jira.Transition]
	selected int
	stage    pickerStage
	// form is filling in the chosen transition's fields while the picker is
	// pickerFilling.
	form fieldForm
	// send is the move sent to Jira, and problem what keeps the chosen move
	// from being sent from here: a field only Jira's own screen can fill, or
	// fields left unanswered.
	send    send.State
	problem error
	// offer pre-selects a transition once the list arrives, for an offer made
	// after branching or after a pull request; the zero value pre-selects none.
	offer statusOffer
	// opened is the round the picker opened in, which its listing and its
	// move carry back, so neither lands in a picker opened after.
	opened int
}

var (
	_ sender[statusPicker] = statusPicker{}
	_ clickable            = statusPicker{}
	_ steppable            = statusPicker{}
	_ pasteable            = statusPicker{}
	_ relaying             = statusPicker{}
)

// moving reports the chosen move sent and not yet answered.
func (p statusPicker) moving() bool {
	return p.send.Stage() == send.StageInFlight
}

// openStatusPicker opens the picker on the selected issue and starts listing its
// transitions.
func (m Model) openStatusPicker() (Model, tea.Cmd) {
	selected, _ := m.issues.current()

	return m.pickStatusFor(selected, statusOffer{})
}

// pickStatusFor opens the picker on an issue and starts listing its transitions.
// The offer pre-selects a transition once they arrive, for an offer made after
// branching or a pull request rather than the picker opened by hand.
func (m Model) pickStatusFor(issue jira.Issue, offer statusOffer) (Model, tea.Cmd) {
	if m.deps.Jira.Transitions == nil {
		return m, nil
	}

	m, opened := m.nextRound()
	m.overlay = statusPicker{issue: issue, offer: offer, opened: opened}
	ctx, list := m.session, m.deps.Jira.Transitions

	return m, func() tea.Msg {
		found, err := list(ctx, issue.Key)

		return transitionsListed{opened: opened, found: found, err: err}
	}
}

// offerReviewStatus offers to move an issue to the configured review status once
// its pull request is open, or closes the overlay when none is configured or
// Jira does not offer it. The status is chosen by name because it shares a
// category with "in progress". The picker names the issue as the list has it,
// summary and status, and only the key when it is not listed.
func (m Model) offerReviewStatus(issueKey jira.Key) (Model, tea.Cmd) {
	if !offersReviewStatus(m.cfg, m.deps) {
		return m.closeOverlay(), nil
	}

	issue, listed := m.issues.find(issueKey)
	if !listed {
		issue = jira.Issue{Key: issueKey}
	}

	return m.pickStatusFor(issue, statusOffer{kind: offerNamed, status: m.cfg.Jira.ReviewStatus})
}

// offersReviewStatus reports whether an open pull request is followed by the
// offer of the review status: one is configured, and Jira can list the moves.
func offersReviewStatus(cfg config.Config, deps Deps) bool {
	return cfg.Jira.ReviewStatus != "" && deps.Jira.Transitions != nil
}

// header is the rows above the picker's list: the issue, its status, and a
// blank. The view draws it and the click measures it, so a change to one cannot
// silently break the other's row math.
func (p statusPicker) header() []string {
	return []string{shownKey(p.issue.Key) + " " + p.issue.Summary, "status  " + p.issue.Status, ""}
}

// view draws the picker in as many rows as fit: reading its transitions,
// why they could not be read, or them.
func (p statusPicker) view(kit renderKit, _, rows int) (string, string) {
	lines := p.header()
	space := rows - len(lines)
	failed := func() []string { return []string{kit.failureLine(p.listing.Err())} }

	lines = append(lines, drawRead(p.listing, drawnRead[[]string]{
		reading:    func() []string { return []string{"reading the statuses" + kit.marks.ellipsis} },
		shown:      func() []string { return p.listed(kit, space) },
		failed:     failed,
		failedOver: failed,
	})...)

	return pickerTitle, strings.Join(lines, "\n")
}

// listed draws the transitions read in space rows: that Jira offers none,
// the fields the chosen one needs, or the transitions to choose from, with
// how applying the chosen one is going.
func (p statusPicker) listed(kit renderKit, space int) []string {
	moves := p.moves()

	switch {
	case len(moves.items) == 0:
		return []string{"Jira offers no status change for " + shownKey(p.issue.Key)}
	case p.filling():
		return append(p.form.view(kit), p.outcome(kit)...)
	default:
		return append(moves.rows(kit.marks, space-outcomeRows, func(move jira.Transition) string {
			return transitionRow(kit.marks, move)
		}), p.outcome(kit)...)
	}
}

// moves is the transitions read, none until they are, with the chosen one.
func (p statusPicker) moves() pickList[jira.Transition] {
	found, _ := p.listing.Value()

	return pickList[jira.Transition]{items: found, selected: p.selected}
}

// transitionRow names a transition by where it leads, and says which fields it
// needs, if any.
func transitionRow(marks glyphs, move jira.Transition) string {
	return marks.status(move.ToStatusCategory) + " " + transitionLabel(marks, move) + needs(marks, move)
}

// outcome says how applying the chosen transition is going, if it was tried:
// being sent, refused, or kept from going by a problem found here.
func (p statusPicker) outcome(kit renderKit) []string {
	if p.moving() {
		chosen, _ := p.moves().chosen()

		return []string{"", "changing " + shownKey(p.issue.Key) + " to " + chosen.ToStatus + kit.marks.ellipsis}
	}

	cause := p.problem
	if cause == nil {
		cause = p.send.Reason()
	}

	if cause == nil {
		return nil
	}

	return []string{"", kit.failureLine(cause)}
}

// footer offers what works in the picker now. While a move is being sent,
// nothing interrupts it, so only quitting is offered.
func (p statusPicker) footer(keys keyMap) []key.Binding {
	switch {
	case p.moving():
		return []key.Binding{keys.interrupt}
	case p.filling():
		return p.form.footer(keys)
	default:
		return keys.listKeys()
	}
}

// handleKey answers a key while the picker has the keyboard.
func (p statusPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.moving():
		// Closing now would hide the answer, and a refused change must never go
		// unseen. The request carries a deadline, so this cannot last.
		return m, nil
	case p.filling():
		return p.handleFormKey(m, msg)
	}

	if listed, answered := m.listKey(p, msg); answered {
		return listed, nil
	}

	if key.Matches(msg, m.keys.confirm) {
		return p.choose(m)
	}

	return m, nil
}

// which names the status picker.
func (statusPicker) which() overlayKind { return overlayStatusPicker }

// openedAs is the round the picker opened in.
func (p statusPicker) openedAs() int { return p.opened }

// sent is the move sent to Jira.
func (p statusPicker) sent() send.State { return p.send }

// withSend is the picker with its move as state.
func (p statusPicker) withSend(state send.State) statusPicker {
	p.send = state

	return p
}

// click selects the transition on a clicked line.
func (p statusPicker) click(m Model, line int) (Model, tea.Cmd) {
	if p.moving() || p.filling() {
		return m, nil
	}

	header := len(p.header())
	p.selected = p.moves().clicked(line-header, m.detailRows()-header-outcomeRows).selected
	m.overlay = p

	return m, nil
}

// choose goes on with the selected transition: to its fields if it needs any it
// can have filled in here, straight to applying it if it needs none, and nowhere
// if it needs one only Jira's own screen can fill.
func (p statusPicker) choose(m Model) (Model, tea.Cmd) {
	chosen, ok := p.moves().chosen()
	if !ok {
		return m, nil
	}

	p.send = p.send.Cleared()

	if blocked, unfillable := chosen.Unfillable(); unfillable {
		m.overlay = p.choosing(errNeedsJira(chosen, blocked))

		return m, nil
	}

	if len(chosen.Fields) > 0 {
		var start tea.Cmd

		p.stage, p.problem = pickerFilling, nil
		width, rows := p.formSize(m)
		p.form, start = newFieldForm(chosen, m.kit(), m.keys, width, rows)
		m.overlay = p
		form := p.form.form

		return m, m.startForm(form, start, func(err error) tea.Msg { return fieldsFilled{form: form, err: err} })
	}

	return p.apply(m, chosen, nil)
}

// apply sends a transition with its field values. A dry run sends nothing,
// but makes the offer the move would, as a preview of what follows it.
func (p statusPicker) apply(m Model, chosen jira.Transition, values []jira.FieldValue) (Model, tea.Cmd) {
	issueKey := p.issue.Key

	if m.dryRun {
		m = m.closeOverlay().noticed("dry run: would change " + shownKey(issueKey) + " to " + chosen.ToStatus)
		m.followUp = m.orMarkDone(issueKey, chosen)

		return m, nil
	}

	p.send, _ = p.send.Sent()
	p.problem = nil
	m.overlay = p
	ctx, transition, opened := m.session, m.deps.Jira.Transition, p.opened

	return m, func() tea.Msg {
		err := transition(ctx, issueKey, chosen, values)

		return transitionApplied{opened: opened, issueKey: issueKey, to: chosen, err: err}
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
