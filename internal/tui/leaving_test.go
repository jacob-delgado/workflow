// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A switch to another directory ends the session's work here, so it first
// names what would be lost, and waits for any write still on its way.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// switchingToWeb presses the keys that switch to the web favorite.
func switchingToWeb(t *testing.T, model tui.Model) string {
	t.Helper()

	return typing(t, model, reposKey, "j", keyEnter).View().Content
}

// withDirectories gives repo the Repositories world's directories.
func withDirectories(repo *world) *world {
	repo.dirs = reposWorld().dirs

	return repo
}

// announcing is the world's interface with a post that waited for CI on its
// way to Slack, not yet answered.
func announcing(t *testing.T, repo *world) tui.Model {
	t.Helper()

	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	queued, check := pressed(t, typing(t, repo.live(t, 120, 40), "5", "p"), "w")
	sending, _ := finish(t, queued, check)

	return sending
}

// writingATask is the world's interface with task 3's start sent to
// Taskwarrior, not yet answered.
func writingATask(t *testing.T, repo *world) tui.Model {
	t.Helper()

	writing, _ := pressed(t, typing(t, repo.live(t, 120, 40), tasksPane, downAction), "s")

	return writing
}

func TestASwitchWaitsForAWriteStillOnItsWay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world   func() *world
		started func(*testing.T, *world) tui.Model
		want    string
	}{
		"an announcement": {
			world: newWorld, started: announcing,
			want: "wait for the announcement to finish before switching",
		},
		"a task's change": {
			world: withTasks, started: writingATask,
			want: "wait for the change to a task to finish before switching",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withDirectories(tt.world())
			started := tt.started(t, repo)

			// Act
			view := switchingToWeb(t, started)

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "Switch directory")
		})
	}
}

// draftingABody is the world's interface with a commit body written in the
// editor and the composer closed, its subject never typed.
func draftingABody(t *testing.T, repo *world) tui.Model {
	t.Helper()

	repo.edited = "Tokens reached the log."

	return typing(t, repo.live(t, 120, 40), "3", "c", keyCtrlO, keyEsc)
}

// draftingAPullRequest is the world's interface with a pull request's body
// edited and its composer closed, as edited says.
func draftingAPullRequest(edited bool) func(*testing.T, *world) tui.Model {
	return func(t *testing.T, repo *world) tui.Model {
		t.Helper()

		repo.pullFound, repo.edited = false, "Why this changes."

		keys := []string{"4", "n", keyEsc}
		if edited {
			keys = []string{"4", "n", keyCtrlO, keyEsc}
		}

		return typing(t, repo.live(t, 120, 40), keys...)
	}
}

// waitingForCI is the world's interface with a post waiting for CI to pass.
func waitingForCI(t *testing.T, repo *world) tui.Model {
	t.Helper()

	repo.ci = []forge.CI{{State: forge.CIRunning}}

	return typing(t, repo.live(t, 120, 40), "5", "p", "w")
}

func TestASwitchNamesTheWorkItWouldLose(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		started func(*testing.T, *world) tui.Model
		lost    string
	}{
		"a commit body with no subject":  {started: draftingABody, lost: "the commit message you were writing"},
		"a pull request's edited body":   {started: draftingAPullRequest(true), lost: "the pull request you were writing"},
		"an announcement waiting for CI": {started: waitingForCI, lost: "the announcement waiting for CI"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			started := tt.started(t, withDirectories(newWorld()))

			// Act
			view := switchingToWeb(t, started)

			// Assert
			requireScreen(t, view, "Switch to ~/src/web", "losing:", tt.lost)
		})
	}
}

func TestASwitchLosesNothingOfAPullRequestOnlyOpenedAndClosed(t *testing.T) {
	t.Parallel()

	// Arrange
	started := draftingAPullRequest(false)(t, withDirectories(newWorld()))

	// Act
	view := switchingToWeb(t, started)

	// Assert
	requireScreen(t, view, "Switch to ~/src/web")
	refuseScreen(t, view, "losing:")
}
