// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package frame draws a titled, bordered box of an exact size.
//
// Lip Gloss can draw a border but cannot put a title inside one, and the design
// carries focus in the border itself — a heavier line around the pane that has
// it. So this draws the box by hand, as a pure function whose output a test can
// read row by row.
package frame

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Style is the weight of a border.
type Style int

const (
	// Light is the border of a pane without focus.
	Light Style = iota
	// Heavy is the border of the pane with focus. Focus is carried by the SHAPE
	// of the line, not by its color, so it survives a monochrome terminal and a
	// colorblind reader.
	Heavy
	// LightASCII is Light in plain ASCII, for a terminal or font without
	// box-drawing characters.
	LightASCII
	// HeavyASCII is Heavy in plain ASCII.
	HeavyASCII
)

// ASCII is the same weight of border in plain ASCII.
func (s Style) ASCII() Style {
	if s == Heavy || s == HeavyASCII {
		return HeavyASCII
	}

	return LightASCII
}

// Heavy is the focused weight of the same character set.
func (s Style) Heavy() Style {
	if s == LightASCII || s == HeavyASCII {
		return HeavyASCII
	}

	return Heavy
}

// borders is the glyph set for one weight.
type borders struct {
	topLeft, topRight, bottomLeft, bottomRight string
	horizontal, vertical                       string
}

// glyphs returns the glyph set for a style.
func glyphs(style Style) borders {
	sets := map[Style]borders{
		Heavy:      {"┏", "┓", "┗", "┛", "━", "┃"},
		LightASCII: {"+", "+", "+", "+", "-", "|"},
		HeavyASCII: {"#", "#", "#", "#", "=", "#"},
	}

	set, ok := sets[style]
	if !ok {
		return borders{"┌", "┐", "└", "┘", "─", "│"}
	}

	return set
}

// minimumSide is the smallest dimension that can hold a border on both sides.
const minimumSide = 2

// padding is the space kept between a border and the text inside it.
const padding = 1

// ellipsis marks text that was cut to fit. Clipping silently mid-word reads as
// the whole value; this says there was more.
const ellipsis = "…"

// BodyRows is how many rows of content fit inside a box of the given height — the
// number a caller needs before deciding which slice of a long list to show.
func BodyRows(height int) int {
	return max(0, height-minimumSide)
}

// Render draws a box exactly width cells wide and height rows tall, with the
// title in its top border and body clipped to the space inside.
func Render(title, body string, width, height int, style Style) string {
	if width < minimumSide || height < minimumSide {
		return blank(width, height)
	}

	lines := glyphs(style)
	inner := width - minimumSide
	rows := make([]string, 0, height)

	rows = append(rows, top(lines, title, inner))

	content := strings.Split(body, "\n")

	for index := range height - minimumSide {
		text := ""
		if index < len(content) {
			text = content[index]
		}

		rows = append(rows, lines.vertical+padded(text, inner)+lines.vertical)
	}

	rows = append(rows, lines.bottomLeft+strings.Repeat(lines.horizontal, inner)+lines.bottomRight)

	return strings.Join(rows, "\n")
}

// Plain draws a region with no border at all: the title on the first row and
// the body under it, each row clipped and padded to exactly width. It is for a
// terminal too narrow to give two columns to a border.
func Plain(title, body string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	content := append([]string{title}, strings.Split(body, "\n")...)
	rows := make([]string, 0, height)

	for index := range height {
		text := ""
		if index < len(content) {
			text = content[index]
		}

		rows = append(rows, fit(text, width))
	}

	return strings.Join(rows, "\n")
}

// top draws the top border with the title set into it. The title is clipped but
// never padded: the rest of the line is rule, not spaces.
func top(lines borders, title string, inner int) string {
	label := ansi.Truncate(lines.horizontal+" "+title+" ", inner, ellipsis)
	fill := strings.Repeat(lines.horizontal, inner-lipgloss.Width(label))

	return lines.topLeft + label + fill + lines.topRight
}

// padded fits text inside a border with a cell of space on each side, dropping
// the padding only where the box is too narrow to afford it.
func padded(text string, inner int) string {
	if inner < 2*padding+1 {
		return fit(text, inner)
	}

	space := strings.Repeat(" ", padding)

	return space + fit(text, inner-2*padding) + space
}

// fit clips text to width cells and pads it out to exactly width. It measures
// display cells rather than bytes, so styled and wide text lines up.
func fit(text string, width int) string {
	clipped := ansi.Truncate(text, width, ellipsis)

	return clipped + strings.Repeat(" ", max(0, width-lipgloss.Width(clipped)))
}

// blank fills a space too small to border, so a tiny terminal never panics and
// never draws past the space it was given.
func blank(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	row := strings.Repeat(" ", width)

	return strings.TrimSuffix(strings.Repeat(row+"\n", height), "\n")
}
