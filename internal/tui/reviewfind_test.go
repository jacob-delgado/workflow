// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The branch's pull request is looked for after every branch read, a merge's
// included, so two finds can be out at once for one branch. The Review pane
// shows what the find begun last found, or what was told since: a pull
// request opened or edited here.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// refinedTitle is the pull request's title once it was changed on the forge.
const refinedTitle = pullTitle + " refined"

func TestAnOlderFindAnsweringLateDoesNotRestoreTheOldPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh looks for the pull request, slowly; its title is then changed
	// on the forge, and a second refresh finds it and answers first.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	slow, late := withheldKeys(t, pane, repo.counting("find "), "r")
	repo.pull.Title = refinedTitle
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, refinedTitle)
}

func TestASupersededFindThatFailedSaysNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh's find fails, slowly; a second refresh finds the pull request
	// and answers first.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.pullErr = errForgeDown
	slow, late := withheldKeys(t, pane, repo.counting("find "), "r")
	repo.pullErr = nil
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	refuseScreen(t, view, errForgeDown.Error())
}

func TestAFindOutAcrossAnEditDoesNotRestoreTheOldTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh looks for the pull request, slowly, and its title is edited
	// here before the find answers with the title it had.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	slow, late := withheldKeys(t, pane, repo.counting("find "), "r")
	edited := typing(t, slow, append([]string{"e"}, append(letters(" refined"), keyEnter)...)...)

	// Act
	view := handedOver(t, edited, late).View().Content

	// Assert
	requireScreen(t, view, refinedTitle)
}

func TestASupersededBranchReadThatFailsLooksForNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Review refresh reads the branch, slowly; a Branch refresh reads it
	// again and fails, and the first read's answer arrives last.
	repo := newWorld()
	review := typing(t, repo.live(t, 120, 40), "4")
	finds := len(repo.asked("find "))
	slow, late := withheldKeys(t, review, repo.counting("branch"), "r")
	repo.branchErr = errLocked
	failed := typing(t, slow, "2", "r", "4")

	// Act
	view := handedOver(t, failed, late).View().Content

	// Assert
	if again := len(repo.asked("find ")); again != finds {
		t.Errorf("looked for the pull request %d more times, want none", again-finds)
	}

	refuseScreen(t, view, "Review ◐")
}

func TestAFailedFindOverAPullRequestIsSaidInRailAndDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.pullErr = errForgeDown

	// Act
	view := typing(t, pane, "r").View().Content

	// Assert
	requireScreen(t, view, "#42 "+pullTitle, pullURL)

	if said := strings.Count(view, errForgeDown.Error()); said != 2 {
		t.Errorf("the failure is said %d times, want twice: in the rail and in the detail\n%s", said, view)
	}
}

func TestAFailedFindWithNoPullRequestHeldSaysTheFailureAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withoutPull()
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.pullErr = errForgeDown

	// Act
	view := typing(t, pane, "r").View().Content

	// Assert
	requireScreen(t, view, errForgeDown.Error())
	refuseScreen(t, view, "no pull request yet")
}

func TestAFindAndCIReadFailingAlikeSayTheFailureOnceInRailAndDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge is down: a refresh's find fails, and so does the read of CI
	// it begins for the pull request held.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.pullErr, repo.ciErr = errForgeDown, errForgeDown

	// Act
	view := typing(t, pane, "r").View().Content

	// Assert
	requireScreen(t, view, "#42 "+pullTitle, "CI     ✗ "+errForgeDown.Error())

	if said := strings.Count(plain(view), errForgeDown.Error()); said != 3 {
		t.Errorf("the failure is said %d times, want three: once in the rail, "+
			"and in the detail on its CI line and in one block\n%s", said, plain(view))
	}
}

func TestNoPullRequestIsOfferedWithNoWayToOpenOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch has a commit and no pull request, and the forge cannot
	// open one.
	repo := newWorld()
	repo.pullFound = false
	deps := repo.deps()
	deps.Forge.CreatePullRequest = nil
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "4").View().Content

	// Assert
	requireScreen(t, view, "no pull request yet")
	refuseScreen(t, footerLine(view), "n open")
}
