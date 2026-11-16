// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// overlay is something that takes the keyboard until it is closed: a picker, a
// composer, a preview, a running hook. It draws in the detail pane and is
// drawn with focus while open, so there is never a second pane that looks like
// it has the keys.
//
// Overlays are values. handleKey returns the whole model, so an overlay can
// replace itself, close itself, or change what the panes show as it finishes.
type overlay interface {
	// view is the overlay's title and body, drawn with kit in as many rows as
	// fit.
	view(kit renderKit, width, rows int) (string, string)
	// footer is the keys that do something in it right now.
	footer(keys keyMap) []key.Binding
	handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd)
	// which is the kind of overlay it is, which names the key context
	// CheckKeys reads for it.
	which() overlayKind
}

// renderKit is what an overlay draws with: the interface's glyphs and styles,
// handed to its view rather than copied into every overlay that opens, and
// the frame the in-flight mark of a read has turned to.
type renderKit struct {
	marks  glyphs
	styles styles
	turned string
}

// kit is the glyphs and styles the interface draws with, and the in-flight
// mark of a read turned to its frame.
func (m Model) kit() renderKit {
	return renderKit{styles: m.styles, marks: m.marks, turned: m.motion.turning()}
}

// reading is a pane's first wait, marked in flight as an overlay's wait is,
// with the mark turned as far as it has.
func (k renderKit) reading() string {
	return k.turned + " reading" + k.marks.ellipsis
}

// clickable is an overlay whose rows can be chosen with the mouse. line is the
// row of its body that was clicked, from zero.
type clickable interface {
	click(m Model, line int) (Model, tea.Cmd)
}

// steppable is an overlay that moves on up and down: through a list, or down its
// lines. step moves it by delta steps, as those keys do, so the wheel moves it
// without pressing a key that ui.keys may have moved elsewhere. What one step
// is, is the overlay's: a row of a list, a line of a job's log, half a page of
// the help.
type steppable interface {
	step(m Model, delta int) Model
}

// listKey answers the keys every list overlay answers alike: esc closes it,
// and the cursor keys step it. answered is false for any other key, which the
// list answers itself.
func (m Model) listKey(list steppable, msg tea.KeyPressMsg) (Model, bool) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), true
	case key.Matches(msg, m.keys.cursorKeys()...):
		return list.step(m, m.keys.stepOf(msg)), true
	default:
		return m, false
	}
}

// pasteable is an overlay with a text field that takes a paste: pasted types
// text into the field that has the keyboard, as typing it would.
type pasteable interface {
	pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd)
}

// scrollable is an overlay that can be taller than the pane it is drawn in. It
// says when it is, so its footer offers the scroll keys only where they move it.
type scrollable interface {
	scrolls(width, rows int) bool
}

// overlayKeys is the open overlay's footer, followed by the scroll keys where
// the overlay is taller than the detail pane that draws it. They come last
// because a footer too narrow for every key drops them from the end, and the
// key that closes the overlay matters more.
func (m Model) overlayKeys() []key.Binding {
	keys := m.overlay.footer(m.keys)

	tall, canScroll := m.overlay.(scrollable)
	if !canScroll || !tall.scrolls(m.detailWidth(), m.detailRows()) {
		return keys
	}

	return append(keys, m.keys.scrollUp, m.keys.scrollDown)
}

// editable is an overlay that has handed its body to $EDITOR and takes the
// result back. One textEdited message serves them all, carrying the round the
// overlay that asked opened in, rather than each carrying a message type of
// its own.
type editable interface {
	opened
	applyEdit(m Model, text string, err error) (Model, tea.Cmd)
}

// editedFor is the command an editor's done callback answers with: the text
// back from the editor, for the overlay opened in round opened.
func editedFor(opened int) func(text string, err error) tea.Msg {
	return func(text string, err error) tea.Msg {
		return textEdited{opened: opened, text: text, err: err}
	}
}

// textEdited is a body back from the editor, for the overlay opened in round
// opened.
type textEdited struct {
	opened int
	text   string
	err    error
}

var _ applier = textEdited{}

// apply hands the edited text to the overlay that asked for it, and nothing
// once that overlay has closed, even when another that edits has opened
// since.
func (msg textEdited) apply(m Model) (Model, tea.Cmd) {
	editing, open := answerFor[editable](m, msg.opened)
	if !open {
		return m, nil
	}

	return editing.applyEdit(m, msg.text, msg.err)
}

// applier is a message that knows what it changes: every load and every result
// the interface waits for. Update hands it the model rather than growing a case
// for each one, and a result for an overlay that has since closed does nothing,
// because the message checks what is open before touching it.
type applier interface {
	apply(m Model) (Model, tea.Cmd)
}

// storeNotKept reports a write the store could not keep, made off the update
// loop once what it records was done: words are what the footer says then, the
// deed and why the next session will not remember it.
type storeNotKept struct{ words string }

var _ applier = storeNotKept{}

// apply says what was done, and what of it the store lost.
func (msg storeNotKept) apply(m Model) (Model, tea.Cmd) { return m.noticedUnrepeated(msg.words), nil }

// noticedDraftKept says a closed composer kept what was written, and the key
// that picks it up again.
func (m Model) noticedDraftKept(reopen key.Binding) Model {
	return m.noticed("draft kept; " + reopen.Help().Key + " picks it up again")
}

// noticed sets the footer's report of what just happened, plainly: a result,
// or guidance on what a key needs. A failure no key has cleared yet keeps the
// row, and the report is dropped (notice.Footer's Reported).
func (m Model) noticed(text string) Model {
	m.notice = m.notice.Reported(m.oneRow(text))

	return m
}

// noticedUnrepeated sets a plain notice no pane repeats, such as what the store
// lost. Unlike noticed, it is never dropped, since its words would then be
// shown nowhere: when it cannot be read now, a search over its row among the
// reasons, it waits its turn.
func (m Model) noticedUnrepeated(text string) Model {
	m.notice = m.notice.Unrepeated(m.oneRow(text), m.noticeCovered())

	return m
}

// nextNoticeWaiting gives the notice, once nothing covers its row, to a
// failure a search hid, or else, once nothing holds it, to the oldest notice
// waiting for it (notice.Footer's Next). It runs after a key's own answer, so
// a notice that answer gives keeps the slot from what no pane repeats, which
// is not lost under it.
func (m Model) nextNoticeWaiting() Model {
	m.notice = m.notice.Next(m.noticeCovered())

	return m
}

// oneRow is text folded onto the one row a notice has: any newline — a joined
// staging error, a program's stderr — becomes the separator, or the footer
// grows past the terminal.
func (m Model) oneRow(text string) string {
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })

	return strings.Join(lines, m.marks.separator)
}

// closeOverlay closes whatever overlay is open.
func (m Model) closeOverlay() Model {
	m.overlay = nil

	return m
}

// nextRound draws the next round, for an overlay as it opens or a run as it
// starts, which every answer for it carries back.
func (m Model) nextRound() (Model, int) {
	m.rounds++

	return m, m.rounds
}

// opened is an overlay an answer finds by the round it opened in.
type opened interface {
	overlay
	// openedAs is the round every read and write it starts carries back:
	// the round it opened in, or, for a last look, which reads nothing,
	// the round its act was sent in.
	openedAs() int
}

// answerFor is the open overlay an answer for round is for, when it is the T
// whose openedAs is that round: the one on top, or the one the owner picker
// or a last look floats over, which an answer must still reach, or esc
// would return to it as it was before the answer; or, for a T that is a
// last look, the look itself. An answer for an overlay since closed finds
// none, even when another of its kind has opened since.
//
// Not every success goes back through withAnswered: the directory look
// closes its picker, opens over it or puts it back on top, the check log
// opens in place of the checks, and the pull request composer's reads put
// it back through settled. Each holds only while nothing floats over that
// overlay; a float added over one of them sends these through withAnswered
// too.
func answerFor[T opened](m Model, round int) (T, bool) {
	target := m.overlay

	switch over := m.overlay.(type) {
	case ownerPicker:
		target = over.back
	case lastLook:
		if _, asked := any(over).(T); !asked {
			target = over.back
		}
	}

	open, isOpen := target.(T)
	if !isOpen || open.openedAs() != round {
		var none T

		return none, false
	}

	return open, true
}

// withAnswered puts back the overlay answerFor found, changed: under the
// owner picker floating over it, whose choices it brings up to date, under
// the last look that goes back to it, or on top, a last look among them.
func (m Model) withAnswered(changed opened) Model {
	if links, linking := changed.(linksOwners); linking {
		if picker, picking := m.overlay.(ownerPicker); picking {
			m.overlay = picker.over(links)

			return m
		}
	}

	if look, looking := m.overlay.(lastLook); looking && changed.which() != overlayLastLook {
		look.back = changed
		m.overlay = look

		return m
	}

	m.overlay = changed

	return m
}

// sender is an overlay that sends one write at a time, held in a send.State
// it hands back as sent and takes as withSend, and whose answer finds it by
// the round it opened in.
type sender[T any] interface {
	opened
	sent() send.State
	withSend(state send.State) T
}

// answered takes err as the answer to the write the T opened in round sent,
// and reports true when the write went through, for the applier to do what
// follows a success. A refusal is pinned in the T, put back where answerFor
// found it. Late, the answer changes nothing: no T opened in round is open,
// or its write is not out, and the T returned is the zero value, idle with
// nothing sent, or the open one unchanged.
func answered[T sender[T]](m Model, round int, err error) (Model, T, bool) {
	open, _ := answerFor[T](m, round)
	state, answer := open.sent().Answered(err)
	open = open.withSend(state)

	if answer == send.AnswerRefused {
		m = m.withAnswered(open)
	}

	return m, open, answer == send.AnswerDone
}

// pasted types a bracketed paste into whatever is taking text, by its input
// route: the open overlay's field, the Issues filter or the Tasks search.
// Elsewhere, an overlay with no field among them, a paste does nothing, as an
// unbound key does.
func (m Model) pasted(paste tea.PasteMsg) (Model, tea.Cmd) {
	paste.Content = pastedText(paste.Content)

	route := m.inputRoute()
	if route == routeOverlay {
		if into, ok := m.overlay.(pasteable); ok {
			return into.pasted(m, paste)
		}

		return m, nil
	}

	if route == routeIssueFilter {
		return m.extendFilterBy(oneLine(paste.Content))
	}

	if route == routeTaskFilter {
		m.tasks.listing = m.tasks.listing.extendFilter(oneLine(paste.Content))

		return m.relistTasks(), nil
	}

	return m, nil
}

// oneLine is text with every run of spaces and line breaks one space, for a
// one-line filter that takes a paste.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// pastedText is a paste made safe to type: a carriage return, which terminals
// often send for a pasted line break, becomes a line feed first, so neutralizing
// the terminal controls the paste carries keeps its words apart.
func pastedText(content string) string {
	return sanitize.Text(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content))
}

// offered is a value a checklist offers, with how many listed things hold it.
type offered[F comparable] struct {
	value F
	count int
}

// checklist narrows a list to the values checked in it: each value is listed
// with how many things hold it, space checks or unchecks the one under the
// cursor, and enter applies the checked set. Every list that narrows by values
// — the Issues list by place, the review queue by facet, the Tasks list — opens
// one, and says through apply what applying it changes.
type checklist[F comparable] struct {
	title, none string
	choices     pickList[offered[F]]
	chosen      []F
	label       func(F) string
	apply       func(Model, []F) (Model, tea.Cmd)
}

var _ floating = checklist[string]{}

// checklistWidth is the columns a checklist would like: a value and its
// count beside a checkbox.
const checklistWidth = 48

// wants is a checklist as wide as checklistWidth, and as tall as its values.
func (c checklist[F]) wants(_ renderKit, _ layout.Size) layout.Size {
	return layout.Size{Width: checklistWidth, Height: max(1, len(c.choices.items)) + dialogFrameRows}
}

// view draws the checklist in as many rows as fit.
func (c checklist[F]) view(kit renderKit, _, rows int) (string, string) {
	if len(c.choices.items) == 0 {
		return c.title, c.none
	}

	return c.title, strings.Join(c.choices.rows(kit.marks, rows, c.choiceRow), "\n")
}

// choiceRow is a value, checked when it is picked, with how many hold it.
func (c checklist[F]) choiceRow(choice offered[F]) string {
	return checkbox(slices.Contains(c.chosen, choice.value)) + c.label(choice.value) + "  " +
		strconv.Itoa(choice.count)
}

// footer offers moving, checking a value, applying and canceling.
func (checklist[F]) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		keys.up, keys.down, keys.toggleOption,
		relabel(keys.confirm, "apply"), relabel(keys.closeOverlay, escCancel),
	}
}

// handleKey answers a key while the checklist has the keyboard.
func (c checklist[F]) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if listed, answered := m.listKey(c, msg); answered {
		return listed, nil
	}

	switch {
	case key.Matches(msg, m.keys.confirm):
		return c.apply(m.closeOverlay(), c.chosen)
	case key.Matches(msg, m.keys.toggleOption):
		m.overlay = c.toggled()
	}

	return m, nil
}

// which names a checklist.
func (checklist[F]) which() overlayKind { return overlayChecklist }

// toggled is the checklist with the value under the cursor picked, or unpicked
// when it was.
func (c checklist[F]) toggled() checklist[F] {
	choice, ok := c.choices.chosen()
	if !ok {
		return c
	}

	if index := slices.Index(c.chosen, choice.value); index >= 0 {
		c.chosen = slices.Delete(slices.Clone(c.chosen), index, index+1)

		return c
	}

	c.chosen = append(slices.Clone(c.chosen), choice.value)

	return c
}

// step moves the cursor by delta.
func (c checklist[F]) step(m Model, delta int) Model {
	c.choices = c.choices.moved(delta)
	m.overlay = c

	return m
}

// click moves the cursor to the clicked value.
func (c checklist[F]) click(m Model, line int) (Model, tea.Cmd) {
	c.choices = c.choices.clicked(line, m.detailRows())
	m.overlay = c

	return m, nil
}
