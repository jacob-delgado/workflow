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

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestAPaneFailureClosesItsColorEachRow(t *testing.T) {
	// Arrange
	defer forceANSI(t)()

	faked := withoutPull()
	faked.pullErr = errLongStatus
	view := typing(t, faked.live(t, 120, 24), "4").View().Content

	// Act
	lines := strings.Split(view, "\n")

	// Assert
	for index, line := range lines {
		if opens, closes := sgrBalance(line); opens != closes {
			t.Errorf("row %d opens %d colors but closes %d, so a color leaks past it:\n%q", index, opens, closes, line)
		}
	}
}
