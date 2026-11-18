// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The held announcement steps in rounds: an arm afresh begins one, and a hold
// asked for again in place, retold, keeps it. These tests hold what each ask
// reads, and when the server reads the hold again on its own.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// countingCIReads is deps with every read of CI counted in reads.
func countingCIReads(deps webserver.Deps, reads *atomic.Int32) webserver.Deps {
	checkCI := deps.Forge.CheckStatus
	deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		reads.Add(1)

		return checkCI(ctx, pull, head)
	}

	return deps
}

func TestHoldingTheWaitingAnnouncementAgainOnceCIPassedPostsIt(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits while CI runs; then CI passes.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)

		if first := announceWhenGreen(t, handler, nil); first.Code != http.StatusAccepted {
			t.Fatalf("first hold = %d (%s), want 202 while CI runs", first.Code, first.Body.String())
		}

		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		again := announceWhenGreen(t, handler, nil)

		// Assert
		if again.Code != http.StatusOK || len(world.posted()) != 1 {
			t.Errorf("POST %s = %d (%s), posted %q; want 200 posted: the hold asked again reads its CI at once",
				announcePath, again.Code, again.Body.String(), world.posted())
		}
	})
}

func TestARetoldHoldIsReadOncePerInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits while CI runs, and is asked for again,
		// CI running still, before its next read is due.
		var reads atomic.Int32

		world := newForgeWorld()
		handler := heldServer(t, countingCIReads(world.deps(), &reads), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		announceWhenGreen(t, handler, nil)

		before := reads.Load()

		// Act
		advance(ciTick)

		// Assert
		if read := reads.Load() - before; read != 1 {
			t.Errorf("CI read %d times in one interval, want once: the retold hold's reads replace the "+
				"ones set before it", read)
		}
	})
}

// heldArmRead is deps whose first read of CI once holding is set is held by
// held until it is let go.
func heldArmRead(deps webserver.Deps, holding *atomic.Bool, held *webtest.HeldCall) webserver.Deps {
	checkCI, wait := deps.Forge.CheckStatus, held.Once()
	deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		if holding.Load() {
			err := wait(ctx)
			if err != nil {
				return forge.CI{}, err
			}
		}

		return checkCI(ctx, pull, head)
	}

	return deps
}

func TestATimedReadOfAReplacedHoldReadsNothing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits while CI runs, its next read set; #43's
		// replaces it, and #43's own read is held across that read's time.
		var (
			holding atomic.Bool
			reads   atomic.Int32
		)

		held, world := webtest.NewHeld(), newForgeWorld()
		handler := heldServer(t, heldArmRead(countingCIReads(world.deps(), &reads), &holding, held), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.pull = 43 })

		fields := map[string]string{
			channelField: slackChannel, textField: previewAnnouncement(t, handler), whenField: whenCIPasses,
		}

		holding.Store(true)

		answered := make(chan *httptest.ResponseRecorder, 1)

		go func() { answered <- postAnnounce(t, handler, fields) }()

		t.Cleanup(func() {
			close(held.Release)
			<-answered
		})
		<-held.Entered

		before := reads.Load()

		// Act
		advance(ciTick)

		// Assert
		if read := reads.Load() - before; read != 0 {
			t.Errorf("CI read %d times as #42's read came due, want none: #42's hold was replaced", read)
		}
	})
}

func TestATimedReadOfAStoppedHoldReadsNothing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits while CI runs, its next read set, and is
		// stopped.
		var reads atomic.Int32

		world := newForgeWorld()
		handler := heldServer(t, countingCIReads(world.deps(), &reads), config.Default())
		announceWhenGreen(t, handler, nil)
		webtest.Send(t, handler, http.MethodDelete, queuedPath, "")

		before := reads.Load()

		// Act
		advance(ciTick)

		// Assert
		if read := reads.Load() - before; read != 0 {
			t.Errorf("CI read %d times as the stopped hold's read came due, want none", read)
		}
	})
}

func TestATimedReadOfAHoldThatWaitsReadsItsCI(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// #42's announcement waits while CI runs, its next read set, and
		// nothing is asked of it before that read comes due.
		var reads atomic.Int32

		world := newForgeWorld()
		handler := heldServer(t, countingCIReads(world.deps(), &reads), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)

		before := reads.Load()

		// Act
		advance(ciTick)

		// Assert
		if read := reads.Load() - before; read != 1 {
			t.Errorf("CI read %d times as the waiting hold's read came due, want once", read)
		}
	})
}

// heldForGreen asks handler to hold #42's previewed announcement for CI on a
// goroutine of its own, once reads of CI are held, and answers the channel
// its answer comes on, once held is let go.
func heldForGreen(
	t *testing.T, handler http.Handler, holding *atomic.Bool, held *webtest.HeldCall,
) <-chan *httptest.ResponseRecorder {
	t.Helper()

	fields := map[string]string{
		channelField: slackChannel, textField: previewAnnouncement(t, handler), whenField: whenCIPasses,
	}

	holding.Store(true)

	answered := make(chan *httptest.ResponseRecorder, 1)

	go func() { answered <- postAnnounce(t, handler, fields) }()

	<-held.Entered

	return answered
}

// letGo is the function that lets held go, once however often it is
// called, which t's cleanup calls too.
func letGo(t *testing.T, held *webtest.HeldCall) func() {
	t.Helper()

	release := sync.OnceFunc(func() { close(held.Release) })
	t.Cleanup(release)

	return release
}

// reasonOf is why held stands where it does, or "no reason".
func reasonOf(held *api.QueuedAnnouncement) string {
	if held == nil || held.Reason == nil {
		return "no reason"
	}

	return *held.Reason
}

// stoppedAt42 reports a hold of #42 stopped, saying why as reason does.
func stoppedAt42(held *api.QueuedAnnouncement, reason error) bool {
	return held != nil && held.Pull == 42 && held.State == api.QueuedAnnouncementStateStopped &&
		held.Reason != nil && *held.Reason == reason.Error()
}

func TestAnArmAnotherAskReplacedIsAnsweredStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's announcement is asked to wait for CI, its read held; meanwhile
	// #43's is asked to wait, replacing it.
	var holding atomic.Bool

	held, world := webtest.NewHeld(), newForgeWorld()
	handler := webtest.Serve(t, heldArmRead(world.deps(), &holding, held), config.Default())
	cancelHeld(t, handler)

	release := letGo(t, held)
	answered := heldForGreen(t, handler, &holding, held)

	world.turn(func(w *forgeWorld) { w.pull = 43 })

	if other := announceWhenGreen(t, handler, nil); other.Code != http.StatusAccepted {
		t.Fatalf("#43's hold = %d (%s), want 202", other.Code, other.Body.String())
	}

	// Act
	release()

	recorder := <-answered

	// Assert
	answer := webtest.Decode[api.QueuedAnnouncement](t, recorder)
	if recorder.Code != http.StatusAccepted || !stoppedAt42(&answer, armed.ErrReplaced) {
		t.Errorf("POST %s for #42 = %d %+v (%s), want 202 naming #42, stopped: another ask replaced it",
			announcePath, recorder.Code, answer, reasonOf(&answer))
	}
}

func TestAPullOpenedWhoseArmAnotherAskReplacedIsAnsweredStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42 opens, its announcement armed and its read held; meanwhile #43's
	// is asked to wait, replacing it.
	var holding atomic.Bool

	held, world := webtest.NewHeld(), newForgeWorld()
	handler := webtest.Serve(t, heldArmRead(openingDeps(world, false), &holding, held), webhookConfig())
	cancelHeld(t, handler)

	release := letGo(t, held)
	answered := make(chan api.OpenedPullRequest, 1)

	holding.Store(true)

	go func() { answered <- opened(t, handler) }()

	<-held.Entered
	world.turn(func(w *forgeWorld) { w.pull = 43 })

	if other := announceWhenGreen(t, handler, nil); other.Code != http.StatusAccepted {
		t.Fatalf("#43's hold = %d (%s), want 202", other.Code, other.Body.String())
	}

	// Act
	release()

	answer := <-answered

	// Assert
	if !stoppedAt42(answer.Announcement, armed.ErrReplaced) {
		t.Errorf("announcement = %+v (%s), want #42's, stopped: another ask replaced it",
			answer.Announcement, reasonOf(answer.Announcement))
	}
}

func TestAnArmStoppedMeanwhileIsAnsweredStoppedAsAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's announcement is asked to wait for CI, its read held; meanwhile
	// Stop waiting stops it.
	var holding atomic.Bool

	held, world := webtest.NewHeld(), newForgeWorld()
	handler := webtest.Serve(t, heldArmRead(world.deps(), &holding, held), config.Default())

	release := letGo(t, held)
	answered := heldForGreen(t, handler, &holding, held)

	if stopped := webtest.Send(t, handler, http.MethodDelete, queuedPath, ""); stopped.Code != http.StatusNoContent {
		t.Fatalf("Stop waiting = %d (%s), want 204", stopped.Code, stopped.Body.String())
	}

	// Act
	release()

	recorder := <-answered

	// Assert
	answer := webtest.Decode[api.QueuedAnnouncement](t, recorder)
	if recorder.Code != http.StatusAccepted || !stoppedAt42(&answer, armed.ErrStopped) {
		t.Errorf("POST %s for #42 = %d %+v (%s), want 202 naming #42, stopped as asked",
			announcePath, recorder.Code, answer, reasonOf(&answer))
	}
}
