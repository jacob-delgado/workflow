// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestTheFooterUsesTheThemeNotFixedGrays(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	model := newWorld().live(t, 120, 40)

	// Act
	footer := footerLine(model.View())

	// Assert
	if !strings.Contains(footer, "\x1b[1m") {
		t.Errorf("the footer draws no bold key:\n%q", footer)
	}

	if !strings.Contains(footer, "\x1b[2m") {
		t.Errorf("the footer draws no faint description:\n%q", footer)
	}
}
