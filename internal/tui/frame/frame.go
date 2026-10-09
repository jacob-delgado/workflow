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

	"charm.land/lipgloss/v2"
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
	if s.ascii() {
		return HeavyASCII
	}

	return Heavy
}

// light is the unfocused weight of the same character set.
func (s Style) light() Style {
	if s.ascii() {
		return LightASCII
	}

	return Light
}

// ascii reports a style drawn in plain ASCII.
func (s Style) ascii() bool {
	return s == LightASCII || s == HeavyASCII
}

// weighed is the glyph set of style's character set at the weight focus
// gives: heavy for the pane with focus, light for every other.
func (s Style) weighed(focused bool) borders {
	if focused {
		return glyphs(s.Heavy())
	}

	return glyphs(s.light())
}

// borders is the glyph set for one weight: the box, and the mark for text cut
// to fit, which has to be ASCII in ASCII mode too.
type borders struct {
	topLeft, topRight, bottomLeft, bottomRight string
	horizontal, vertical                       string
	ellipsis                                   string
}

// glyphs returns the glyph set for a style, and the light one for a Style
// outside the four, so a box is never drawn without its border.
func glyphs(style Style) borders {
	sets := map[Style]borders{
		Light:      {"┌", "┐", "└", "┘", "─", "│", ellipsis},
		Heavy:      {"┏", "┓", "┗", "┛", "━", "┃", ellipsis},
		LightASCII: {"+", "+", "+", "+", "-", "|", asciiEllipsis},
		HeavyASCII: {"#", "#", "#", "#", "=", "#", asciiEllipsis},
	}

	set, known := sets[style]
	if !known {
		return sets[Light]
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

	rows = append(rows, ruleLine(lines.topLeft, lines.topRight, lines, title, inner))

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

	inner := width - minimumSide
	first := style.weighed(panes[0].Focused)
	rows := []string{ruleLine(first.topLeft, first.topRight, first, panes[0].Title, inner)}

	for index, pane := range panes {
		if index > 0 {
			rows = append(rows, railRule(panes[index-1], pane, style, inner))
		}

		rows = append(rows, railContent(pane, inner, style.weighed(pane.Focused))...)
	}

	last := style.weighed(panes[len(panes)-1].Focused)

	return strings.Join(append(rows, last.bottomLeft+strings.Repeat(last.horizontal, inner)+last.bottomRight), "\n")
}

// railContent draws a pane's content rows between its sides.
func railContent(pane RailPane, inner int, lines borders) []string {
	content := strings.Split(pane.Body, "\n")
	rows := make([]string, 0, pane.Rows)

	for index := range pane.Rows {
		text := ""
		if index < len(content) {
			text = content[index]
		}

		rows = append(rows, lines.vertical+padded(text, inner, lines.ellipsis)+lines.vertical)
	}

	return rows
}

// railRule is the shared rule between two panes, carrying the lower one's
// title, drawn heavy when it touches the focused pane.
func railRule(above, below RailPane, style Style, inner int) string {
	left, right := railJunctions(style.weighed(above.Focused), style.weighed(below.Focused))

	return ruleLine(left, right, style.weighed(above.Focused || below.Focused), below.Title, inner)
}

// ruleLine sets a title into a rule in lines' weight between the given left
// and right glyphs. The title is clipped but never padded: the rest of the
// line is rule, not spaces.
func ruleLine(left, right string, lines borders, title string, inner int) string {
	label := ansi.Truncate(lines.horizontal+" "+title+" ", inner, lines.ellipsis)
	fill := strings.Repeat(lines.horizontal, inner-lipgloss.Width(label))

	return left + label + fill + right
}

// railJunctions are the left and right ends of the rule a box drawn in above
// shares with one drawn in below: the glyphs that join their sides. In ASCII
// a focused box keeps its corners where it meets another, as a box drawn on
// its own does.
func railJunctions(above, below borders) (string, string) {
	ends := map[[2]string][2]string{
		{"│", "│"}: {"├", "┤"},
		{"│", "┃"}: {"┢", "┪"},
		{"┃", "│"}: {"┡", "┩"},
		{"┃", "┃"}: {"┣", "┫"},
		{"|", "|"}: {"+", "+"},
		{"|", "#"}: {"#", "#"},
		{"#", "|"}: {"#", "#"},
		{"#", "#"}: {"#", "#"},
	}

	pair := ends[[2]string{above.vertical, below.vertical}]

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
