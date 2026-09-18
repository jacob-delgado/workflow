// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	// content is the two-column key list, rendered once when the help opens. It
	// comes from the model's bindings, which do not change while it is open, so
	// there is nothing to recompute on each frame.
	content  string
	ellipsis string
	scroll   int
}

var (
	_ overlay       = helpOverlay{}
	_ lightBordered = helpOverlay{}
)

func (helpOverlay) lightBorder() {}

// view is the key list scrolled to fit, with a mark on the last row when there
// is more below it.
func (h helpOverlay) view(_, rows int) (string, string) {
	if strings.Count(h.content, "\n")+1 <= h.scroll+rows {
		return helpTitle, scrolled(h.content, h.scroll, rows)
	}

	shown := strings.Split(scrolled(h.content, h.scroll, max(1, rows-1)), "\n")

	return helpTitle, strings.Join(append(shown, h.ellipsis+" more below"), "\n")
}

// footer is what works while the help is open: close it, or quit.
func (helpOverlay) footer(keys keyMap) []key.Binding {
	return []key.Binding{keys.closeOverlay, keys.quit}
}

// handleKey scrolls the help on a terminal too short to show every key at once,
// or closes it.
func (h helpOverlay) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.toggleHelp, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.scrollDown, m.keys.down):
		h.scroll += m.halfPage()
	case key.Matches(msg, m.keys.scrollUp, m.keys.up):
		h.scroll = max(0, h.scroll-m.halfPage())
	}

	m.overlay = h

	return m, nil
}

// openHelp opens the key list, rendering it from the current bindings.
func (m Model) openHelp() (Model, tea.Cmd) {
	m.overlay = helpOverlay{content: m.helpView(), ellipsis: m.marks.ellipsis}

	return m, nil
}
