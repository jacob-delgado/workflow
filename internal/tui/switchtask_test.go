// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// otherTaskBranch is a second issue's branch, one the switcher should offer.
const otherTaskBranch = "fix/PROJ-388-add-retries"

// switchIntro opens the switcher's body, whichever state it is in.
const switchIntro = "Switch to another task"

// Errors git answers the switcher with.
var (
	errNoGitRepo      = errors.New("fatal: not a git repository")
	errWouldOverwrite = errors.New("fatal: local changes would be overwritten")
)

// cleanSwitcher is a world on a clean tree with a second task branch to switch
// to.
func cleanSwitcher() *world {
	repo := newWorld()
	repo.changes = nil
	repo.branches = []string{featureName, otherTaskBranch}

	return repo
}

// openSwitcher focuses the Branch pane, opens the switcher and delivers the
// branch listing.
func openSwitcher(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	loading, cmd := pressed(t, press(t, model, "2"), "s")
	listed, _ := finish(t, loading, cmd)

	return listed
}

func TestSwitchingTaskListsIssueBranchesAndChecksOneOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.changes = nil // a clean tree, so a switch carries nothing across
	repo.branches = []string{featureName, otherTaskBranch, "chore/tidy", baseName}
	model := repo.live(t, 120, 40)

	// Act: open the switcher on the Branch pane
	opened := typing(t, model, "2", "s")

	// Assert: it lists the other issue's branch, named by its issue
	requireScreen(t, opened.View().Content, "PROJ-388", "Add retries")

	// Act: check it out
	switched := typing(t, opened, keyEnter)

	// Assert: git switched to the chosen branch and the interface says so
	if got := repo.asked("checkout"); len(got) != 1 || got[0] != "checkout "+otherTaskBranch {
		t.Errorf("checkout calls = %v, want one for the chosen branch", got)
	}

	requireScreen(t, switched.View().Content, "switched to "+otherTaskBranch)
}

func TestSwitchingTaskReloadsThePanesForTheNewBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	model := repo.live(t, 120, 40)

	// Act
	typing(t, model, "2", "s", keyEnter)

	// Assert
	if changes := repo.asked("changes"); len(changes) < 2 {
		t.Errorf("changes reads = %v, want another after the switch", changes)
	}
}

func TestSwitchingTaskRefusesADirtyTree(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld() // the default world has an uncommitted change
	repo.branches = []string{featureName, otherTaskBranch}
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "s", keyEnter).View().Content

	// Assert
	if got := repo.asked("checkout"); len(got) != 0 {
		t.Errorf("checkout calls = %v, want none on a dirty tree", got)
	}

	requireScreen(t, view, "commit or stash")
}

func TestSwitchingTaskSaysWhenThereIsNowhereToSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld() // only the current branch and non-issue branches exist
	repo.changes = nil
	repo.branches = []string{featureName, baseName, "chore/tidy"}
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "s", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "No other task branch")

	if got := repo.asked("checkout"); len(got) != 0 {
		t.Errorf("checkout calls = %v, want none with nowhere to switch", got)
	}
}

func TestSwitchingTaskShowsWhyTheBranchesCouldNotBeListed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branchesErr = errNoGitRepo

	// Act
	view := openSwitcher(t, repo.live(t, 120, 40)).View().Content

	// Assert
	requireScreen(t, view, switchIntro, "not a git repository")
}

func TestTheFooterOffersOnlyQuittingWhileASwitchIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	screen := openSwitcher(t, cleanSwitcher().live(t, 120, 40))

	// Act
	sending, _ := pressed(t, screen, keyEnter) // enter, without letting the checkout finish

	// Assert
	footer := footerLine(sending.View().Content)
	requireScreen(t, footer, "quit")
	refuseScreen(t, footer, "apply", keyEsc)
	requireScreen(t, sending.View().Content, "switching")
}

func TestNothingInterruptsASwitchBeingSent(t *testing.T) {
	t.Parallel()

	// Nothing may switch twice, and nothing may hide an answer still to come.
	for _, stroke := range []string{keyEnter, keyEsc} {
		t.Run(stroke, func(t *testing.T) {
			t.Parallel()

			// Arrange
			sending, _ := pressed(t, openSwitcher(t, cleanSwitcher().live(t, 120, 40)), keyEnter)

			// Act
			after, cmd := pressed(t, sending, stroke)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command while the switch is sent", stroke)
			}

			if after.View().Content != sending.View().Content {
				t.Errorf("%s changed the screen while the switch is sent", stroke)
			}
		})
	}
}

func TestAFailedSwitchKeepsTheSwitcherOpenToTryAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.checkoutErr = errWouldOverwrite
	screen := openSwitcher(t, repo.live(t, 120, 40))

	// Act: git refuses the switch
	refused := typing(t, screen, keyEnter)

	// Assert: the switcher stays open with git's reason
	requireScreen(t, refused.View().Content, switchIntro, "would be overwritten")

	// Act: try again
	typing(t, refused, keyEnter)

	// Assert: the switch was attempted twice
	if got := repo.asked("checkout"); len(got) != 2 {
		t.Errorf("checkout calls = %v, want the retry sent", got)
	}
}

func TestBranchesArrivingAfterTheSwitcherClosesDoNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	loading, cmd := pressed(t, press(t, cleanSwitcher().live(t, 120, 40), "2"), "s")

	// Act
	closed := press(t, loading, keyEsc) // close the switcher before the listing arrives
	settled, _ := finish(t, closed, cmd)

	// Assert
	refuseScreen(t, settled.View().Content, switchIntro)
}

func TestTheBranchPaneOffersSwitchingTasks(t *testing.T) {
	t.Parallel()

	// Arrange
	model := cleanSwitcher().live(t, 120, 40)

	// Act
	view := typing(t, model, "2").View().Content

	// Assert
	requireScreen(t, footerLine(view), "s switch task")
}

// A dry run switches nothing and says what it would have done.
func TestSwitchingTaskUnderDryRunSwitchesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	model := sized(t, dryInterface(repo), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "s", keyEnter).View().Content

	// Assert
	if got := repo.asked("checkout"); len(got) != 0 {
		t.Errorf("checkout calls = %v, want none under a dry run", got)
	}

	requireScreen(t, view, "dry run", otherTaskBranch)
}
