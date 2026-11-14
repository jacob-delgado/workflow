// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The branch is read by eighteen things — every pane's refresh that reads it,
// each write that moves it — so two reads can be out at once. The panes show
// what the read begun last found, and a read a later one superseded says
// nothing at all: not its branch, not its failure, not that it is done.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// otherFeature is a second feature branch, checked out in a shell.
const otherFeature = "fix/PROJ-413-other-work"

func TestAnOlderBranchReadAnsweringLateDoesNotRestoreTheBranchLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the branch, slowly; another branch is then checked out
	// in a shell, and a second refresh finds it and answers first.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "2")
	slow, late := withheldKeys(t, pane, repo.counting("branch"), "r")
	repo.branch = gitrepo.Branch{Name: otherFeature, Base: baseRef}
	newer := typing(t, slow, "r")
	finds := len(repo.asked("find " + featureName))

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, otherFeature)
	refuseScreen(t, view, featureName)

	if again := len(repo.asked("find " + featureName)); again != finds {
		t.Errorf("looked for the pull request of the branch left %d more times", again-finds)
	}
}

func TestABranchReadSupersededAnsweringKeepsTheBranchMarkedInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Review refresh reads the branch, slowly; a Branch refresh reads it
	// again, and its answer is still out when the first one's arrives.
	repo := newWorld()
	review := typing(t, repo.live(t, 120, 40), "4")
	slow, late := withheldKeys(t, review, repo.counting("branch"), "r")
	outstanding, _ := withheldKeys(t, typing(t, slow, "2"), repo.counting("branch"), "r")

	// Act
	view := handedOver(t, outstanding, late).View().Content

	// Assert
	requireScreen(t, view, "Branch ◐")
}

func TestASupersededBranchReadThatFailedSaysNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh's read of the branch fails, slowly; a second refresh reads
	// it and answers first.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "2")
	repo.branchErr = errLocked
	slow, late := withheldKeys(t, pane, repo.counting("branch"), "r")
	repo.branchErr = nil
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	refuseScreen(t, view, "could not read the branch")
}

func TestAFailedBranchRefreshIsSaidBeneathTheBranchItKept(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "2")
	repo.branchErr = errLocked

	// Act
	// The refresh fails, and the next key clears the footer's notice of it.
	view := typing(t, pane, "r", letterTyped).View().Content

	// Assert
	requireScreen(t, view, featureName, "✗ could not read the branch", "index.lock")
}

func TestAFailedBranchRefreshLeadsTheBranchRailWithTheFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	pane := typing(t, repo.live(t, 80, 24), "2")
	repo.branchErr = errLocked

	// Act
	// The refresh fails, and Commits takes the focus, so the Branch rail
	// draws a single row.
	view := typing(t, pane, "r", "3").View().Content

	// Assert
	if row := firstRailRow(t, view, "2 Branch"); !strings.Contains(row, "✗ could not read") {
		t.Errorf("the Branch rail's one row is %q, want the failure:\n%s", row, plain(view))
	}
}

// firstRailRow is the screen's line just below the rail's title, its color
// escapes stripped.
func firstRailRow(t *testing.T, view, title string) string {
	t.Helper()

	lines := strings.Split(plain(view), "\n")
	for index, line := range lines[:len(lines)-1] {
		if strings.Contains(line, title) {
			return lines[index+1]
		}
	}

	t.Fatalf("no line of the screen shows %q:\n%s", title, plain(view))

	return ""
}
