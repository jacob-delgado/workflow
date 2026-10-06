// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

// withSyncedTasks is withTasks with a backend to sync with.
func withSyncedTasks() *world {
	repo := withTasks()
	repo.tasks.install.SyncConfigured = true

	return repo
}

func TestATaskWriteThatCannotBeTakenBackWaitsOnALastLook(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys  []string
		asks  []string
		write string
		want  string
	}{
		"mark done": {
			keys: []string{tasksPane, downAction, "d"}, asks: []string{"Mark the task done", "Mark task 3", "done?"},
			write: "task done " + trackedTaskUUID, want: "● marked 3 done",
		},
		"undo": {
			keys: []string{tasksPane, "u"}, asks: []string{"Undo in Taskwarrior", "Undo Taskwarrior's last change?"},
			write: "task undo", want: "● undone",
		},
		"sync": {
			keys: []string{tasksPane, "S"}, asks: []string{"Sync Taskwarrior", "Sync Taskwarrior with its server?"},
			write: syncWrite, want: "● synced",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withSyncedTasks()

			// Act: press the key
			asked := typing(t, repo.live(t, 120, 40), tt.keys...)

			// Assert: the look asks, and Taskwarrior is asked nothing yet
			requireScreen(t, asked.View().Content, tt.asks...)
			requireTaskWrites(t, repo)

			// Act: go ahead
			view := typing(t, asked, keyEnter).View().Content

			// Assert: the write is sent and told
			requireTaskWrites(t, repo, tt.write)
			requireScreen(t, view, tt.want)
		})
	}
}

func TestEscOnATaskWritesLastLookSendsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withSyncedTasks()
	asked := typing(t, repo.live(t, 120, 40), tasksPane, downAction, "d")

	// Act
	view := typing(t, asked, keyEsc).View().Content

	// Assert
	requireTaskWrites(t, repo)
	refuseScreen(t, view, "Mark the task done")
}

func TestForgettingAPersonWaitsOnALastLook(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()

	// Act: press d
	asked := typing(t, openPeople(t, ben), "d")

	// Assert: the look names what forgetting costs, and nothing is forgotten
	requireScreen(t, asked.View().Content, "Forget ben?", "asks whom they are on Slack again")

	if calls := ben.asked("forget-owner "); len(calls) != 0 {
		t.Fatalf("forgot %q before the look was confirmed", calls)
	}

	// Act: go ahead
	forgotten := typing(t, asked, keyEnter)

	// Assert: ben is forgotten, back in People and groups
	if calls := ben.asked("forget-owner "); len(calls) != 1 {
		t.Errorf("forgets = %q, want ben once", calls)
	}

	requireScreen(t, forgotten.View().Content, "People and groups")
}

func TestEscOnForgetGoesBackToPeopleAndGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	asked := typing(t, openPeople(t, ben), "d")

	// Act
	view := typing(t, asked, keyEsc).View().Content

	// Assert
	if calls := ben.asked("forget-owner "); len(calls) != 0 {
		t.Errorf("forgot %q after esc", calls)
	}

	requireScreen(t, view, "People and groups", "ben")
	refuseScreen(t, view, "Forget ben?")
}

func TestEnterOnAFavoriteAsksBeforeSwitching(t *testing.T) {
	t.Parallel()

	// Act
	asked, cmd := pressed(t, onWeb(t, reposWorld()), keyEnter)

	// Assert
	if quits(cmd) || asked.Destination().Dir != "" {
		t.Fatalf("left for %q at once, want a last look first", asked.Destination().Dir)
	}

	requireScreen(t, asked.View().Content, "Switch to ~/src/web?", "enter switch", "esc stay")
}

func TestEscOnTheSwitchLookStays(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := typing(t, onWeb(t, reposWorld()), keyEnter)

	// Act
	stayed, cmd := pressed(t, asked, keyEsc)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("esc on the look left for %q", stayed.Destination().Dir)
	}

	refuseScreen(t, stayed.View().Content, "Switch to ~/src/web?")
}

func TestGoToAsksBeforeSwitching(t *testing.T) {
	t.Parallel()

	// Arrange
	typed := goingTo(t, reposWorld(), "~/src/web")

	// Act
	asked, cmd := pressedAndAnswered(t, typed, keyEnter)

	// Assert
	if quits(cmd) || asked.Destination().Dir != "" {
		t.Fatalf("left for %q at once, want a last look first", asked.Destination().Dir)
	}

	requireScreen(t, asked.View().Content, "Switch to ~/src/web?")
}
