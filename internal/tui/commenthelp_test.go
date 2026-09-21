// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

func TestCommentEditorNamesTheMarkupTheInstanceUses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		markdown bool
		want     string
	}{
		"wiki markup by default":        {markdown: false, want: "Jira's own markup works here."},
		"Markdown note when configured": {markdown: true, want: "Markdown works here"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := newWorld()
			world.cfg.Jira.MarkdownComments = testCase.markdown

			// Act
			typing(t, world.live(t, 120, 40), "c")

			// Assert
			if !strings.Contains(world.editHelp, testCase.want) {
				t.Errorf("comment editor help = %q, want it to mention %q", world.editHelp, testCase.want)
			}
		})
	}
}
