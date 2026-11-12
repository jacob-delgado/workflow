// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// waitingOn is the held announcement of #42 to #dev, waiting, its CI last
// read as ci.
func waitingOn(ci api.CIState) api.QueuedAnnouncement {
	return api.QueuedAnnouncement{State: api.QueuedAnnouncementStateWaiting, Channel: slackChannel, Pull: 42, Ci: &ci}
}

// sameHold reports whether held stands as want does: its state, channel,
// pull request, reason and CI.
func sameHold(held *api.QueuedAnnouncement, want api.QueuedAnnouncement) bool {
	return held != nil && held.State == want.State && held.Channel == want.Channel && held.Pull == want.Pull &&
		orEmpty(held.Reason) == orEmpty(want.Reason) && ciOf(held.Ci) == ciOf(want.Ci)
}

// ciOf is a CI state that may be left out, as text.
func ciOf(state *api.CIState) string {
	if state == nil {
		return ""
	}

	return string(*state)
}

func TestAHeldAnnouncementWaitsOnAFix(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIFailed })

		// Act: a read after a check failed
		advance(ciTick)

		// Assert: it waits, on a fix
		if failed := heldAnnouncement(t, handler); !sameHold(failed, waitingOn(api.CIStateFailed)) ||
			failed.WaitingOn == nil || *failed.WaitingOn != api.QueuedAnnouncementWaitingOnFix {
			t.Errorf("frame's held announcement = %+v, want it waiting on a fix", failed)
		}

		// Act: a read once a run passes
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
		advance(ciTick)

		// Assert: it went
		passed := heldAnnouncement(t, handler)
		if passed == nil || passed.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
			t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", passed, world.posted())
		}
	})
}

func TestAHeldAnnouncementGivesUpWhenNoCheckIsReported(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// No check is reported, and an hour passes, past the ten minutes'
		// patience.
		world := newForgeWorld()
		world.ci = forge.CINone
		handler := heldServer(t, world.deps(), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(*forgeWorld) {})

		// Act
		advance(ciTick)

		// Assert
		held := heldAnnouncement(t, handler)
		reason := "no checks reported on #42 within 10m; announce it by hand"

		if held == nil || held.State != api.QueuedAnnouncementStateDropped || orEmpty(held.Reason) != reason {
			t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, reason)
		}

		if posts := world.posted(); len(posts) != 0 {
			t.Errorf("posted %q, want nothing", posts)
		}
	})
}

func TestAHeldAnnouncementGivesUpOnCIThatRunsUnchangedForTwoHours(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// CI runs, and reads the same for two hours (P033).
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(*forgeWorld) {})
		world.turn(func(*forgeWorld) {})

		// Act
		advance(ciTick)

		// Assert
		held := heldAnnouncement(t, handler)
		reason := "CI on #42 has been running for 2h"

		if held == nil || held.State != api.QueuedAnnouncementStateDropped || orEmpty(held.Reason) != reason {
			t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, reason)
		}

		if posts := world.posted(); len(posts) != 0 {
			t.Errorf("posted %q, want nothing", posts)
		}
	})
}

func TestAHeldAnnouncementCountsNoQuietTimeAcrossAnOutage(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// No check is reported; then CI cannot be read for an hour, past the
		// ten minutes' patience, and a read finds it so; then an hour on, a
		// read is heard, still with no check.
		world := newForgeWorld()
		world.ci = forge.CINone
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ciErr = errForgeDown })
		advance(ciTick)
		world.turn(func(w *forgeWorld) { w.ciErr = nil })

		// Act
		advance(ciTick)

		// Assert
		if held := heldAnnouncement(t, handler); !sameHold(held, waitingOn(api.CIStateNone)) {
			t.Errorf("frame's held announcement = %+v, want it still waiting, an outage not counted as quiet", held)
		}
	})
}

// step changes w, then moves its clock on by elapsed.
func (w *forgeWorld) step(elapsed time.Duration, change func(*forgeWorld)) {
	w.mu.Lock()
	defer w.mu.Unlock()

	change(w)
	w.now = w.now.Add(elapsed)
}

func TestAHeldAnnouncementCountsNoQuietTimeAcrossAFrameShowingCIHeldFromBefore(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// A read hears no check; the next one's read of CI fails, while a
		// frame shows the CI read before in its place; half an hour on, a
		// read is heard, still with no check.
		world := newForgeWorld()
		world.ci = forge.CINone
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.step(time.Minute, func(*forgeWorld) {})
		advance(ciTick)
		heldAnnouncement(t, handler)
		world.step(time.Minute, func(w *forgeWorld) { w.ciErr = errForgeDown })
		heldAnnouncement(t, handler)
		advance(ciTick)
		world.step(28*time.Minute, func(w *forgeWorld) { w.ciErr = nil })

		// Act
		advance(ciTick)

		// Assert
		if held := heldAnnouncement(t, handler); !sameHold(held, waitingOn(api.CIStateNone)) {
			t.Errorf("frame's held announcement = %+v, want it still waiting: a read that failed is not quiet", held)
		}
	})
}

// failingAtTheGate is a CI read that answers running until it is armed, then
// passed for the read that lets a held announcement go, and failed for every
// read after it: a check that failed between the frame and the gate's own
// read.
type failingAtTheGate struct {
	mu    sync.Mutex
	armed bool
	reads int
}

// check answers the CI read.
func (g *failingAtTheGate) check(context.Context, forge.PullRequest, string) (forge.CI, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.armed {
		return forge.CI{State: forge.CIRunning, Total: 1}, nil
	}

	g.reads++
	if g.reads == 1 {
		return forge.CI{State: forge.CIPassed, Total: 1}, nil
	}

	return forge.CI{State: forge.CIFailed, Total: 1}, nil
}

// arm makes the next read pass, and every one after it fail.
func (g *failingAtTheGate) arm() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.armed = true
}

func TestAHeldAnnouncementTheGateRefusesKeepsWaiting(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		world, gate := newForgeWorld(), &failingAtTheGate{}
		deps := world.deps()
		deps.Forge.CheckStatus = gate.check
		handler := heldServer(t, deps, config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		gate.arm()
		world.turn(func(*forgeWorld) {})

		// Act
		advance(ciTick)

		// Assert
		if held := heldAnnouncement(t, handler); !sameHold(held, waitingOn(api.CIStateFailed)) ||
			len(world.posted()) != 0 {
			t.Errorf("frame's held announcement = %+v, posted %q; want it waiting again, nothing posted",
				held, world.posted())
		}
	})
}

func TestAnnouncingWhenCIPassesHoldsAnythingShortOfPassing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*forgeWorld){
		"no check reported": func(w *forgeWorld) { w.ci = forge.CINone },
		"a run that failed": func(w *forgeWorld) { w.ci = forge.CIFailed },
		"CI running":        func(w *forgeWorld) { w.ci = forge.CIRunning },
		"CI unread":         func(w *forgeWorld) { w.ciErr = forge.ErrUnreachable },
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			change(world)
			handler := webtest.Serve(t, world.deps(), config.Default())
			cancelHeld(t, handler)

			// Act
			recorder := announceWhenGreen(t, handler, nil)

			// Assert
			if recorder.Code != http.StatusAccepted || len(world.posted()) != 0 {
				t.Errorf("status = %d (%s), posted %q; want 202 and nothing posted",
					recorder.Code, recorder.Body.String(), world.posted())
			}
		})
	}
}

func TestOnlyAReadyAnnouncementWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	world.state = forge.StateMerged
	handler := webtest.Serve(t, world.deps(), config.Default())

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	if failure := webtest.Decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict ||
		failure.Code != api.ProblemCodeConflict || len(world.posted()) != 0 {
		t.Errorf("status/code = %d/%s, posted %q; want 409 and nothing posted",
			recorder.Code, failure.Code, world.posted())
	}
}

func TestAPreviewSaysHowCIStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change  func(*forgeWorld)
		ci      api.CIState
		canWait bool
	}{
		"CI running":   {change: func(w *forgeWorld) { w.ci = forge.CIRunning }, ci: api.CIStateRunning, canWait: true},
		"failed":       {change: func(w *forgeWorld) { w.ci = forge.CIFailed }, ci: api.CIStateFailed, canWait: true},
		"no check yet": {change: func(w *forgeWorld) { w.ci = forge.CINone }, ci: api.CIStateNone, canWait: true},
		"CI passed":    {change: func(w *forgeWorld) { w.ci = forge.CIPassed }, ci: api.CIStatePassed},
		"CI unread":    {change: func(w *forgeWorld) { w.ciErr = forge.ErrUnreachable }, ci: "", canWait: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newForgeWorld()
			tt.change(world)

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, world.deps(), config.Default()), "/api/announcement")

			// Assert
			preview := webtest.Decode[api.Announcement](t, recorder)
			canWait := preview.CanWaitForCi != nil && *preview.CanWaitForCi

			if ciOf(preview.Ci) != string(tt.ci) || canWait != tt.canWait {
				t.Errorf("ci = %q, can_wait_for_ci = %v; want %q, %v", ciOf(preview.Ci), canWait, tt.ci, tt.canWait)
			}
		})
	}
}

func TestARefusedPostNowKeepsTheHeldAnnouncement(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// An announcement is held for CI; a check fails, and Announce now is
		// asked.
		world := newForgeWorld()
		handler := heldServer(t, world.deps(), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIFailed })
		advance(ciTick)

		// Act: announce now
		refused := postAnnounce(t, handler, map[string]string{channelField: slackChannel})

		// Assert: refused, and the held one still waits
		if refused.Code != http.StatusConflict {
			t.Fatalf("status = %d (%s), want 409 for a failed check", refused.Code, refused.Body.String())
		}

		if held := heldAnnouncement(t, handler); !sameHold(held, waitingOn(api.CIStateFailed)) {
			t.Fatalf("frame's held announcement = %+v, want the held one still waiting on a fix", held)
		}

		// Act: a run passes
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })
		advance(ciTick)

		// Assert: it goes, once
		held := heldAnnouncement(t, handler)
		if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
			t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
		}
	})
}

// fixedHead is the commit a fix for a check failed at webtest.FilledHead is
// pushed as.
const fixedHead = "def5678"

// fixedOnANewHead is deps reading a check failed at webtest.FilledHead and
// every check passed at any other head: a fix pushed as a new commit.
func fixedOnANewHead(deps webserver.Deps) webserver.Deps {
	deps.Forge.CheckStatus = func(_ context.Context, _ forge.PullRequest, head string) (forge.CI, error) {
		if head == webtest.FilledHead {
			return forge.CI{State: forge.CIFailed, Total: 1}, nil
		}

		return forge.CI{State: forge.CIPassed, Total: 1}, nil
	}

	return deps
}

func TestAHeldAnnouncementGoesOnceAFixLandsOnANewHead(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// A check failed where it was held, and the fix was pushed as a new
		// commit whose run passes.
		world := newForgeWorld()
		handler := heldServer(t, fixedOnANewHead(world.deps()), config.Default())
		cancelHeld(t, handler)
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.head = fixedHead })

		// Act
		advance(ciTick)

		// Assert
		held := heldAnnouncement(t, handler)
		if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
			t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
		}
	})
}
