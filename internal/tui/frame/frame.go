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

// Heavy is the focused weight of the same character set.
func (s Style) Heavy() Style {
	if s == LightASCII || s == HeavyASCII {
		return HeavyASCII
	}

	return Heavy
}

// borders is the glyph set for one weight: the box, and the mark for text cut
// to fit, which has to be ASCII in ASCII mode too.
type borders struct {
	topLeft, topRight, bottomLeft, bottomRight string
	horizontal, vertical                       string
	ellipsis                                   string
}

// glyphs returns the glyph set for a style.
func glyphs(style Style) borders {
	//nolint:exhaustive // Light is absent by design; its lookup miss returns the default light border below.
	sets := map[Style]borders{
		Heavy:      {"┏", "┓", "┗", "┛", "━", "┃", ellipsis},
		LightASCII: {"+", "+", "+", "+", "-", "|", asciiEllipsis},
		HeavyASCII: {"#", "#", "#", "#", "=", "#", asciiEllipsis},
	}

	set, ok := sets[style]
	if !ok {
		return borders{"┌", "┐", "└", "┘", "─", "│", ellipsis}
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

// asciiEllipsis is the same mark in plain ASCII.
const asciiEllipsis = "..."

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

		rows = append(rows, lines.vertical+padded(text, inner, lines.ellipsis)+lines.vertical)
	}

	rows = append(rows, lines.bottomLeft+strings.Repeat(lines.horizontal, inner)+lines.bottomRight)

	return strings.Join(rows, "\n")
}

// Plain draws a region with no border at all: the title on the first row and
// the body under it, each row clipped and padded to exactly width. It is for a
// terminal too narrow to give two columns to a border; the style only decides
// how cut text is marked.
func Plain(title, body string, width, height int, style Style) string {
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

		rows = append(rows, fit(text, width, glyphs(style).ellipsis))
	}

	return strings.Join(rows, "\n")
}

// top draws the top border with the title set into it. The title is clipped but
// never padded: the rest of the line is rule, not spaces.
func top(lines borders, title string, inner int) string {
	label := ansi.Truncate(lines.horizontal+" "+title+" ", inner, lines.ellipsis)
	fill := strings.Repeat(lines.horizontal, inner-lipgloss.Width(label))

	return lines.topLeft + label + fill + lines.topRight
}

// padded fits text inside a border with a cell of space on each side, dropping
// the padding only where the box is too narrow to afford it.
func padded(text string, inner int, mark string) string {
	if inner < 2*padding+1 {
		return fit(text, inner, mark)
	}

	space := strings.Repeat(" ", padding)

	return space + fit(text, inner-2*padding, mark) + space
}

// fit clips text to width cells and pads it out to exactly width. It measures
// display cells rather than bytes, so styled and wide text lines up.
func fit(text string, width int, mark string) string {
	clipped := ansi.Truncate(text, width, mark)

	return clipped + strings.Repeat(" ", max(0, width-lipgloss.Width(clipped)))
}

// RailPane is one pane in the shared rail: its title, its body, how many content
// rows it gets, and whether it has focus.
type RailPane struct {
	Title   string
	Body    string
	Rows    int
	Focused bool
}

// RailHeight is the rows a rail of these panes takes: one rule around and
// between them, plus each pane's content rows.
func RailHeight(panes []RailPane) int {
	total := len(panes) + 1
	for _, pane := range panes {
		total += pane.Rows
	}

	return total
}

// Rail draws the stacked panes as one box: a top rule, each pane's content, a
// single shared rule between consecutive panes, and a bottom rule. The focused
// pane's sides and its two rules are drawn heavy, so focus reads by weight
// alone, as a per-pane box did, without a border row between every pair.
func Rail(panes []RailPane, width int, style Style) string {
	if width < minimumSide || len(panes) == 0 {
		return blank(width, RailHeight(panes))
	}

	ascii := style == LightASCII || style == HeavyASCII
	mark := ellipsisFor(ascii)
	inner := width - minimumSide

	rows := []string{railTop(panes[0], inner, ascii, mark)}

	for index, pane := range panes {
		if index > 0 {
			rows = append(rows, railRule(panes[index-1].Focused, pane.Focused, pane.Title, inner, ascii, mark))
		}

		rows = append(rows, railContent(pane, inner, ascii, mark)...)
	}

	return strings.Join(append(rows, railBottom(panes[len(panes)-1].Focused, inner, ascii)), "\n")
}

// ellipsisFor is the mark for cut text in a glyph set.
func ellipsisFor(ascii bool) string {
	if ascii {
		return asciiEllipsis
	}

	return ellipsis
}

// railSide is a pane's vertical border, heavy when it has focus.
func railSide(focused, ascii bool) string {
	switch {
	case ascii && focused:
		return "#"
	case ascii:
		return "|"
	case focused:
		return "┃"
	default:
		return "│"
	}
}

// railHorizontal is a rule's line, heavy when it touches the focused pane.
func railHorizontal(heavy, ascii bool) string {
	switch {
	case ascii && heavy:
		return "="
	case ascii:
		return "-"
	case heavy:
		return "━"
	default:
		return "─"
	}
}

// railContent draws a pane's content rows between its sides.
func railContent(pane RailPane, inner int, ascii bool, mark string) []string {
	side := railSide(pane.Focused, ascii)
	content := strings.Split(pane.Body, "\n")
	rows := make([]string, 0, pane.Rows)

	for index := range pane.Rows {
		text := ""
		if index < len(content) {
			text = content[index]
		}

		rows = append(rows, side+padded(text, inner, mark)+side)
	}

	return rows
}

// railTop is the top edge, carrying the first pane's title.
func railTop(pane RailPane, inner int, ascii bool, mark string) string {
	left, right := railCorners(pane.Focused, ascii, true)

	return ruleLine(left, railHorizontal(pane.Focused, ascii), right, pane.Title, inner, mark)
}

// railBottom is the bottom edge, with no title to carry.
func railBottom(focused bool, inner int, ascii bool) string {
	left, right := railCorners(focused, ascii, false)
	horizontal := railHorizontal(focused, ascii)

	return left + strings.Repeat(horizontal, inner) + right
}

// railRule is the shared rule between two panes, carrying the lower one's title.
func railRule(aboveFocused, belowFocused bool, title string, inner int, ascii bool, mark string) string {
	left, right := railJunctions(aboveFocused, belowFocused, ascii)
	horizontal := railHorizontal(aboveFocused || belowFocused, ascii)

	return ruleLine(left, horizontal, right, title, inner, mark)
}

// ruleLine sets a title into a rule between the given left and right glyphs.
func ruleLine(left, horizontal, right, title string, inner int, mark string) string {
	label := ansi.Truncate(horizontal+" "+title+" ", inner, mark)
	fill := strings.Repeat(horizontal, inner-lipgloss.Width(label))

	return left + label + fill + right
}

// railCorners are the box's corners at the top or the bottom, heavy when that
// pane has focus.
func railCorners(focused, ascii, atTop bool) (string, string) {
	switch {
	case ascii:
		return "+", "+"
	case atTop && focused:
		return "┏", "┓"
	case atTop:
		return "┌", "┐"
	case focused:
		return "┗", "┛"
	default:
		return "└", "┘"
	}
}

// railJunctions are the left and right glyphs of a shared rule, by which of the
// panes it touches have focus.
func railJunctions(aboveFocused, belowFocused, ascii bool) (string, string) {
	if ascii {
		return "+", "+"
	}

	pairs := map[[2]bool][2]string{
		{false, false}: {"├", "┤"},
		{false, true}:  {"┢", "┪"},
		{true, false}:  {"┡", "┩"},
		{true, true}:   {"┣", "┫"},
	}

	pair := pairs[[2]bool{aboveFocused, belowFocused}]

	return pair[0], pair[1]
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
