// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// lastLook is an outward act held for a last look before it goes — a push, a
// re-run of CI, a rebase — so an act reachable from a single key is never one
// key from its request. esc backs out; enter calls proceed, which either sends
// a request of its own through sendingLook — the look then keeps the keys
// until the answer, and a refusal stays pinned under its title until esc — or
// refuses here, or closes the look or opens a run in its place.
type lastLook struct {
	title string
	body  string
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
	stayed string
	// ends is the ending the act asks for: a switch or a reopen, which name
	// what leaving would lose; LeavingNone for an act that leaves the
	// interface open.
	ends    Leaving
	proceed func(m Model) (Model, tea.Cmd)
	// asked is the round the act was sent in, which its answer carries back.
	asked int
	send  send.State
	// problem is why proceed refused the act here, before sending anything.
	problem error
}

var (
	_ sender[lastLook] = lastLook{}
	_ floating         = lastLook{}
)

// A last look would like at least lookWidth columns, room for a sentence of
// what it does and a path or a branch name on a line, and at most
// lookWidthMost, past which a paragraph is hard to read across.
const (
	lookWidth     = 64
	lookWidthMost = 100
)

// wants is a last look as wide as its longest line, between lookWidth and
// lookWidthMost, and as tall as what it says at that width.
func (l lastLook) wants(kit renderKit, most layout.Size) layout.Size {
	widest := min(lookWidthMost, most.Width+dialogFrameColumns)
	_, unwrapped := l.view(kit, widest-dialogFrameColumns, most.Height)
	width := min(max(lookWidth, lipgloss.Width(unwrapped)+dialogFrameColumns), widest)
	_, body := l.view(kit, width-dialogFrameColumns, most.Height)

	return layout.Size{Width: width, Height: strings.Count(body, "\n") + 1 + dialogFrameRows}
}

// view names the act and what it acts on, its outcome pinned under the title:
// the act in flight, or why it was refused, here or by the service.
func (l lastLook) view(kit renderKit, width, _ int) (string, string) {
	lines := append(kit.pinnedProblem(l.problem, width), kit.pinnedSend(l.send, l.doing, width)...)

	return l.title, strings.Join(append(lines, wrap(l.body, width)), "\n")
}

// openedAs is the round the act was sent in. A look reads nothing, so its
// act's answer is the only one it waits for, and it draws its round as the
// act goes rather than as it opens.
func (l lastLook) openedAs() int { return l.asked }

// sent is the act's write.
func (l lastLook) sent() send.State { return l.send }

// withSend is the look with its act's write as state.
func (l lastLook) withSend(state send.State) lastLook {
	l.send = state

	return l
}

// sendingLook sends the act of the last look on top, which handleKey puts
// there before proceed: write makes the request for the round the act goes
// in, and the look keeps the keys, in flight, until its answer. A problem
// from an earlier enter goes, as Sent drops an earlier refusal.
func (m Model) sendingLook(write func(round int) tea.Msg) (Model, tea.Cmd) {
	look, _ := m.overlay.(lastLook)
	m, look.asked = m.nextRound()
	look.send, _ = look.send.Sent()
	look.problem = nil
	m.overlay = look
	round := look.asked

	return m, func() tea.Msg { return write(round) }
}

// refusedHere keeps the last look on top open with problem, why proceed
// would not send its act; a refusal the service gave an earlier enter goes.
func (m Model) refusedHere(problem error) Model {
	look, _ := m.overlay.(lastLook)
	look.problem, look.send = problem, look.send.Cleared()
	m.overlay = look

	return m
}

// footer offers going ahead or backing out, and nothing while the act is in
// flight.
func (l lastLook) footer(keys keyMap) []key.Binding {
	if l.send.InFlight() {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, l.verb), relabel(keys.closeOverlay, cmp.Or(l.leave, escCancel))}
}

// handleKey answers a key while the act waits for its last look.
func (l lastLook) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case l.send.InFlight():
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.overlay = l.back
		if l.stayed != "" {
			m = m.noticed(l.stayed)
		}

		return m, nil
	case key.Matches(msg, m.keys.confirm):
		m.overlay = l

		return l.proceed(m)
	default:
		return m, nil
	}
}

// which names a last look.
func (lastLook) which() overlayKind { return overlayLastLook }
