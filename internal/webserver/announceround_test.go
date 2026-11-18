// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// The held announcement steps in rounds: an arm afresh begins one, and a hold
// asked for again in place, retold, keeps it. These tests hold what each ask
// reads, and when the server reads the hold again on its own.

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
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
