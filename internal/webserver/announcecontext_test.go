// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// The tests here hold the context an announcement is posted and remembered
// under: a held one goes once whatever page's frame takes it to post.

func TestAHeldAnnouncementAFrameTakesToPostGoesOnThoughThatPageLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newForgeWorld()
	deps := world.deps()
	posted := deps.Messaging.Post
	ended := make(chan error, 1)
	deps.Messaging.Post = func(ctx context.Context, channel, text string) error {
		ended <- ctx.Err()

		return posted(ctx, channel, text)
	}
	handler := serve(t, deps, config.Default())
	announceWhenGreen(t, handler, nil)
	world.turn(func(w *forgeWorld) { w.ci = forge.CIPassed })

	// Act
	// A frame whose page has already left.
	streamOnce(t, handler, "/api/events")

	// Assert
	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("the held announcement was posted under a context that ended with %v, want the server's", err)
		}
	default:
		t.Fatal("the frame with green CI did not post the held announcement")
	}
}
