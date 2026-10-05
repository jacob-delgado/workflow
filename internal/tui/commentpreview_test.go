// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

// markdownComment is a comment written in Markdown, which Jira stores as
// "See *the docs* at {{run()}}.".
const markdownComment = "See **the docs** at `run()`."

func TestCommentPreviewShowsTheConvertedMarkupWhenMarkdownIsOn(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.cfg.Jira.MarkdownComments = true
	world.edited = markdownComment

	// Act
	view := typing(t, world.live(t, 120, 40), "c").View().Content

	// Assert
	// The preview is the one gate before a comment is posted, so with conversion
	// on it must show the wiki markup Jira will store, not the Markdown source.
	requireScreen(t, view, "See *the docs* at {{run()}}.")
}

func TestCommentPreviewShowsTheTextVerbatimByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.edited = markdownComment

	// Act
	view := typing(t, world.live(t, 120, 40), "c").View().Content

	// Assert
	requireScreen(t, view, "See **the docs** at `run()`.")
}

func TestPostingSendsTheMarkupThePreviewShowed(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.cfg.Jira.MarkdownComments = true
	world.edited = markdownComment
	previewed := typing(t, world.live(t, 120, 40), "c")

	// Act
	typing(t, previewed, "enter")

	// Assert
	// What the preview showed is what is sent: the conversion happens before the
	// last look, never after it, in a client that read the setting once.
	if got := world.asked("comment "); len(got) != 1 || got[0] != "comment "+issueKey+" See *the docs* at {{run()}}." {
		t.Errorf("posted %q, want the wiki markup the preview showed", got)
	}
}
