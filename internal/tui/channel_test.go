// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// rememberChannel records the channel a post was sent to.
func (w *world) rememberChannel(channel string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.postedChannel = channel
}

// channelPostedTo is the channel the last post named.
func (w *world) channelPostedTo() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.postedChannel
}

func TestTheChannelCanBeChangedBeforePosting(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Slack.Channels = []string{"#team-b"}
	world := newWorld()
	model := sized(t, tui.New(cfg, nil, world.deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act: open the preview, change the channel, and post
	preview := typing(t, model, "5", "p")

	// Assert: the preview offers the change and starts on the default channel
	requireScreen(t, preview.View().Content, "to  "+devChannel, "change channel")

	// Act: cycle to the other channel and post
	typing(t, preview, "right", keyEnter)

	// Assert: the post went to the chosen channel
	if got := world.channelPostedTo(); got != "#team-b" {
		t.Errorf("posted to %q, want the chosen channel #team-b", got)
	}
}

func TestOneChannelOffersNoChange(t *testing.T) {
	t.Parallel()

	// Act
	preview := typing(t, newWorld().live(t, 120, 40), "5", "p")

	// Assert
	requireScreen(t, preview.View().Content, "to  "+devChannel)
	refuseScreen(t, preview.View().Content, "change channel")
}
