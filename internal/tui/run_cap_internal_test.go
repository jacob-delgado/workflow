// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// A run's kept output is bounded, so a hook that prints without end does not
// grow the model without end. This reaches capLines directly.

import (
	"strconv"
	"testing"
)

func TestCapLinesKeepsTheTailPastTheLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	lines := make([]string, maxRunLines+10)
	for index := range lines {
		lines[index] = strconv.Itoa(index)
	}

	// Act
	capped := capLines(lines)

	// Assert
	if len(capped) != maxRunLines || capped[0] != strconv.Itoa(10) {
		t.Errorf("capLines kept %d lines starting %q, want the last %d starting %q",
			len(capped), capped[0], maxRunLines, "10")
	}
}

func TestCapLinesKeepsEverythingUnderTheLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	lines := []string{"one", "two", "three"}

	// Act
	capped := capLines(lines)

	// Assert
	if len(capped) != len(lines) {
		t.Errorf("capLines kept %d of %d lines under the limit", len(capped), len(lines))
	}
}
