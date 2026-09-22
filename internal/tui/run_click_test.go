// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A click below a wrapped jobs line must skip every row the line took, not one,
// so the place clicked is the place selected.

import (
	"strings"
	"testing"
)

// firstFailureLine and secondFailureLine are lint output the commit fakes across
// these tests share; named here so the shared literals do not trip goconst.
const (
	firstFailureLine  = "a.go:1:1: first"
	secondFailureLine = "b.go:2:1: second"
)

func TestClickingBelowWrappedJobsSelectsTheRightFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// Four long job names make the jobs line wrap to more than one row, with three
	// places to jump to below it.
	wrapping := newWorld()
	wrapping.commitErr = errHookFailed
	wrapping.commitLines = []string{
		"┃ verylongjobname-alpha ❯", "┃ verylongjobname-bravo ❯",
		"┃ verylongjobname-charlie ❯", "┃ verylongjobname-delta ❯",
		firstFailureLine, secondFailureLine, "c.go:3:1: third",
	}
	failed := typing(t, wrapping.live(t, 120, 40), commitKeys("x")...)

	// The second place is not selected yet; find the screen row it renders on.
	row := screenRow(t, failed.View().Content, "b.go:2 second")

	// Act
	picked := click(t, failed, 60, row)

	// Assert
	requireScreen(t, picked.View().Content, "▸ b.go:2 second")
}

// screenRow is the row a phrase renders on, so a click can land on it without
// counting the header the jobs line wrapped into.
func screenRow(t *testing.T, view, phrase string) int {
	t.Helper()

	for row, line := range strings.Split(plain(view), "\n") {
		if strings.Contains(line, phrase) {
			return row
		}
	}

	t.Fatalf("the screen does not show %q:\n%s", phrase, plain(view))

	return -1
}
