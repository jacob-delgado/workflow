// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// overlay is something that takes the keyboard until it is closed: a picker, a
// composer, a preview, a running hook. It draws in the detail pane — Lip Gloss
// v1 cannot layer one view over another — and is drawn with focus while open,
// so there is never a second pane that looks like it has the keys.
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

// Every message the interface waits for is an applier.
var (
	_ applier = issuesLoaded{}
	_ applier = detailLoaded{}
	_ applier = detailDue{}
	_ applier = transitionsListed{}
	_ applier = transitionApplied{}
	_ applier = commentEdited{}
	_ applier = commentPosted{}
	_ applier = branchLoaded{}
	_ applier = branchCreated{}
	_ applier = worktreeCreated{}
	_ applier = branchesListed{}
	_ applier = treeChecked{}
	_ applier = taskSwitched{}
	_ applier = fetched{}
	_ applier = changesLoaded{}
	_ applier = diffLoaded{}
	_ applier = staged{}
	_ applier = runStarted{}
	_ applier = runLine{}
	_ applier = runFinished{}
	_ applier = placesFound{}
	_ applier = editorClosed{}
	_ applier = textEdited{}
	_ applier = pullFound{}
	_ applier = ciChecked{}
	_ applier = checkOpened{}
	_ applier = ciPoll{}
	_ applier = pullCreated{}
	_ applier = pullEdited{}
	_ applier = rerunRequested{}
	_ applier = mergeMethodsLoaded{}
	_ applier = mergeRequested{}
	_ applier = finished{}
	_ applier = issueLinked{}
	_ applier = authorFound{}
	_ applier = messagingPosted{}
	_ applier = hooksFound{}
	_ applier = hooksWritten{}
)

// notice is the footer's one-line report of something that just happened, and
// whether what happened is a failure, which it then draws in the failure style.
type notice struct {
	text   string
	failed bool
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
	doing   string
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

	return []key.Binding{relabel(keys.confirm, l.verb), keys.closeOverlay}
}

// handleKey answers a key while the act waits for its last look.
func (l lastLook) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		l.send = starting()
		m.overlay = l

		return l.proceed(m)
	default:
		return m, nil
	}
}
