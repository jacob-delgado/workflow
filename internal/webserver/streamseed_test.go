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
		// forge's list lags behind its opens: no read for the old head, the
		// waiter's own included, may replace the open's answer.
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
		moving.commit("def456")

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
// laggingForgeDeps', counting its pull request reads, over a checkout the
// test moves and a clock it moves by hand. A pull request is opened on it
// at webtest.FilledHead, so the forge cache holds the open's answer.
type seededForge struct {
	handler http.Handler
	moving  *movingHead
	clock   *sharedClock
	reads   atomic.Int32
}

// openedOnALaggingForge is a seededForge whose pull request is opened.
func openedOnALaggingForge(t *testing.T) *seededForge {
	t.Helper()

	seeded := &seededForge{moving: &movingHead{head: webtest.FilledHead}, clock: newSharedClock()}
	deps := laggingForgeDeps()
	deps.Clock = seeded.clock.read
	deps.Forge.FindPullRequest = func(context.Context, string) (forge.PullRequest, bool, error) {
		seeded.reads.Add(1)

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

// readsOverTwoFrames is how many times the forge is asked for its pull
// request over two frames, one after the other.
func (f *seededForge) readsOverTwoFrames(t *testing.T) int32 {
	t.Helper()

	before := f.reads.Load()

	webtest.StreamOnce(t, f.handler, "/api/events")
	webtest.StreamOnce(t, f.handler, "/api/events")

	return f.reads.Load() - before
}

func TestAReadOfAnotherBranchReplacesAPullRequestOpenedHere(t *testing.T) {
	t.Parallel()

	// Arrange
	seeded := openedOnALaggingForge(t)
	seeded.moving.checkout("feat/PROJ-413", "fed321")

	// Act
	reads := seeded.readsOverTwoFrames(t)

	// Assert
	if reads != 1 {
		t.Errorf("two frames of another branch asked the forge %d times, want once: the first's answer serves the second",
			reads)
	}
}

func TestAReadOfANewHeadReplacesAPullRequestOpenedHereOnceItIsNoLongerFresh(t *testing.T) {
	t.Parallel()

	// Arrange
	seeded := openedOnALaggingForge(t)
	seeded.moving.commit("def456")
	seeded.clock.pastTheForgeInterval()

	// Act
	reads := seeded.readsOverTwoFrames(t)

	// Assert
	if reads != 1 {
		t.Errorf("two frames of a new head after the open's answer aged asked the forge %d times, want once", reads)
	}
}
