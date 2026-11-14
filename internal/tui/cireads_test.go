// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CI is read by its poll, by each refresh and on a pull request opening, and
// told running by a re-run asked for here. The pane shows what the read begun
// last found, or what was told since, and lists checks only from a read that
// answered cleanly.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// failedUnlogged is a failed CI of one check whose log the forge does not
// serve: a status reported by something else, which a re-run does not wait
// on.
func failedUnlogged() forge.CI {
	return forge.CI{
		State: forge.CIFailed, Total: 1, Done: 1, Failed: 1,
		Checks: []forge.Check{{ID: "11", Name: rerunCheck, State: forge.CIFailed}},
	}
}

func TestCIReadBegunBeforeAReRunDoesNotPutItBackToFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads CI, slowly, and finds it failed; a re-run is asked for
	// and started before that read answers.
	repo := newWorld()
	repo.ci = []forge.CI{failedUnlogged(), failedUnlogged()}
	pane := typing(t, repo.live(t, 120, 40), "4")
	slow, late := withheldKeys(t, pane, repo.counting("ci "), "r")
	rerun := typing(t, slow, "R", keyEnter)

	// Act
	view := handedOver(t, rerun, late).View().Content

	// Assert
	requireScreen(t, view, "CI     ◐ running")
	refuseScreen(t, view, "CI     ✗ failed")
}

func TestAFailedCIReadOffersNoReRunOfTheChecksItCannotList(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI failed, and a re-run was offered; a refresh's read of it then fails.
	repo := newWorld()
	repo.ci = []forge.CI{failedRun("11")}
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.ciErr = errForgeDown

	// Act
	view := typing(t, pane, "r").View().Content

	// Assert
	requireScreen(t, view, errForgeDown.Error())
	refuseScreen(t, footerLine(view), "re-run")
}

func TestAReadHeldBackForAReRunEndsTheReviewMark(t *testing.T) {
	t.Parallel()

	// Arrange
	// After the re-run, a refresh's read still lists the run that failed: the
	// forge has not started the re-run yet.
	repo := newWorld()
	repo.ci = []forge.CI{failedRun("11"), failedRun("11")}
	rerun := typing(t, repo.live(t, 120, 40), "4", "R", keyEnter)
	reads := len(repo.asked("ci "))

	// Act
	view := typing(t, rerun, "r").View().Content

	// Assert
	if again := len(repo.asked("ci ")); again == reads {
		t.Errorf("the refresh read CI %d more times, want a read to hold back", again-reads)
	}

	requireScreen(t, view, "CI     ◐ running")
	refuseScreen(t, view, "Review ◐")
}

func TestAFailedCIReadListsNoCheckAndOffersNoChecksKey(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI failed on a check the pane listed; a refresh's read of it then fails.
	repo := newWorld()
	repo.ci = []forge.CI{failedRun("11")}
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.ciErr = errForgeDown

	// Act
	view := typing(t, pane, "r").View().Content

	// Assert
	refuseScreen(t, view, rerunCheck)
	refuseScreen(t, footerLine(view), "c checks")
}

func TestAReadHeldBackForAReRunThatALaterReadSupersededChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// After the re-run, a refresh reads CI, slowly; a second refresh's read
	// answers first, and both still list the run that failed.
	repo := newWorld()
	repo.ci = []forge.CI{failedRun("11"), failedRun("11"), failedRun("11")}
	rerun := typing(t, repo.live(t, 120, 40), "4", "R", keyEnter)
	slow, late := withheldKeys(t, rerun, repo.counting("ci "), "r")
	newer := typing(t, slow, "r")
	reads := len(repo.asked("ci "))

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	if again := len(repo.asked("ci ")); again != reads {
		t.Errorf("the superseded answer read CI %d more time(s), want none", again-reads)
	}

	requireScreen(t, view, "CI     ◐ running")
	refuseScreen(t, view, "Review ◐")
}

func TestWithNoWayToOpenAPageTheChecksAreNotOffered(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{failedRun("11")}
	deps := repo.deps()
	deps.OpenURL = nil
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "4").View().Content

	// Assert
	requireScreen(t, view, rerunCheck)
	refuseScreen(t, footerLine(view), "c checks")
}
