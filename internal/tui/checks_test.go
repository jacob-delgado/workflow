// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errNoOpener is what the opener answers when the platform's browser command is
// not installed.
var errNoOpener = errors.New(`exec: "xdg-open": executable file not found in $PATH`)

// checkedCI is a world whose CI reports two named checks, one failed.
func checkedCI() *world {
	repo := newWorld()
	repo.ci = []forge.CI{{
		State: forge.CIFailed, Total: 2, Done: 2, Failed: 1,
		Checks: []forge.Check{
			{Name: "lint", State: forge.CIFailed, URL: "https://ci/lint"},
			{Name: "build", State: forge.CIPassed, URL: "https://ci/build"},
		},
	}}

	return repo
}

func TestTheChecksListShowsEachCheckAndOpensOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := checkedCI()
	model := repo.live(t, 120, 40)

	// Act: focus Review and open the checks
	opened := typing(t, model, "4", "c")

	// Assert: both checks are listed
	requireScreen(t, opened.View(), "lint", "build")

	// Act: open the selected check's page
	browsed := typing(t, opened, keyEnter)

	// Assert: the forge page for the first check was opened, and it says so
	if got := repo.asked("browse https://ci/lint"); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the selected check", got)
	}

	requireScreen(t, browsed.View(), "opened lint")
}

func TestTheChecksListMovesTheSelectionBeforeOpening(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := checkedCI()
	model := repo.live(t, 120, 40)

	// Act
	typing(t, model, "4", "c", "j", keyEnter) // open the checks, move to the second, open it

	// Assert
	if got := repo.asked("browse https://ci/build"); len(got) != 1 {
		t.Errorf("browse calls = %v, want the second check opened", got)
	}
}

func TestTheReviewPaneOffersTheChecksKey(t *testing.T) {
	t.Parallel()

	// Arrange
	model := checkedCI().live(t, 120, 40)

	// Act
	view := typing(t, model, "4").View()

	// Assert
	requireScreen(t, footerLine(view), "c checks")
}

func TestAFailedOpenShowsTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := checkedCI()
	repo.openURLErr = errNoOpener
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "4", "c", keyEnter).View()

	// Assert
	requireScreen(t, view, "executable file not found")
}

func TestTheChecksListNavigatesClosesAndShowsEveryState(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{{
		State: forge.CIRunning,
		Checks: []forge.Check{
			{Name: "lint", State: forge.CIFailed, URL: "https://ci/lint"},
			{Name: "vet", State: forge.CIRunning, URL: "https://ci/vet"},
			{Name: "cache", State: forge.CINone, URL: "https://ci/cache"},
		},
	}}
	model := repo.live(t, 120, 40)

	// Act: open the checks
	onChecks := typing(t, model, "4", "c")

	// Assert: every check is drawn, whatever its state
	requireScreen(t, onChecks.View(), "lint", "vet", "cache")

	// Act: move down, back up, press a key it ignores, then close
	closed := typing(t, onChecks, "j", "k", "z", keyEsc)

	// Assert: the list is gone and nothing was opened
	refuseScreen(t, closed.View(), "Open a check's page")

	if got := repo.asked("browse"); len(got) != 0 {
		t.Errorf("browse calls = %v, want none after only navigating", got)
	}
}

func TestOpeningAfterTheChecksCloseIsIgnored(t *testing.T) {
	t.Parallel()

	// Arrange
	onChecks := typing(t, checkedCI().live(t, 120, 40), "4", "c")

	// Act
	opening, cmd := pressed(t, onChecks, keyEnter) // dispatch the open
	closed := press(t, opening, keyEsc)            // close the list before the result arrives
	settled, _ := finish(t, closed, cmd)

	// Assert
	refuseScreen(t, settled.View(), "Open a check's page")
}

func TestACheckWithNoPageSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{{
		State:  forge.CIRunning,
		Checks: []forge.Check{{Name: "queued", State: forge.CIRunning, URL: ""}},
	}}
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "4", "c", keyEnter).View()

	// Assert
	requireScreen(t, view, "no page to open")

	if got := repo.asked("browse"); len(got) != 0 {
		t.Errorf("browse calls = %v, want none for a check with no page", got)
	}
}
