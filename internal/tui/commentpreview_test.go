// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestCommentPreviewShowsTheConvertedMarkupWhenMarkdownIsOn(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.cfg.Jira.MarkdownComments = true
	world.edited = "See **the docs** at `run()`."

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
	world.edited = "See **the docs** at `run()`."

	// Act
	view := typing(t, world.live(t, 120, 40), "c").View().Content

	// Assert
	requireScreen(t, view, "See **the docs** at `run()`.")
}
