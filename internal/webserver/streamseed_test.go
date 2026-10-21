// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestAPullRequestOpenedAtANewHeadOutlastsAReadAnotherFrameWaitedOn(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// One frame reads the forge for the head before a commit, and an open
		// page's frame for that head waits on its read. The pull request is
		// opened at the head after the commit before the read lands, and the
		// forge's list lags behind its opens: the read does not replace the
		// open's answer, and the waiter takes what it found rather than read
		// the old head again, a read that would.
		slow, moving := newSlowForge(), &movingHead{head: webtest.FilledHead}
		deps := slow.wire(laggingForgeDeps())
		read := deps.Git.Branch
		deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) { return moving.branch(ctx, read) }
		handler := webtest.Serve(t, deps, webhookConfig())
		cancelHeld(t, handler)

		slow.slowDown()
		reading := frameInFlight(t, handler, slow.slowRead)
		waiting := webtest.OpenStream(t, handler, "/api/events")

		synctest.Wait()
		moving.commit()

		opened := webtest.Send(t, handler, http.MethodPost, "/api/pull-request", openRequestBody)
		if opened.Code != http.StatusOK {
			t.Fatalf("opening the pull request answered %d, want 200", opened.Code)
		}

		// Act
		close(slow.release)
		<-reading
		synctest.Wait()

		// Assert
		if frame := lastSnapshot(t, waiting.Close()); !frame.Review.Found || frame.Review.Pull.Number != openedPull {
			t.Errorf("the waiting page's last review = %+v, want #%d, opened at the new head",
				frame.Review, openedPull)
		}
	})
}

// seededForge is a forge whose list lags behind its opens, as
// laggingForgeDeps', counting its pull request reads, which the test may slow
// down, over a checkout the test moves and a clock that stands still, so no
// answer the forge cache holds ages. A pull request is opened on it at
// webtest.FilledHead, so the forge cache holds the open's answer.
type seededForge struct {
	handler http.Handler
	moving  *movingHead
	slow    *slowRead
	reads   atomic.Int32
}

// openedOnALaggingForge is a seededForge whose pull request is opened.
func openedOnALaggingForge(t *testing.T) *seededForge {
	t.Helper()

	seeded := &seededForge{moving: &movingHead{head: webtest.FilledHead}, slow: newSlowRead()}
	deps := laggingForgeDeps()
	deps.Clock = newSharedClock().read
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		seeded.reads.Add(1)
		seeded.slow.call()

		return forge.PullRequest{}, false, nil
	}
	read := deps.Git.Branch
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) { return seeded.moving.branch(ctx, read) }
	seeded.handler = webtest.Serve(t, deps, webhookConfig())
	cancelHeld(t, seeded.handler)

	opened := webtest.Send(t, seeded.handler, http.MethodPost, "/api/pull-request", openRequestBody)
	if opened.Code != http.StatusOK {
		t.Fatalf("opening the pull request answered %d, want 200", opened.Code)
	}

	return seeded
}

// readsOverFrames is how many times the forge is asked for its pull request
// over this many frames, one after the other.
func (f *seededForge) readsOverFrames(t *testing.T, frames int) int32 {
	t.Helper()

	before := f.reads.Load()

	for range frames {
		webtest.StreamOnce(t, f.handler, "/api/events")
	}

	return f.reads.Load() - before
}

func TestFramesAtANewHeadAfterAPullRequestOpenedHereAskTheForgeOnceAnInterval(t *testing.T) {
	t.Parallel()

	// Arrange
	seeded := openedOnALaggingForge(t)
	seeded.moving.commit()

	// Act
	reads := seeded.readsOverFrames(t, webtest.StreamedFrames)

	// Assert
	if reads != 1 {
		t.Errorf("%d frames of a new head while the open's answer is fresh asked the forge %d times, want once",
			webtest.StreamedFrames, reads)
	}
}

func TestPagesWaitingOnAReadAtANewHeadAfterAPullRequestOpenedHereShareIt(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// While the open's answer is fresh, one frame reads the forge at the
		// head after a commit and other pages' frames wait on its read: each
		// waiter takes what that read found rather than asking the forge.
		seeded := openedOnALaggingForge(t)
		seeded.moving.commit()
		before := seeded.reads.Load()

		seeded.slow.slowDown()
		reading := frameInFlight(t, seeded.handler, seeded.slow)

		for range webtest.StreamedFrames {
			webtest.OpenStream(t, seeded.handler, "/api/events")
		}

		synctest.Wait()

		// Act
		close(seeded.slow.release)
		<-reading
		synctest.Wait()

		// Assert
		if reads := seeded.reads.Load() - before; reads != 1 {
			t.Errorf("a frame and %d pages waiting on its read asked the forge %d times, want once",
				webtest.StreamedFrames, reads)
		}
	})
}

func TestAReadOfAnotherBranchReplacesAPullRequestOpenedHere(t *testing.T) {
	t.Parallel()

	// Arrange
	seeded := openedOnALaggingForge(t)
	seeded.moving.checkout("feat/PROJ-413", "fed321")

	// Act
	reads := seeded.readsOverFrames(t, 2)

	// Assert
	if reads != 1 {
		t.Errorf("two frames of another branch asked the forge %d times, want once: the first's answer serves the second",
			reads)
	}
}
