// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The announcement's preview saves one owner link at a time: while one is
// being saved, the tag section says so, and a link key, or the owner
// picker's, is refused in plain words in the footer, lest the store keep one
// answer while the screen shows the other.

import (
	"strings"
	"testing"
)

// linkBeingSaved is the footer's guidance for a link key pressed while
// another link is being saved.
const linkBeingSaved = "a link is being saved; try again once it is"

// requireGuidance fails the test unless the notice row, the one above the
// footer, says text plainly, with no failure mark, and nothing else on the
// screen repeats it.
func requireGuidance(t *testing.T, view, text string) {
	t.Helper()

	requireNotice(t, view, text)

	if lines := strings.Split(plain(view), "\n"); strings.Contains(lines[len(lines)-2], failGlyph) {
		t.Errorf("the notice is %q; want it plain, with no failure mark", lines[len(lines)-2])
	}

	if shown := strings.Count(plain(view), text); shown != 1 {
		t.Errorf("the screen shows %q %d times; want it on the notice row alone:\n%s", text, shown, plain(view))
	}
}

func TestALinkKeyWaitsForTheLinkBeingSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	// ben is marked not on Slack, and while that is saved, carla too.
	marked, _ := withheldKeys(t, preview, tagging.counting("link-owner "), "x", "down", "x")

	// Assert
	if links := tagging.asked("link-owner "); len(links) != 1 {
		t.Errorf("link calls = %q while one was out; want 1", links)
	}

	requireGuidance(t, marked.View().Content, linkBeingSaved)
}

func TestTheSectionSaysALinkIsBeingSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	marking, _ := withheldKeys(t, preview, tagging.counting("link-owner "), "x")

	// Assert
	requireScreen(t, marking.View().Content, "Code owners", "saving the link…")
}

func TestThePickerWaitsForTheLinkBeingSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	// ben is marked not on Slack, and while that is saved, carla's picker is
	// asked for.
	picking, _ := withheldKeys(t, preview, tagging.counting("link-owner "), "x", "down", "a")

	// Assert
	view := picking.View().Content
	requireScreen(t, view, "Announce to Slack")
	refuseScreen(t, view, "Link carla to Slack")
	requireGuidance(t, view, linkBeingSaved)
}

func TestALinkKeyIsTakenAgainOnceTheLinkIsSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")
	refused, kept := withheldKeys(t, preview, tagging.counting("link-owner "), "x", "down", "x")
	saved := handedOver(t, refused, kept)

	// Act
	// once ben's link is saved, carla is marked again.
	view := typing(t, saved, "x").View().Content

	// Assert
	refuseScreen(t, view, "a link is being saved")

	if links := tagging.asked("link-owner "); len(links) != 2 || links[1] != "link-owner carla nobody" {
		t.Errorf("link calls = %q, want ben's, then carla's once ben's was saved", links)
	}
}

// linkHint and notOnSlackHint are how the preview's footer offers the link
// keys on an owner.
const (
	linkHint       = "a link to Slack"
	notOnSlackHint = "x not on Slack"
)

func TestTheLinkHintsHideWhileALinkIsBeingSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")
	requireScreen(t, footerLine(preview.View().Content), linkHint, notOnSlackHint)

	// Act
	marking, _ := withheldKeys(t, preview, tagging.counting("link-owner "), "x")

	// Assert
	refuseScreen(t, footerLine(marking.View().Content), linkHint, notOnSlackHint)
}

func TestTheLinkHintsComeBackOnceTheLinkIsAnswered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		linkErr error
	}{
		{name: "link saved", linkErr: nil},
		{name: "link refused", linkErr: errDirectoryDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tagging := taggingWorld()
			tagging.slack.linkErr = tt.linkErr
			preview := typing(t, tagging.live(t, 140, 40), "5", "p")
			marking, kept := withheldKeys(t, preview, tagging.counting("link-owner "), "x")

			// Act
			answered := handedOver(t, marking, kept)

			// Assert
			requireScreen(t, footerLine(answered.View().Content), linkHint, notOnSlackHint)
		})
	}
}
