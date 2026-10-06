// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// savedSettingsPath is where settingsFile is kept, from ana's home.
const savedSettingsPath = "~/src/api/.workflow.json"

// draftingCommit is the session with a commit message being written, which
// reopening would lose.
func draftingCommit(t *testing.T, working *world) tui.Model {
	t.Helper()

	return typing(t, working.live(t, 120, 40), append(append([]string{"3", "c"}, letters("wip")...), keyEsc)...)
}

// savedAndEnded is the session after a save that nothing held back from
// reopening.
func savedAndEnded(t *testing.T) tui.Model {
	t.Helper()

	changed := typing(t, reposWorld().live(t, 120, 40), edited(projectRow, editedProject)...)
	saved, _ := pressedAndAnswered(t, changed, saveKey)

	return saved
}

func TestASaveReopensWorkflowWhereYouWork(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	changed := typing(t, working.live(t, 120, 40), edited(projectRow, editedProject)...)

	// Act
	saved, cmd := pressedAndAnswered(t, changed, saveKey)

	// Assert
	// The interface ends and says to open again where it was, as a switch
	// does, so the configuration saved is the one it is wired with.
	if !quits(cmd) || saved.Destination().Dir != apiCmd {
		t.Errorf("a save: quit %v, destination %q; want the program ended for %s",
			quits(cmd), saved.Destination().Dir, apiCmd)
	}
}

func TestASaveThatWouldLoseACommitMessageAsksBeforeReopening(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	changed := typing(t, draftingCommit(t, working), edited(projectRow, editedProject)...)

	// Act
	asked, cmd := pressedAndAnswered(t, changed, saveKey)

	// Assert
	if quits(cmd) || asked.Destination().Dir != "" {
		t.Fatalf("reopened for %q at once, want to be asked first", asked.Destination().Dir)
	}

	if working.settings.Jira.Project != editedProject {
		t.Errorf("saved project %q, want the file written before asking", working.settings.Jira.Project)
	}

	requireScreen(t, asked.View().Content, "Reopen", "the commit message you were writing")
}

func TestAskedBeforeReopeningEnterReopens(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	asked := typing(t, draftingCommit(t, working), editing(projectRow, editedProject)...)

	// Act
	left, cmd := pressed(t, asked, keyEnter)

	// Assert
	if !quits(cmd) || left.Destination().Dir != apiCmd {
		t.Errorf("enter on the question: quit %v, destination %q; want %s", quits(cmd), left.Destination().Dir, apiCmd)
	}
}

func TestAskedBeforeReopeningEscStaysAndSaysWhenItApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	asked := typing(t, draftingCommit(t, working), editing(projectRow, editedProject)...)

	// Act
	stayed, cmd := pressed(t, asked, keyEsc)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Fatalf("esc on the question reopened for %q", stayed.Destination().Dir)
	}

	requireScreen(t, stayed.View().Content, "saved "+savedSettingsPath, "applies once workflow reopens")
}

func TestReopeningAfterASaveSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	saved := savedAndEnded(t)

	// Act
	reopened := reposWorld().live(t, 120, 40).Arrived(saved.Destination())

	// Assert
	view := reopened.View().Content
	requireScreen(t, view, "saved "+savedSettingsPath, "reopened with it")
	refuseScreen(t, view, "switched to")
}

func TestAReopenThatFailsSaysTheSaveIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	saved := savedAndEnded(t)

	// Act
	stayed := reposWorld().live(t, 120, 40).StayedAfter(saved.Destination(), errJiraDown)

	// Assert
	requireScreen(t, stayed.View().Content, "saved "+savedSettingsPath+" but could not reopen with it", "jira is down")
}
