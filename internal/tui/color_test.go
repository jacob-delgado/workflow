// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// failGlyph is the failure mark the default (unicode) glyph set draws.
const failGlyph = "✗"

// redOpen is the escape lipgloss writes to open the failure color, learned from
// the style itself rather than hardcoded. It needs forceANSI to be in effect.
func redOpen() string {
	open, _, _ := strings.Cut(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(failGlyph), failGlyph)

	return open
}

// forceANSI is the seam the color tests call to make lipgloss emit color. In v2
// a style renders its colors into the string whether or not the output is a
// terminal — downsampling happens at the program's writer, not at Render — so
// there is no global profile to force, and this is a no-op that returns a no-op
// restore.
func forceANSI(t *testing.T) func() {
	t.Helper()

	return func() {}
}

// sgrBalance counts the color codes a line opens and the resets that close
// them. Every escape in a rendered view is an SGR; a reset is empty or 0.
func sgrBalance(line string) (int, int) {
	resets := strings.Count(line, "\x1b[0m") + strings.Count(line, "\x1b[m")

	return strings.Count(line, "\x1b[") - resets, resets
}
