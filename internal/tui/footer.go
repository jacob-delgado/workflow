// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// footer draws the keys that matter right now. A notice normally has its own
// row above this one; only on a terminal too short for that row does the footer
// stand in and report it, so a result is never lost.
func (m Model) footer(width int) string {
	switch {
	case m.showsNotice() || !m.hasNotice():
		return ansi.Truncate(" "+m.footerRow(width-1), width, "")
	case m.notice.text != "":
		return m.notice.row(m.kit(), width)
	}

	return m.narrowingFooter(width)
}

// narrowingFooter is the footer that stands in for the notice row while a
// search or filter narrows the list: what narrows it, then the keys that fit
// beside it.
func (m Model) narrowingFooter(width int) string {
	narrowing := m.noticeLine(width)
	room := width - lipgloss.Width(narrowing) - lipgloss.Width(m.marks.helpSeparator) - 1

	if room <= 0 {
		return narrowing
	}

	return ansi.Truncate(narrowing+m.marks.helpSeparator+m.footerRow(room), width, "")
}

// footerRow offers the keys that do something where the user is, then the way
// to the rest — never a verb with nothing to act on — in room columns. Keys
// that do not fit are dropped whole and an ellipsis says so: the movement keys
// first, which ? lists; then the verbs, from the end; then enter; and the way
// out — ? and q on a pane, esc in an overlay — last of all.
func (m Model) footerRow(room int) string {
	keys, captured := m.capturedKeys()
	if !captured {
		keys = slices.Concat(behaviorOf(m.focus).keys(m), m.keys.ShortHelp())
	}

	return fitKeys(m.kit().keyRow(), keys, room, m.keys.rank)
}

// capturedKeys is the footer of whatever has the keyboard to itself — an open
// overlay, or the issue filter being typed — and whether anything has. The keys
// that work everywhere else, ? among them, do not work there.
func (m Model) capturedKeys() ([]key.Binding, bool) {
	switch {
	case m.overlay != nil:
		return m.overlayKeys(), true
	case m.filteringIssues(), m.filteringTasks():
		return m.filterKeys(), true
	default:
		return nil, false
	}
}

// footerRank orders the keys a footer too narrow for all of them gives
// up: the lowest rank goes first.
type footerRank int

// rank is how long binding holds its place in a footer too narrow for
// every key, told by the keys it answers to, whatever it is labeled here.
func (k keyMap) rank(binding key.Binding) footerRank {
	switch {
	case answersAs(binding, k.toggleHelp, k.closeOverlay, k.quit, k.interrupt):
		return rankWayOut
	case answersAs(binding, k.confirm):
		return rankAct
	case answersAs(binding, k.up, k.down, k.first, k.last, k.scrollUp, k.scrollDown, k.next, k.previous, k.jump):
		return rankMovement
	}

	return rankVerb
}

// answersAs reports binding answering to the same keys as one of others.
func answersAs(binding key.Binding, others ...key.Binding) bool {
	return slices.ContainsFunc(others, func(other key.Binding) bool { return slices.Equal(binding.Keys(), other.Keys()) })
}

// fitKeys draws keys in room columns: all of them where they fit, and otherwise
// without the keys rank gives up first — the last of the lowest rank, one at a
// time — then an ellipsis saying some were dropped.
func fitKeys(row help.Model, keys []key.Binding, room int, rank func(key.Binding) footerRank) string {
	if drawn := row.ShortHelpView(keys); lipgloss.Width(drawn) <= room {
		return drawn
	}

	tail := ellipsisOf(row)
	kept := slices.Clone(keys)

	for len(kept) > 0 && lipgloss.Width(row.ShortHelpView(kept)+tail) > room {
		kept = slices.Delete(kept, firstGivenUp(kept, rank), firstGivenUp(kept, rank)+1)
	}

	return row.ShortHelpView(kept) + tail
}

// firstGivenUp is the index of the key a footer drops next: the last of those
// with the lowest rank.
func firstGivenUp(keys []key.Binding, rank func(key.Binding) footerRank) int {
	dropped := len(keys) - 1

	for index, binding := range slices.Backward(keys) {
		if rank(binding) < rank(keys[dropped]) {
			dropped = index
		}
	}

	return dropped
}

// ellipsisOf is the mark a row of keys ends with when some were dropped.
func ellipsisOf(row help.Model) string {
	return " " + row.Styles.Ellipsis.Inline(true).Render(row.Ellipsis)
}

// keyRow is the footer's key renderer, in this session's styles and marks, with
// no width of its own: footerRow decides what fits, since the renderer's own
// cut, finding no room for its ellipsis, lets a key run past the edge.
func (kit renderKit) keyRow() help.Model {
	row := help.New()
	row.Styles.ShortKey = kit.styles.strong
	row.Styles.ShortDesc = kit.styles.label
	row.Styles.ShortSeparator = kit.styles.label
	row.ShortSeparator, row.Ellipsis = kit.marks.helpSeparator, kit.marks.ellipsis

	return row
}

// relabel is a binding with help that says what it does here.
func relabel(binding key.Binding, help string) key.Binding {
	return key.NewBinding(key.WithKeys(binding.Keys()...), key.WithHelp(binding.Help().Key, help))
}
