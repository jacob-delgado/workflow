// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
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

	// Arrange
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIFailed })

	// Act: a frame after a check failed
	failed := heldAnnouncement(t, handler)

	// Assert: it waits, on a fix
	if !sameHold(failed, waitingOn(api.CIStateFailed)) {
		t.Errorf("frame's held announcement = %+v, want it waiting with CI failed", failed)
	}

	// Act: a frame once a run passes
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	passed := heldAnnouncement(t, handler)

	// Assert: it went
	if passed == nil || passed.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", passed, world.posted())
	}
}

func TestAHeldAnnouncementGivesUpWhenNoCheckIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	// No check is reported, and an hour passes, past the ten minutes'
	// patience.
	world := newForgeWorld()
	world.ci = forge.CINone
	handler := serve(t, world.deps(), config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(*forgeWorld) {})

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	reason := "no checks reported on #42 within 10m; announce it by hand"
	if held == nil || held.State != api.QueuedAnnouncementStateDropped || orEmpty(held.Reason) != reason {
		t.Errorf("frame's held announcement = %+v, want it dropped because %q", held, reason)
	}

	if posts := world.posted(); len(posts) != 0 {
		t.Errorf("posted %q, want nothing", posts)
	}
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

	// Arrange
	world, gate := newForgeWorld(), &failingAtTheGate{}
	deps := world.deps()
	deps.Forge.CheckStatus = gate.check
	handler := serve(t, deps, config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	gate.arm()
	world.turn(func(*forgeWorld) {})

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if !sameHold(held, waitingOn(api.CIStateFailed)) || len(world.posted()) != 0 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it waiting again, nothing posted",
			held, world.posted())
	}
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
			handler := serve(t, world.deps(), config.Default())
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
	handler := serve(t, world.deps(), config.Default())

	// Act
	recorder := announceWhenGreen(t, handler, nil)

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusConflict ||
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
			recorder := get(t, serve(t, world.deps(), config.Default()), "/api/announcement")

			// Assert
			preview := decode[api.Announcement](t, recorder)
			canWait := preview.CanWaitForCi != nil && *preview.CanWaitForCi

			if ciOf(preview.Ci) != string(tt.ci) || canWait != tt.canWait {
				t.Errorf("ci = %q, can_wait_for_ci = %v; want %q, %v", ciOf(preview.Ci), canWait, tt.ci, tt.canWait)
			}
		})
	}
}

func TestARefusedPostNowKeepsTheHeldAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// An announcement is held for CI; a check fails, and Announce now is asked.
	world := newForgeWorld()
	handler := serve(t, world.deps(), config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIFailed })

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

	held := heldAnnouncement(t, handler)

	// Assert: it goes, once
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
	}
}

// fixedHead is the commit a fix for a check failed at filledHead is
// pushed as.
const fixedHead = "def5678"

// fixedOnANewHead is deps reading a check failed at filledHead and every
// check passed at any other head: a fix pushed as a new commit.
func fixedOnANewHead(deps webserver.Deps) webserver.Deps {
	deps.Forge.CheckStatus = func(_ context.Context, _ forge.PullRequest, head string) (forge.CI, error) {
		if head == filledHead {
			return forge.CI{State: forge.CIFailed, Total: 1}, nil
		}

		return forge.CI{State: forge.CIPassed, Total: 1}, nil
	}

	return deps
}

func TestAHeldAnnouncementGoesOnceAFixLandsOnANewHead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A check failed where it was held, and the fix was pushed as a new
	// commit whose run passes.
	world := newForgeWorld()
	handler := serve(t, fixedOnANewHead(world.deps()), config.Default())
	cancelHeld(t, handler)
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.head = fixedHead })

	// Act
	held := heldAnnouncement(t, handler)

	// Assert
	if held == nil || held.State != api.QueuedAnnouncementStateAnnounced || len(world.posted()) != 1 {
		t.Errorf("frame's held announcement = %+v, posted %q; want it announced once", held, world.posted())
	}
}
