// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// How the announcement held for CI steps, as the screen says it: its beat,
// a read or a key that comes after it ended, a post refused while another is
// being posted, why it was given up on, and what a quit leaves said.

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// namedFailure is a run in which the lint check failed and the build passed.
func namedFailure() forge.CI {
	return checkedCI().ci[0]
}

// waitingSince moves the world's clock on by wait.
func waitingSince(repo *world, wait time.Duration) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.elapsed += wait
}

func TestCtrlCWithAPostWaitingSaysItStoppedWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning)

	// Act
	quit, cmd := pressed(t, waiting, "ctrl+c")

	// Assert
	if got, want := messageType(cmd), messageType(tea.Quit); got != want {
		t.Errorf("ctrl+c returned %s, want %s", got, want)
	}

	line, stopped := quit.Destination().StoppedWaiting()
	if want := "Stopped waiting for CI on #42; nothing was announced."; !stopped || line != want {
		t.Errorf("StoppedWaiting = %q, %v; want %q", line, stopped, want)
	}
}

func TestAQuitWithNothingWaitingStopsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	idle := newWorld().live(t, 120, 40)

	// Act
	quit, _ := pressed(t, idle, "q")

	// Assert
	if line, stopped := quit.Destination().StoppedWaiting(); stopped {
		t.Errorf("StoppedWaiting = %q, want nothing stopped", line)
	}
}

func TestStopWaitingWithNothingWaitingSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	pane := typing(t, newWorld().live(t, 120, 40), "5")

	// Act
	view := typing(t, pane, "x").View().Content

	// Assert
	requireScreen(t, view, armed.ErrNothingWaiting.Error())
}

func TestWhenCIPassesWhileTheWaitingPostIsBeingPostedIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// w arms the post; a preview opens over it, announcing now instead; the
	// read w began finds CI passed, so the post is on its way, unanswered.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	posting, _ := finish(t, typing(t, armedPost, "p"), read)

	// Act
	view := typing(t, posting, "w").View().Content

	// Assert
	requireScreen(t, view, "┏━ Announce to Slack", armed.ErrBeingPosted.Error())
}

func TestAReadAnsweringAfterTheWaitStoppedChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// w arms the post and begins a read, which finds CI passed; x stops the
	// wait before the read answers.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	armedPost, read := pressed(t, typing(t, repo.live(t, 140, 40), "5", "p"), "w")
	stopped := typing(t, armedPost, "x")

	// Act
	answered := deliver(t, stopped, read).View().Content

	// Assert
	if posts := repo.asked("post "); len(posts) != 0 {
		t.Errorf("post calls = %q, want none once the wait was stopped", posts)
	}

	requireScreen(t, answered, "state  ○ nothing announced")
}

func TestAWaitingPostReadsOnOneBeat(t *testing.T) {
	t.Parallel()

	// Arrange
	// w schedules the next read; a read asked for meanwhile schedules its own,
	// so the first, once due, reads nothing.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}
	timer := &heldTimer{}
	waiting := typing(t, timed(t, repo, timer.after), "5", "p", "w")
	reread := typing(t, waiting, "r")
	before := len(repo.asked("find "))

	// Act
	timer.release(t, reread)

	// Assert
	if finds := len(repo.asked("find ")) - before; finds != 1 {
		t.Errorf("the reads due found the pull request %d times, want once: one beat", finds)
	}
}

func TestAStoppedPostReadsNoMore(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}
	timer := &heldTimer{}
	stopped := typing(t, timed(t, repo, timer.after), "5", "p", "w", "x")
	before := len(repo.asked("find "))

	// Act
	timer.release(t, stopped)

	// Assert
	if finds := len(repo.asked("find ")) - before; finds != 0 {
		t.Errorf("the read due found the pull request %d times once the wait was stopped, want none", finds)
	}
}

func TestAPostTheGateRefusesNamesTheChecksThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{namedFailure()}

	// Act
	view := typing(t, repo.live(t, 160, 40), "5", "p", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "a check failed on the pull request (lint)")
}

func TestAHeldPostTheGateRefusesSaysWhichChecksFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs as the post is set to wait, has passed on the read that lets it
	// go, and the lint check has failed by the gate's own read.
	repo := newWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}, namedFailure()}

	// Act
	view := typing(t, repo.live(t, 160, 40), "5", "p", "w").View().Content

	// Assert
	requireScreen(t, view, "a check failed on #42 (lint) as it was to be posted; it waits for a run that passes")
}

func TestAnAnnouncementTheForgeNeverListsIsGivenUpOn(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	opened := typing(t, repo.live(t, 160, 40), "4", "n", keyEnter, keyEnter, keyEsc, keyEsc, "5")
	waitingSince(repo, armed.UnheardPatience)

	// Act
	view := typing(t, opened, "r").View().Content

	// Assert
	requireScreen(t, view, "dropped the announcement to Slack waiting for #42: the forge did not list #42 within 10m")
}

func TestAWaitingPostWhoseCICannotBeReadIsGivenUpOn(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	repo.mu.Lock()
	repo.ciErr = errForgeDown
	repo.mu.Unlock()

	unheard := typing(t, waiting, "r")

	waitingSince(repo, armed.UnheardPatience)

	// Act
	view := typing(t, unheard, "r").View().Content

	// Assert
	requireScreen(t, view, "not announced: tried for 10m and failed: the forge is down")
}
