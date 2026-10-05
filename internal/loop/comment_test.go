// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

func TestACommentIsStoredInTheMarkupItsTrackerTakes(t *testing.T) {
	t.Parallel()

	const markdown = "See **the docs** at `run()`."

	cases := map[string]struct {
		issueKey         jira.Key
		markdownComments bool
		want             string
	}{
		"Jira with Markdown on reads wiki markup converted from it": {jiraKey, true, "See *the docs* at {{run()}}."},
		"Jira with Markdown off takes the text as typed":            {jiraKey, false, markdown},
		"a forge renders Markdown itself, whatever Jira's setting":  {"42", true, markdown},
		"a forge takes Markdown with Jira's setting off":            {"42", false, markdown},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			settings := config.Jira{MarkdownComments: testCase.markdownComments}

			// Act
			got := loop.CommentMarkupOf(settings, testCase.issueKey).Stored(markdown)

			// Assert
			if got != testCase.want {
				t.Errorf("Stored = %q, want %q", got, testCase.want)
			}
		})
	}
}
