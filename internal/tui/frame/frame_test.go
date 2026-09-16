// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package frame_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/jacob-delgado/workflow/internal/tui/frame"
)

// lines splits a rendered frame into its rows.
func lines(rendered string) []string {
	return strings.Split(rendered, "\n")
}

func TestRenderDrawsAnExactlySizedBox(t *testing.T) {
	t.Parallel()

	rendered := frame.Render("Issues", "PROJ-1 fix it", 20, 4, frame.Light)
	rows := lines(rendered)

	if len(rows) != 4 {
		t.Fatalf("rendered %d rows, want 4:\n%s", len(rows), rendered)
	}

	// Every row is exactly the requested width, so boxes placed side by side
	// line up without the caller measuring anything.
	for index, row := range rows {
		if width := lipgloss.Width(row); width != 20 {
			t.Errorf("row %d is %d cells wide, want 20: %q", index, width, row)
		}
	}
}

func TestRenderPutsTheTitleInTheTopBorder(t *testing.T) {
	t.Parallel()

	rows := lines(frame.Render("Issues", "", 20, 3, frame.Light))

	// The WHOLE line, not a prefix. A prefix check passed while the rest of the
	// border was being filled with spaces instead of rule, which is what a
	// rendered screen showed and no test did.
	if rows[0] != "┌─ Issues ─────────┐" {
		t.Errorf("top border = %q, want the title set into an unbroken rule", rows[0])
	}
}

func TestRenderPadsContentAwayFromTheBorder(t *testing.T) {
	t.Parallel()

	rows := lines(frame.Render("x", "text", 12, 3, frame.Light))

	if rows[1] != "│ text     │" {
		t.Errorf("body row = %q, want one cell of padding inside each border", rows[1])
	}
}

func TestRenderMarksTruncatedContent(t *testing.T) {
	t.Parallel()

	rows := lines(frame.Render("x", "https://jira.example.com/jira", 16, 3, frame.Light))

	// Clipping silently mid-word reads as the whole value. An ellipsis says
	// there was more.
	if !strings.Contains(rows[1], "…") {
		t.Errorf("clipped row = %q, want an ellipsis marking the cut", rows[1])
	}
}

func TestRenderDistinguishesFocusByBorderWeight(t *testing.T) {
	t.Parallel()

	// Focus is carried by the SHAPE of the border, not by color, so it survives
	// a monochrome terminal and a colorblind reader.
	light := lines(frame.Render("Branch", "", 20, 3, frame.Light))
	heavy := lines(frame.Render("Branch", "", 20, 3, frame.Heavy))

	if !strings.HasPrefix(light[0], "┌") || !strings.HasPrefix(light[2], "└") {
		t.Errorf("light frame corners = %q / %q", light[0], light[2])
	}

	if !strings.HasPrefix(heavy[0], "┏━ Branch ") || !strings.HasPrefix(heavy[2], "┗") {
		t.Errorf("heavy frame = %q / %q, want heavy corners and rule", heavy[0], heavy[2])
	}
}

func TestRenderTruncatesContentToFit(t *testing.T) {
	t.Parallel()

	long := "this line is far too long to fit inside a narrow box"
	rows := lines(frame.Render("x", long, 16, 3, frame.Light))

	if width := lipgloss.Width(rows[1]); width != 16 {
		t.Errorf("an overlong row is %d cells wide, want it clipped to 16", width)
	}
}

func TestRenderTruncatesAnOverlongTitle(t *testing.T) {
	t.Parallel()

	rows := lines(frame.Render("a title much longer than the box", "", 12, 3, frame.Light))

	if width := lipgloss.Width(rows[0]); width != 12 {
		t.Errorf("the top border is %d cells wide, want 12: %q", width, rows[0])
	}
}

func TestRenderDropsRowsThatDoNotFit(t *testing.T) {
	t.Parallel()

	body := "one\ntwo\nthree\nfour"
	rows := lines(frame.Render("x", body, 12, 4, frame.Light))

	if len(rows) != 4 {
		t.Fatalf("rendered %d rows, want exactly 4", len(rows))
	}

	if strings.Contains(strings.Join(rows, "\n"), "three") {
		t.Error("a row beyond the box's height was drawn")
	}
}

func TestRenderPadsShortContent(t *testing.T) {
	t.Parallel()

	rows := lines(frame.Render("x", "", 12, 5, frame.Light))

	// Three empty body rows, each still bordered on both sides.
	for _, row := range rows[1:4] {
		if !strings.HasPrefix(row, "│") || !strings.HasSuffix(row, "│") {
			t.Errorf("an empty body row lost its borders: %q", row)
		}
	}
}

func TestRenderOfABoxTooSmallToBorder(t *testing.T) {
	t.Parallel()

	// A terminal resized to almost nothing must not panic or overflow.
	for _, size := range [][2]int{{0, 0}, {1, 5}, {5, 1}} {
		rendered := frame.Render("x", "body", size[0], size[1], frame.Light)

		for _, row := range lines(rendered) {
			if width := lipgloss.Width(row); width > size[0] {
				t.Errorf("a %dx%d box drew a row %d cells wide", size[0], size[1], width)
			}
		}
	}
}

func TestBodyRowsIsTheHeightInsideTheBorder(t *testing.T) {
	t.Parallel()

	cases := map[int]int{10: 8, 3: 1, 2: 0, 1: 0, 0: 0}

	for height, want := range cases {
		if got := frame.BodyRows(height); got != want {
			t.Errorf("BodyRows(%d) = %d, want %d", height, got, want)
		}
	}
}

func TestASCIIStylesDrawWithPlainCharactersAndStillShowFocus(t *testing.T) {
	t.Parallel()

	light := lines(frame.Render("Issues", "x", 16, 3, frame.LightASCII))
	heavy := lines(frame.Render("Issues", "x", 16, 3, frame.HeavyASCII))

	if light[0] != "+- Issues -----+" || light[1] != "| x            |" || light[2] != "+--------------+" {
		t.Errorf("light ASCII frame =\n%s", strings.Join(light, "\n"))
	}

	// Focus is still carried by the shape of the line.
	if heavy[0] != "#= Issues =====#" || heavy[1] != "# x            #" || heavy[2] != "#==============#" {
		t.Errorf("heavy ASCII frame =\n%s", strings.Join(heavy, "\n"))
	}
}

func TestStylesKnowTheirASCIIAndHeavyCounterparts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		got, want frame.Style
	}{
		"light to ascii":    {got: frame.Light.ASCII(), want: frame.LightASCII},
		"heavy to ascii":    {got: frame.Heavy.ASCII(), want: frame.HeavyASCII},
		"ascii stays ascii": {got: frame.HeavyASCII.ASCII(), want: frame.HeavyASCII},
		"heavy of light":    {got: frame.Light.Heavy(), want: frame.Heavy},
		"heavy of ascii":    {got: frame.LightASCII.Heavy(), want: frame.HeavyASCII},
		"heavy stays heavy": {got: frame.Heavy.Heavy(), want: frame.Heavy},
	}

	for name, tt := range cases {
		if tt.got != tt.want {
			t.Errorf("%s: got %v, want %v", name, tt.got, tt.want)
		}
	}
}

func TestPlainDrawsATitleLineAndNoBorder(t *testing.T) {
	t.Parallel()

	rendered := frame.Plain("Issues", "PROJ-1 a very long summary\nsecond", 12, 4)
	rows := lines(rendered)

	want := []string{"Issues      ", "PROJ-1 a ve…", "second      ", "            "}
	if len(rows) != len(want) {
		t.Fatalf("Plain rendered %d rows, want %d:\n%s", len(rows), len(want), rendered)
	}

	for index := range want {
		if rows[index] != want[index] {
			t.Errorf("row %d = %q, want %q", index, rows[index], want[index])
		}
	}

	if frame.Plain("x", "y", 0, 3) != "" || frame.Plain("x", "y", 5, 0) != "" {
		t.Error("Plain drew something with no room to draw in")
	}
}
