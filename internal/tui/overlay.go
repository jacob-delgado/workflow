// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// overlay is something that takes the keyboard until it is closed: a picker, a
// composer, a preview, a running hook. It draws in the detail pane and is
// drawn with focus while open, so there is never a second pane that looks like
// it has the keys.
//
// Overlays are values. handleKey returns the whole model, so an overlay can
// replace itself, close itself, or change what the panes show as it finishes.
type overlay interface {
	// view is the overlay's title and body, in as many rows as fit.
	view(width, rows int) (string, string)
	// footer is the keys that do something in it right now.
	footer(keys keyMap) []key.Binding
	handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd)
}

// clickable is an overlay whose rows can be chosen with the mouse. line is the
// row of its body that was clicked, from zero.
type clickable interface {
	click(m Model, line int) (Model, tea.Cmd)
}

// steppable is an overlay that moves on up and down: through a list, or down its
// lines. step moves it by delta, as those keys do, so the wheel moves it
// without pressing a key that ui.keys may have moved elsewhere.
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
// result back. One textEdited message serves them all: the open overlay is the
// one that asked, so it updates whichever overlay is editing rather than each
// carrying a message type of its own.
type editable interface {
	overlay
	applyEdit(m Model, text string, err error) (Model, tea.Cmd)
}

// textEdited is a body back from the editor, for whichever overlay is editing.
type textEdited struct {
	text string
	err  error
}

var _ applier = textEdited{}

// apply hands the edited text to the open overlay, or drops it if the overlay
// that asked has since closed.
func (msg textEdited) apply(m Model) (Model, tea.Cmd) {
	editing, open := m.overlay.(editable)
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

// notice is the footer's one-line report of something that just happened, and
// whether what happened is a failure, which it then draws in the failure style.
type notice struct {
	text   string
	failed bool
}

// storeNotKept reports a write the store could not keep, made off the update
// loop once what it records was done: notice is what the footer says then, the
// deed and why the next session will not remember it.
type storeNotKept struct{ notice string }

var _ applier = storeNotKept{}

// apply says what was done, and what of it the store lost.
func (msg storeNotKept) apply(m Model) (Model, tea.Cmd) { return m.noticed(msg.notice), nil }

// noticedDraftKept says a closed composer kept what was written, and the key
// that picks it up again.
func (m Model) noticedDraftKept(reopen key.Binding) Model {
	return m.noticed("draft kept; " + reopen.Help().Key + " picks it up again")
}

// noticed sets the footer's report of what just happened, plainly: a result,
// or guidance on what a key needs.
func (m Model) noticed(text string) Model {
	// A notice is one row, so any newline — a joined staging error, a program's
	// stderr — is folded onto it, or the footer grows past the terminal.
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	m.notice = notice{text: strings.Join(lines, m.marks.separator), failed: false}

	return m
}

// closeOverlay closes whatever overlay is open.
func (m Model) closeOverlay() Model {
	m.overlay = nil

	return m
}

// opening counts one more overlay opened whose reads answer later, and is
// the count it is opened as.
func (m Model) opening() (Model, int) {
	m.overlaysOpened++

	return m, m.overlaysOpened
}

// failable is an overlay that stays open when the request it sent fails, and
// says why. T is the overlay's own type, so a failure lands only in the kind of
// overlay that sent the request.
type failable[T any] interface {
	overlay
	failed(err error) T
}

// keepOpenWith pins err in the open overlay when it is a T — the kind that sent
// the request — and changes nothing when that overlay has since closed.
func keepOpenWith[T failable[T]](m Model, err error) Model {
	if open, isOpen := m.overlay.(T); isOpen {
		m.overlay = open.failed(err)
	}

	return m
}

// lastLook is an outward act held for a last look before it goes — a push, a
// re-run of CI, a rebase — so an act reachable from a single key is never one
// key from its request. esc backs out; enter marks the look in flight and calls
// proceed, which either sends a request of its own — the look then keeps the
// keys until the answer, and a refusal stays pinned under its title until esc —
// or closes the look or opens a run in its place.
type lastLook struct {
	marks  glyphs
	styles styles
	title  string
	body   string
	// verb names the act on the confirm key: "push", "re-run", "rebase".
	verb string
	// doing is the act in flight, pinned under the title while proceed's request
	// is out. Empty for a look whose proceed opens a run, which never shows it.
	doing string
	// leave names esc on the footer: skip for an offer that follows a done act,
	// stay at a guard, back for a step inside a flow; empty cancels.
	leave string
	// back is the overlay esc goes back to, nil where it closes the look.
	back overlay
	// stayed is said when esc leaves the look, or nothing is.
	stayed  string
	proceed func(m Model) (Model, tea.Cmd)
	send    sendState
}

var _ failable[lastLook] = lastLook{}

// view names the act and what it acts on, its outcome pinned under the title.
func (l lastLook) view(width, _ int) (string, string) {
	lines := pinnedOutcome(l.styles, l.marks, l.send, l.doing, width)

	return l.title, strings.Join(append(lines, wrap(l.body, width)), "\n")
}

// failed is the look kept open with the reason its act was refused.
func (l lastLook) failed(err error) lastLook {
	l.send = l.send.failed(err)

	return l
}

// footer offers going ahead or backing out, and nothing while the act is in
// flight.
func (l lastLook) footer(keys keyMap) []key.Binding {
	if l.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, l.verb), relabel(keys.closeOverlay, cmp.Or(l.leave, escCancel))}
}

// handleKey answers a key while the act waits for its last look.
func (l lastLook) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.overlay = l.back
		if l.stayed != "" {
			m = m.noticed(l.stayed)
		}

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		l.send = starting()
		m.overlay = l

		return l.proceed(m)
	default:
		return m, nil
	}
}

// pasted types a bracketed paste into whatever is taking text: the open
// overlay's field, or the Issues filter. Elsewhere a paste does nothing, as an
// unbound key does.
func (m Model) pasted(paste tea.PasteMsg) (Model, tea.Cmd) {
	paste.Content = pastedText(paste.Content)

	if into, ok := m.overlay.(pasteable); ok {
		return into.pasted(m, paste)
	}

	switch {
	case m.overlay != nil:
		return m, nil
	case m.filteringIssues():
		return m.extendFilterBy(oneLine(paste.Content))
	case m.filteringTasks():
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
	marks       glyphs
	title, none string
	choices     pickList[offered[F]]
	chosen      []F
	label       func(F) string
	apply       func(Model, []F) (Model, tea.Cmd)
}

// view draws the checklist in as many rows as fit.
func (c checklist[F]) view(_, rows int) (string, string) {
	if len(c.choices.items) == 0 {
		return c.title, c.none
	}

	return c.title, strings.Join(c.choices.rows(c.marks, rows, c.choiceRow), "\n")
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
