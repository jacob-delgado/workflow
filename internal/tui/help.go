// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// lightBordered marks an overlay drawn with the light border rather than the
// heavy one — for one that reports rather than acts on anything.
type lightBordered interface {
	lightBorder()
}

// helpOverlay lists every key, grouped by where it works. It is an overlay like
// the pickers and composers rather than a flag on the model, but a quieter one:
// it wears the light border because it acts on nothing, and it carries its own
// scroll so opening it does not move the detail pane's.
type helpOverlay struct {
	// wide is the key list in two columns and narrow the same list in one, both
	// rendered once when the help opens. They come from the model's bindings,
	// which do not change while it is open, so there is nothing to recompute on
	// each frame but which of them the pane's width holds.
	wide, narrow string
	ellipsis     string
	scroll       int
}

var (
	_ overlay       = helpOverlay{}
	_ lightBordered = helpOverlay{}
	_ scrollable    = helpOverlay{}
	_ steppable     = helpOverlay{}
)

func (helpOverlay) lightBorder() {}

// view is the key list scrolled to fit, with a mark on the last row when there
// is more below it.
func (h helpOverlay) view(width, rows int) (string, string) {
	content := h.fitting(width)
	if h.lines(width) <= h.scroll+rows {
		return helpTitle, scrolled(content, h.scroll, rows)
	}

	shown := strings.Split(scrolled(content, h.scroll, max(1, rows-1)), "\n")

	return helpTitle, strings.Join(append(shown, h.ellipsis+" more below"), "\n")
}

// fitting is the key list in two columns where the pane holds them, and in one
// where the second would be cut at the edge, taking a key's name with it.
func (h helpOverlay) fitting(width int) string {
	if lipgloss.Width(h.wide) <= width {
		return h.wide
	}

	return h.narrow
}

// lines is how many lines the key list runs to at a width.
func (h helpOverlay) lines(width int) int {
	return strings.Count(h.fitting(width), "\n") + 1
}

// scrolls reports whether the key list is taller than the pane, so the scroll
// keys move it.
func (h helpOverlay) scrolls(width, rows int) bool {
	return h.lines(width) > rows
}

// footer is what works while the help is open: close it, or quit. A list
// taller than the pane adds the scroll keys after these.
func (helpOverlay) footer(keys keyMap) []key.Binding {
	return []key.Binding{keys.closeOverlay, keys.quit}
}

// handleKey scrolls the help on a terminal too short to show every key at once,
// or closes it.
func (h helpOverlay) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.toggleHelp, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.scrollDown):
		return h.step(m, 1), nil
	case key.Matches(msg, m.keys.scrollUp):
		return h.step(m, -1), nil
	case key.Matches(msg, m.keys.cursorKeys()...):
		return h.step(m, m.keys.stepOf(msg)), nil
	}

	m.overlay = h

	return m, nil
}

// step scrolls the key list by delta half pages, as the scroll keys and up and
// down do.
func (h helpOverlay) step(m Model, delta int) Model {
	m.overlay = h.scrollBy(m, delta*m.halfPage())

	return m
}

// scrollBy moves the key list by delta lines from where it is drawn, clamped to
// the list before and after the move as the detail pane's scroll is, so paging
// past the end leaves no offset for the next page up to spend first.
func (h helpOverlay) scrollBy(m Model, delta int) helpOverlay {
	lines, rows := h.lines(m.detailWidth()), m.detailRows()
	from := firstShown(lines, h.scroll, rows)
	h.scroll = firstShown(lines, from+delta, rows)

	return h
}

// openHelp opens the key list, rendering it from the current bindings.
func (m Model) openHelp() (Model, tea.Cmd) {
	m.overlay = helpOverlay{
		wide:     m.helpView(),
		narrow:   m.helpColumn(0, len(helpGroups(m.cfg.Messaging.Service()))),
		ellipsis: m.marks.ellipsis,
	}

	return m, nil
}

// helpColumnGap is the space between the help's two columns.
const helpColumnGap = 4

// helpView lists every key, grouped by where it works, in two columns so the
// whole set fits a short pane with less scrolling.
func (m Model) helpView() string {
	groups := m.keys.FullHelp()
	split := balancedSplit(groups)
	left := m.helpColumn(0, split)
	right := m.helpColumn(split, len(groups))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, lipgloss.NewStyle().PaddingLeft(helpColumnGap).Render(right))
}

// balancedSplit is the group the help's second column starts at: the one that
// leaves the taller column shortest, worked out from the bindings themselves so
// a key added to either side moves the split with it.
func balancedSplit(groups [][]key.Binding) int {
	heights := make([]int, len(groups))
	for index, group := range groups {
		heights[index] = helpGroupLines(group)
	}

	best, tallest := 1, columnLines(heights)

	for split := 1; split < len(heights); split++ {
		taller := max(columnLines(heights[:split]), columnLines(heights[split:]))
		if taller < tallest {
			best, tallest = split, taller
		}
	}

	return best
}

// helpGroupLines is how many lines a group takes: its name, and a line for each
// binding with help of its own.
func helpGroupLines(group []key.Binding) int {
	lines := 1

	for _, binding := range group {
		if binding.Help().Key != "" {
			lines++
		}
	}

	return lines
}

// columnLines is how tall a column of groups stands, a blank line between each.
func columnLines(heights []int) int {
	total := max(0, len(heights)-1)
	for _, height := range heights {
		total += height
	}

	return total
}

// helpColumn renders the help groups in a range, one key a line under each
// group's name. A binding with no help text of its own is left out; it rides
// another's line.
func (m Model) helpColumn(first, last int) string {
	groups := m.keys.FullHelp()[first:last]
	names := helpGroups(m.cfg.Messaging.Service())[first:last]
	keyWidth := widestHelpKey(groups) + 1

	var lines []string

	for index, group := range groups {
		if index > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, m.styles.strong.Render(names[index]))

		for _, binding := range group {
			shown := binding.Help().Key
			if shown == "" {
				continue
			}

			padding := strings.Repeat(" ", keyWidth-lipgloss.Width(shown))
			lines = append(lines, "  "+shown+padding+binding.Help().Desc)
		}
	}

	return strings.Join(lines, "\n")
}

// widestHelpKey is how many cells the widest key shown in groups takes, so
// every description in a column starts past it, whatever ui.keys binds.
func widestHelpKey(groups [][]key.Binding) int {
	widest := 0

	for _, group := range groups {
		for _, binding := range group {
			widest = max(widest, lipgloss.Width(binding.Help().Key))
		}
	}

	return widest
}
