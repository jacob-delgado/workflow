// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// The tests here hold the context each kind of request hands the forge: a
// read's ends with its page, a read every stream shares runs on when the page
// whose frame began it leaves, and a frame waiting on such a read stops
// waiting when its own page leaves.

// heldFirstFind is a pull request find whose first call is held by held,
// under the context it is asked under, before it answers as answer does;
// every later call answers at once.
func heldFirstFind(
	held *heldCall, answer func(context.Context, string) (forge.PullRequest, bool, error),
) func(context.Context, string) (forge.PullRequest, bool, error) {
	var began atomic.Bool

	return func(ctx context.Context, branch string) (forge.PullRequest, bool, error) {
		if began.CompareAndSwap(false, true) {
			err := held.wait(ctx)
			if err != nil {
				return forge.PullRequest{}, false, err
			}
		}

		return answer(ctx, branch)
	}
}

func TestAReviewReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeldUntilEnded(), filledDeps()
	deps.Forge.FindPullRequest = func(ctx context.Context, _ string) (forge.PullRequest, bool, error) {
		return forge.PullRequest{}, false, held.wait(ctx)
	}
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, deps, config.Default()), http.MethodGet, "/api/review")

	<-held.entered

	// Act
	leave()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered
}

// Every stream shares the forge's read of the branch, and holds one that
// failed for the forge's interval: a read canceled because the page that
// began it left would be held as failed for every other page.
func TestASharedForgeReadGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeld(), filledDeps()
	deps.Forge.FindPullRequest = heldFirstFind(held, deps.Forge.FindPullRequest)
	handler := serve(t, deps, config.Default())
	leftMidRead(t, handler, held)

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !frame.Review.Found || frame.Problems != nil {
		t.Errorf("the next page's review = %+v, problems %+v; want the pull request found, though the page "+
			"that began the read left mid-read", frame.Review, frame.Problems)
	}
}

// Every stream shares the ask of who a post would come from, and holds one
// that failed for the forge's interval: an ask canceled because the page that
// began it left would leave every page without the author.
func TestASharedAuthorAskGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeld(), filledDeps()
	answer := deps.Forge.Author

	var began atomic.Bool

	deps.Forge.Author = func(ctx context.Context) (string, error) {
		if began.CompareAndSwap(false, true) {
			err := held.wait(ctx)
			if err != nil {
				return "", err
			}
		}

		return answer(ctx)
	}
	handler := serve(t, deps, config.Default())
	leftMidRead(t, handler, held)

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if frame.Messaging.Author != testAuthor {
		t.Errorf("the next page's author = %q, want %q, though the page that began the ask left mid-ask",
			frame.Messaging.Author, testAuthor)
	}
}

func TestAFrameWaitingOnAnotherPagesForgeReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeld(), filledDeps()
	deps.Forge.FindPullRequest = heldFirstFind(held, deps.Forge.FindPullRequest)
	handler := serve(t, deps, config.Default())
	reading := openStream(t, handler, "/api/events")

	<-held.entered

	defer func() {
		reading.cancel()
		close(held.release)
		<-reading.done
	}()

	waiting := openStream(t, handler, "/api/events")

	// Act
	waiting.cancel()

	// Assert
	select {
	case <-waiting.done:
	case <-time.After(endsWithin):
		t.Error("the frame that left kept waiting on the other page's forge read")
	}
}
