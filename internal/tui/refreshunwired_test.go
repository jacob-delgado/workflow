// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A pane whose read has no seam to ask begins no read: a refresh asks
// nothing, and marks nothing in flight, since no answer would end the mark.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestARefreshWithNoSeamToReadAsksNothingAndMarksNothing(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		world  func() *world
		unwire func(deps *tui.Deps)
		// pane is the key to the pane refreshed, title the pane's name,
		// and asked the prefix of the read a refresh would ask.
		pane, title, asked string
	}{
		"Reviews, with no forge to list them": {
			world: withReviews, unwire: func(deps *tui.Deps) { deps.Forge.ReviewRequests = nil },
			pane: "6", title: reviewsGroup, asked: "reviews",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := tt.world()
			deps := repo.deps()
			tt.unwire(&deps)
			model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)
			pane := typing(t, drain(t, model, model.Init()), tt.pane)
			before := len(repo.asked(tt.asked))

			// Act
			view := typing(t, pane, "r").View().Content

			// Assert
			if after := len(repo.asked(tt.asked)); after != before {
				t.Errorf("the refresh asked %q %d time(s), want none", tt.asked, after-before)
			}

			refuseScreen(t, view, tt.title+" ◐")
		})
	}
}
