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
	cfg.Messaging.Channels = []string{teamChannel}
	world := newWorld()
	model := sized(t, tui.New(cfg, nil, world.deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act: open the preview, change the channel, and post
	preview := typing(t, model, "5", "p")

	// Assert: the preview offers the change and starts on the default channel
	requireScreen(t, preview.View().Content, "to  "+devChannel, "change channel")

	// Act: cycle to the other channel and post
	posted := typing(t, preview, "right", keyEnter)

	// Assert: the post went to the chosen channel, and the notice names it
	if got := world.channelPostedTo(); got != teamChannel {
		t.Errorf("posted to %q, want the chosen channel #team-b", got)
	}

	requireScreen(t, posted.View().Content, "announced to "+teamChannel)
}

func TestOneChannelOffersNoChange(t *testing.T) {
	t.Parallel()

	// Act
	preview := typing(t, newWorld().live(t, 120, 40), "5", "p", keyRight)

	// Assert
	requireScreen(t, preview.View().Content, "to  "+devChannel)
	refuseScreen(t, preview.View().Content, "change channel")
}

func TestTheChannelCyclesBackwardsToo(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Messaging.Channels = []string{teamChannel, "#team-c"}
	model := sized(t, tui.New(cfg, nil, newWorld().deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	preview := typing(t, model, "5", "p", "left")

	// Assert
	requireScreen(t, preview.View().Content, "to  #team-c")
}
