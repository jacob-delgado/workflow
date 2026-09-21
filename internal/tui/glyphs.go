// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"cmp"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/frame"
)

// glyphs is every character the interface draws with that a terminal or font
// might not have. State is carried by the SHAPE of a glyph rather than its
// color, so it reads in monochrome — and in ASCII, where the shapes differ.
type glyphs struct {
	notStarted, inFlight, done, failed, unknown string
	selected, unselected                        string
	arrow, separator, rule                      string
	ellipsis, ahead, behind                     string
	chosenOpen, chosenClose                     string
	// helpSeparator separates keys in the footer; upKey, downKey and
	// sideways name the arrow keys in it.
	helpSeparator, upKey, downKey, sideways string
	border                                  frame.Style
}

// unicodeGlyphs is the default set.
func unicodeGlyphs() glyphs {
	return glyphs{
		notStarted: "○", inFlight: "◐", done: "●", failed: "✗", unknown: "·",
		selected: "▸ ", unselected: "  ",
		arrow: " → ", separator: " · ", rule: " ─ ",
		ellipsis: "…", ahead: "↑", behind: "↓", chosenOpen: "‹", chosenClose: "›",
		helpSeparator: " • ", upKey: "↑", downKey: "↓", sideways: "←/→",
		border: frame.Light,
	}
}

// asciiGlyphs is the set for ui.ascii.
func asciiGlyphs() glyphs {
	return glyphs{
		notStarted: "o", inFlight: "*", done: "#", failed: "x", unknown: ".",
		selected: "> ", unselected: "  ",
		arrow: " -> ", separator: " - ", rule: " - ",
		ellipsis: "...", ahead: "+", behind: "-", chosenOpen: "<", chosenClose: ">",
		helpSeparator: " | ", upKey: "up", downKey: "down", sideways: "left/right",
		border: frame.LightASCII,
	}
}

// status is the glyph for a Jira status category: not started, in flight, done.
// A category an instance invented gets a neutral mark rather than a guess.
func (g glyphs) status(category jira.StatusCategory) string {
	marks := map[jira.StatusCategory]string{
		jira.CategoryNew: g.notStarted, jira.CategoryIndeterminate: g.inFlight, jira.CategoryDone: g.done,
	}

	return cmp.Or(marks[category], g.unknown)
}

// marker is what starts a row in a list: a pointer at the selected one.
func (g glyphs) marker(selected bool) string {
	if selected {
		return g.selected
	}

	return g.unselected
}

// checkbox marks whether a multi-select row is one of those chosen.
func (g glyphs) checkbox(chosen bool) string {
	if chosen {
		return g.done + " "
	}

	return g.notStarted + " "
}

// styles is the interface's type. It inherits the terminal's own colors for
// everything but meaning: Lip Gloss degrades to plain text where color is not
// available, so there is no capability check here.
type styles struct {
	label  lipgloss.Style
	strong lipgloss.Style
	// failure is red, so a red status always means something broke; the diff
	// preview reuses it where red instead means a removed line.
	failure lipgloss.Style
	// Each system on the spine has its own hue, from the terminal's own
	// palette, so the user's theme chooses the shade.
	jira, git, forge, slack lipgloss.Style
}

// ANSI palette indices for the spine and for failure. Indices, not colors: the
// terminal's theme decides what they look like.
const (
	ansiRed     = "1"
	ansiGreen   = "2"
	ansiYellow  = "3"
	ansiBlue    = "4"
	ansiMagenta = "5"
)

// newStyles builds the styles. With color off, the hues drop to plain while
// bold, faint and the reverse-video cursor — which carry meaning without
// color — stay.
func newStyles(color bool) styles {
	label, strong := lipgloss.NewStyle().Faint(true), lipgloss.NewStyle().Bold(true)

	if !color {
		plain := lipgloss.NewStyle()

		return styles{label: label, strong: strong, failure: plain, jira: plain, git: plain, forge: plain, slack: plain}
	}

	return styles{
		label:   label,
		strong:  strong,
		failure: lipgloss.NewStyle().Foreground(lipgloss.Color(ansiRed)),
		jira:    lipgloss.NewStyle().Foreground(lipgloss.Color(ansiBlue)),
		git:     lipgloss.NewStyle().Foreground(lipgloss.Color(ansiYellow)),
		forge:   lipgloss.NewStyle().Foreground(lipgloss.Color(ansiGreen)),
		slack:   lipgloss.NewStyle().Foreground(lipgloss.Color(ansiMagenta)),
	}
}
