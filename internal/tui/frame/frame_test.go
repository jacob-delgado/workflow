// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package frame_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/tui/frame"
)

// Rows and a title the frames below share.
const (
	issuesTitle = "Issues"
	smallTop    = "┌─ x ──────┐"
	smallEmpty  = "│          │"
	smallBottom = "└──────────┘"
)

// lines splits a rendered frame into its rows.
func lines(rendered string) []string {
	return strings.Split(rendered, "\n")
}

func TestRenderDrawsEveryRowAtTheRequestedSize(t *testing.T) {
	t.Parallel()

	// Every row is exactly the requested width, so boxes placed side by side
	// line up without the caller measuring anything — whatever the content.
	cases := map[string]struct {
		title, body   string
		width, height int
	}{
		"a title and a body": {title: issuesTitle, body: "PROJ-1 fix it", width: 20, height: 4},
		"a body too long to fit": {
			title: "x", body: "this line is far too long to fit inside a narrow box", width: 16, height: 3,
		},
		"a title too long to fit": {title: "a title much longer than the box", body: "", width: 12, height: 3},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			rows := lines(frame.Render(tt.title, tt.body, tt.width, tt.height, frame.Light))

			// Assert
			if len(rows) != tt.height {
				t.Fatalf("rendered %d rows, want %d:\n%s", len(rows), tt.height, strings.Join(rows, "\n"))
			}

			for index, row := range rows {
				if width := lipgloss.Width(row); width != tt.width {
					t.Errorf("row %d is %d cells wide, want %d: %q", index, width, tt.width, row)
				}
			}
		})
	}
}

func TestRenderDrawsTheFrameExactly(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		title, body   string
		width, height int
		style         frame.Style
		want          []string
	}{
		// The WHOLE line, not a prefix. A prefix check passed while the rest of
		// the border was being filled with spaces instead of rule, which is what
		// a rendered screen showed and no test did.
		"the title set into an unbroken rule": {
			title: issuesTitle, width: 20, height: 3, style: frame.Light,
			want: []string{"┌─ Issues ─────────┐", "│                  │", "└──────────────────┘"},
		},
		"content padded one cell from each border": {
			title: "x", body: "text", width: 12, height: 3, style: frame.Light,
			want: []string{smallTop, "│ text     │", smallBottom},
		},
		// Clipping silently mid-word reads as the whole value. An ellipsis says
		// there was more.
		"an ellipsis where content is cut": {
			title: "x", body: "https://jira.example.com/jira", width: 16, height: 3, style: frame.Light,
			want: []string{"┌─ x ──────────┐", "│ https://jir… │", "└──────────────┘"},
		},
		// Focus is carried by the SHAPE of the border, not by color, so it
		// survives a monochrome terminal and a colorblind reader.
		"focus in a heavier border": {
			title: "Branch", width: 20, height: 3, style: frame.Heavy,
			want: []string{"┏━ Branch ━━━━━━━━━┓", "┃                  ┃", "┗━━━━━━━━━━━━━━━━━━┛"},
		},
		"rows beyond the box dropped": {
			title: "x", body: "one\ntwo\nthree\nfour", width: 12, height: 4, style: frame.Light,
			want: []string{smallTop, "│ one      │", "│ two      │", smallBottom},
		},
		"empty rows still bordered": {
			title: "x", width: 12, height: 5, style: frame.Light,
			want: []string{smallTop, smallEmpty, smallEmpty, smallEmpty, smallBottom},
		},
		"plain characters in ascii": {
			title: issuesTitle, body: "x", width: 16, height: 3, style: frame.LightASCII,
			want: []string{"+- Issues -----+", "| x            |", "+--------------+"},
		},
		// Focus is still carried by the shape of the line.
		"focus in ascii": {
			title: issuesTitle, body: "x", width: 16, height: 3, style: frame.HeavyASCII,
			want: []string{"#= Issues =====#", "# x            #", "#==============#"},
		},
		"an ascii ellipsis where ascii content is cut": {
			title: "x", body: "a long line of text", width: 12, height: 3, style: frame.LightASCII,
			want: []string{"+- x ------+", "| a lon... |", "+----------+"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			rows := lines(frame.Render(tt.title, tt.body, tt.width, tt.height, tt.style))

			// Assert
			if !slices.Equal(rows, tt.want) {
				t.Errorf("rendered\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestRenderOfABoxTooSmallToBorder(t *testing.T) {
	t.Parallel()

	// A terminal resized to almost nothing must not panic or overflow.
	cases := map[string]struct{ width, height int }{
		"no room at all": {width: 0, height: 0},
		"one column":     {width: 1, height: 5},
		"one row":        {width: 5, height: 1},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			rendered := frame.Render("x", "body", tt.width, tt.height, frame.Light)

			// Assert
			for _, row := range lines(rendered) {
				if width := lipgloss.Width(row); width > tt.width {
					t.Errorf("a %dx%d box drew a row %d cells wide", tt.width, tt.height, width)
				}
			}
		})
	}
}

func TestBodyRowsIsTheHeightInsideTheBorder(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ height, want int }{
		"a tall box":           {height: 10, want: 8},
		"one row inside":       {height: 3, want: 1},
		"only the borders":     {height: 2, want: 0},
		"not even the borders": {height: 1, want: 0},
		"nothing":              {height: 0, want: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := frame.BodyRows(tt.height); got != tt.want {
				t.Errorf("BodyRows(%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

func TestStylesKnowTheirASCIICounterparts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ style, want frame.Style }{
		"light":             {style: frame.Light, want: frame.LightASCII},
		"heavy":             {style: frame.Heavy, want: frame.HeavyASCII},
		"ascii stays ascii": {style: frame.HeavyASCII, want: frame.HeavyASCII},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.style.ASCII(); got != tt.want {
				t.Errorf("ASCII() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStylesKnowTheirHeavyCounterparts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ style, want frame.Style }{
		"light":             {style: frame.Light, want: frame.Heavy},
		"ascii":             {style: frame.LightASCII, want: frame.HeavyASCII},
		"heavy stays heavy": {style: frame.Heavy, want: frame.Heavy},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.style.Heavy(); got != tt.want {
				t.Errorf("Heavy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlainDrawsATitleLineAndNoBorder(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body          string
		width, height int
		style         frame.Style
		want          []string
	}{
		"a title, clipped and padded rows": {
			body: "PROJ-1 a very long summary\nsecond", width: 12, height: 4, style: frame.Light,
			want: []string{"Issues      ", "PROJ-1 a ve…", "second      ", "            "},
		},
		"an ascii ellipsis": {
			body: "a long line of text", width: 8, height: 2, style: frame.HeavyASCII,
			want: []string{"Issues  ", "a lon..."},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			rows := lines(frame.Plain(issuesTitle, tt.body, tt.width, tt.height, tt.style))

			// Assert
			if !slices.Equal(rows, tt.want) {
				t.Errorf("Plain rendered %q, want %q", rows, tt.want)
			}
		})
	}
}

func TestPlainDrawsNothingWithoutRoom(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ width, height int }{
		"no columns": {width: 0, height: 3},
		"no rows":    {width: 5, height: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := frame.Plain("x", "y", tt.width, tt.height, frame.Light); got != "" {
				t.Errorf("Plain drew %q with no room to draw in", got)
			}
		})
	}
}
