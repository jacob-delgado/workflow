// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestAWaitingPostIsDroppedWhenItsPullRequestMerges(t *testing.T) {
	t.Parallel()

	// Arrange
	merging := newWorld()
	merging.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, merging.live(t, 120, 40), "5", "p", "w")
	merging.pull.State = forge.StateMerged

	// Act
	merged := typing(t, waiting, "2", "r", "5").View().Content

	// Assert
	if calls := merging.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request that merged before its CI passed", calls)
	}

	requireScreen(t, merged, "not announced: #42 merged before its CI passed")
}

func TestAWaitingPostIsDroppedWhenItsPullRequestCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	closing := newWorld()
	closing.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, closing.live(t, 120, 40), "5", "p", "w")
	closing.pull.State = forge.StateClosed

	// Act
	closed := typing(t, waiting, "2", "r").View().Content

	// Assert
	if calls := closing.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request closed without merging", calls)
	}

	requireScreen(t, closed, droppedPost)
}

// waitingOn is a world whose CI answers each of states in turn, the last
// repeated, read only when asked, since its interval is past the harness's
// horizon; and the interface with a post set, in its preview, to wait for CI
// to pass, on the messaging pane.
func waitingOn(t *testing.T, states ...forge.CIState) (*world, tui.Model) {
	t.Helper()

	repo := newWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = make([]forge.CI, 0, len(states))

	for _, state := range states {
		repo.ci = append(repo.ci, forge.CI{State: state, Total: 1})
	}

	return repo, typing(t, repo.live(t, 140, 40), "5", "p", "w")
}

// passesNow makes the world's CI pass from its next read on.
func passesNow(repo *world) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1}}
}

func TestAWaitingPostWaitsOnAFixAndGoesOnceARunPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, failed := waitingOn(t, forge.CIRunning, forge.CIFailed)

	// Act: look at the pane
	pane := failed.View().Content

	// Assert: it still waits, on a fix
	requireScreen(t, pane, "announces when CI passes · a check failed; waiting on a fix")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q while a check failed", calls)
	}

	// Act: a fix's run passes, and CI is read again
	passesNow(repo)
	typing(t, failed, "r")

	// Assert: it goes, once
	if calls := repo.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want one once a run passed", calls)
	}
}

func TestAWaitingPostSaysItWaitsForChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning, forge.CINone)

	// Act
	view := waiting.View().Content

	// Assert
	requireScreen(t, view, "announces when CI passes · no checks reported yet")
}

func TestAWaitingPostGivesUpWhenNoCheckIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CINone)
	repo.mu.Lock()
	repo.elapsed += loop.NoCIPatience
	repo.mu.Unlock()

	// Act
	gaveUp := typing(t, waiting, "r").View().Content

	// Assert
	requireScreen(t, gaveUp, "not announced: no checks reported on #42 within 10m; announce it by hand")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q with no check ever reported", calls)
	}
}

func TestWhenCIPassesIsOfferedWhileACheckFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	red := newWorld()
	red.ci = []forge.CI{failedCI()}

	// Act
	view := typing(t, red.live(t, 140, 40), "5", "p").View().Content

	// Assert
	requireScreen(t, footerLine(view), "w when CI passes")
}

func TestWhenCIPassesIsOfferedWithNoCheckReported(t *testing.T) {
	t.Parallel()

	// Arrange
	unchecked := newWorld()
	unchecked.ci = []forge.CI{{State: forge.CINone}}

	// Act
	view := typing(t, unchecked.live(t, 140, 40), "5", "p").View().Content

	// Assert
	requireScreen(t, footerLine(view), "w when CI passes")
}

func TestAnnounceNowIsNotOfferedWhileACheckFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	red := newWorld()
	red.ci = []forge.CI{failedCI()}

	// Act
	view := typing(t, red.live(t, 140, 40), "5", "p").View().Content

	// Assert
	refuseScreen(t, footerLine(view), "announce now")
	requireScreen(t, view, "A check failed, so it is not announced now; announcing once CI passes waits for a fix.")
}

func TestStopWaitingDropsTheWaitingPostUnposted(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)

	// Act
	stopped := typing(t, waiting, "x")

	// Assert
	requireScreen(t, stopped.View().Content, "stopped waiting; nothing was announced", "○ nothing announced")

	passesNow(repo)
	typing(t, stopped, "r")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q after waiting was stopped", calls)
	}
}

func TestStopWaitingIsOfferedOnlyWhileAPostWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	pane := typing(t, repo.live(t, 140, 40), "5")

	// Act: look at the pane with nothing waiting
	idle := footerLine(pane.View().Content)

	// Assert: there is nothing to stop
	refuseScreen(t, idle, "stop waiting")

	// Act: leave a post waiting
	waiting := footerLine(typing(t, pane, "p", "w").View().Content)

	// Assert: it can be stopped
	requireScreen(t, waiting, "x stop waiting")
}

func TestWhileAPostWaitsAnnouncingIsOfferedAsAnnounceNowInsteadAfterStopWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning)

	// Act
	footer := footerLine(waiting.View().Content)

	// Assert
	refuseScreen(t, footer, "announce to Slack")
	requireScreen(t, footer, "x stop waiting", "p announce now instead")

	if strings.Index(footer, "p announce now instead") < strings.Index(footer, "x stop waiting") {
		t.Errorf("announce now instead comes before stop waiting: %q", footer)
	}
}

func TestAnnounceNowInsteadOpensThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning)

	// Act
	preview := typing(t, waiting, "p").View().Content

	// Assert
	requireScreen(t, preview, "┏━ Announce to Slack", "enter announce now")
}

func TestAnnounceNowInsteadIsNotOfferedOnceAnotherSessionAnnouncedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	repo.mu.Lock()
	repo.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}
	repo.mu.Unlock()

	// Act
	reread := typing(t, waiting, "r")

	// Assert
	footer := footerLine(reread.View().Content)
	requireScreen(t, footer, "x stop waiting")
	refuseScreen(t, footer, "announce now instead", "announce to Slack")
}

func TestOnceAPostStopsWaitingAnnouncingIsOfferedAsAnnounceToTheServiceAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning)

	// Act
	stopped := typing(t, waiting, "x")

	// Assert
	footer := footerLine(stopped.View().Content)
	requireScreen(t, footer, "p announce to Slack")
	refuseScreen(t, footer, "announce now instead")
}

func TestAHeldPostTheGateRefusesKeepsWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs as the post is set to wait, has passed on the read that lets it
	// go, and a check has failed by the gate's own read.
	repo := newWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}, failedCI()}

	// Act
	refused := typing(t, repo.live(t, 140, 40), "5", "p", "w").View().Content

	// Assert
	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q past a gate that found a check failed", calls)
	}

	requireScreen(t, refused, "announces when CI passes · a check failed; waiting on a fix")
}

func TestAWaitingPostSeesAHeadMovedByAPush(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	timer := &heldTimer{}
	waiting := typing(t, timed(t, repo, timer.after), "5", "p", "w")
	repo.mu.Lock()
	repo.branch.Head = "def4567"
	repo.mu.Unlock()

	// Act
	timer.release(t, waiting)

	// Assert
	if asked := repo.asked("ci def4567"); len(asked) == 0 {
		t.Errorf("CI was read at %q, want the poll to read it at the head the push moved to", repo.asked("ci "))
	}
}

// failsNow makes a check fail on the world's CI from its next read on.
func failsNow(repo *world) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.ci = []forge.CI{failedCI()}
}

func TestARefusedPostNowKeepsTheWaitingPost(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits for CI; a check fails, unseen, and announcing now is asked.
	repo, waiting := waitingOn(t, forge.CIRunning)
	failsNow(repo)

	// Act
	refused := typing(t, waiting, "p", keyEnter, keyEsc).View().Content

	// Assert
	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q past a failed check", calls)
	}

	requireScreen(t, refused, "announces when CI passes")
}

func TestAPostWaitsForCIThePaneCouldNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge does not answer the pane's read of CI.
	unread := newWorld()
	unread.ciInterval = 2 * time.Second
	unread.ciErr = errForgeDown
	preview := typing(t, unread.live(t, 140, 40), "5", "p")

	// Act: look at the preview
	offered := footerLine(preview.View().Content)

	// Assert: both are offered, nothing having been seen to fail
	requireScreen(t, offered, "enter announce now", "w when CI passes")

	// Act: wait for CI to pass
	waiting := typing(t, preview, "w").View().Content

	// Assert: it waits, on CI as yet unread
	requireScreen(t, waiting, "◐ announces when CI passes")
	refuseScreen(t, waiting, "waiting on a fix", "no checks reported yet")
}

func TestAWaitingPostWaitsPastABranchItCouldNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits; the poll's read of the branch fails.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	timer := &heldTimer{}
	deps := repo.deps()
	deps.After = timer.after
	read, unreadable := deps.Git.Branch, atomic.Bool{}
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) {
		if unreadable.Load() {
			return gitrepo.Branch{}, errForgeDown
		}

		return read(ctx)
	}
	model := sized(t, tui.New(repo.cfg, nil, deps), 140, 40)
	waiting := typing(t, drain(t, model, model.Init()), "5", "p", "w")

	unreadable.Store(true)

	// Act
	timer.release(t, waiting)

	// Assert
	if polls := timer.waiting(); polls != 1 {
		t.Errorf("%d polls are scheduled after the branch could not be read, want the next one", polls)
	}
}
