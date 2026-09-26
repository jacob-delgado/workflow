// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"
)

// errLongStatus is a forge failure long enough to wrap across several rows, so
// a color that leaked past a row's end would show.
var errLongStatus = errors.New("reading the status of /very/long/path: git: exit status 128: " +
	"fatal: not a git repository (or any of the parent directories): .git and yet more words to spill")

func TestAPaneFailureClosesItsColorEachRow(t *testing.T) {
	t.Parallel()

	// Arrange
	faked := withoutPull()
	faked.pullErr = errLongStatus

	// Act
	view := typing(t, faked.live(t, 120, 24), "4").View().Content

	// Assert
	for index, line := range strings.Split(view, "\n") {
		if opens, closes := sgrBalance(line); opens != closes {
			t.Errorf("row %d opens %d colors but closes %d, so a color leaks past it:\n%q", index, opens, closes, line)
		}
	}
}
