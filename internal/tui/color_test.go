// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// forceANSI makes lipgloss emit ANSI color, which it does not when a test's
// output is not a terminal, and restores the profile afterward. The tests that
// use it must not run in parallel, so the global profile is theirs alone.
func forceANSI(t *testing.T) func() {
	t.Helper()

	previous := lipgloss.ColorProfile()

	lipgloss.SetColorProfile(termenv.ANSI)

	return func() { lipgloss.SetColorProfile(previous) }
}

// sgrBalance counts the color codes a line opens and the resets that close
// them. Every escape in a rendered view is an SGR; a reset is empty or 0.
func sgrBalance(line string) (int, int) {
	resets := strings.Count(line, "\x1b[0m") + strings.Count(line, "\x1b[m")

	return strings.Count(line, "\x1b[") - resets, resets
}
