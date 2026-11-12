// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The tests here hold the context an announcement is posted and remembered
// under: a held one goes under the server's lifetime, whatever page asked.

func TestAHeldAnnouncementPostedOnItsOwnReadGoesUnderTheServersLifetime(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		world := newForgeWorld()
		deps := world.deps()
		posted := deps.Messaging.Post
		ended := make(chan error, 1)
		deps.Messaging.Post = func(ctx context.Context, channel, text string) error {
			ended <- ctx.Err()

			return posted(ctx, channel, text)
		}
		handler := heldServer(t, deps, config.Default())
		announceWhenGreen(t, handler, nil)
		world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

		// Act
		advance(ciTick)

		// Assert
		select {
		case err := <-ended:
			if err != nil {
				t.Errorf("the held announcement was posted under a context that ended with %v, want the server's", err)
			}
		default:
			t.Fatal("the read with green CI did not post the held announcement")
		}
	})
}

// The store's read of what was announced swallows its error, as the wiring's
// does, so a read a frame's page left mid-way reads as nothing announced. Held
// for every stream, it would let the next post repeat an announcement made.
func TestAFrameWhosePageLeftDoesNotLetAnAnnouncementPostTwice(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	deps := world.deps()
	deps.Store.Announced = func(ctx context.Context) ([]loop.AnnounceRecord, error) {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("reading what was announced: %w", err)
		}

		ready := loop.Announced{Pull: 42, Moment: messaging.MomentReady}

		return []loop.AnnounceRecord{{Made: ready, Generation: 1, Posted: true}}, nil
	}
	handler := webtest.Serve(t, deps, config.Default())
	webtest.StreamOnce(t, handler, "/api/events")
	previewed := previewAnnouncement(t, handler)

	// Act
	recorder := postAnnounce(t, handler, map[string]string{channelField: slackChannel, textField: previewed})

	// Assert
	if recorder.Code != http.StatusConflict || len(world.posted()) != 0 {
		t.Errorf("status = %d, posted %q; want 409 and nothing posted again", recorder.Code, world.posted())
	}
}

// The scope a commit opens on is read from the store once and held for every
// stream; the store's read takes a failure for none, so a read a frame's page
// left mid-way would be held as no scope learned.
func TestAFrameWhosePageLeftStillLearnsTheScopeForEveryStream(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Store.LastScope = func(ctx context.Context) (string, bool) {
		if ctx.Err() != nil {
			return "", false
		}

		return learnedScope, true
	}
	handler := webtest.Serve(t, deps, config.Default())

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if frame.SuggestedScope != learnedScope {
		t.Errorf("suggested scope = %q, want the learned api though the page left", frame.SuggestedScope)
	}
}
