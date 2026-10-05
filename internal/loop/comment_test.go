// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
)

func TestACommentIsStoredInTheMarkupItsTrackerTakes(t *testing.T) {
	t.Parallel()

	const markdown = "See **the docs** at `run()`."

	cases := map[string]struct {
		markdownComments bool
		want             string
	}{
		"Jira with Markdown on reads wiki markup converted from it": {true, "See *the docs* at {{run()}}."},
		"Jira with Markdown off takes the text as typed":            {false, markdown},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			settings := config.Jira{MarkdownComments: testCase.markdownComments}

			// Act
			got := loop.CommentMarkupOf(settings).Stored(markdown)

			// Assert
			if got != testCase.want {
				t.Errorf("Stored = %q, want %q", got, testCase.want)
			}
		})
	}
}
