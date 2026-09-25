// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestThePushPreviewNamesTheRemoteThePushGoesTo(t *testing.T) {
	t.Parallel()

	// remote.pushDefault sends the push to fork, whatever the branch tracks.
	cases := map[string]struct{ upstream string }{
		"tracking origin": {upstream: gitrepo.DefaultRemote + "/" + featureName},
		"never pushed":    {upstream: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			forked := newWorld()
			forked.branch.Upstream = tt.upstream
			forked.branch.PushRemote = forkRemote

			// Act
			preview := typing(t, forked.live(t, 120, 40), "2", "P")

			// Assert
			requireScreen(t, preview.View().Content, "push "+featureName+" to "+forkRemote)
		})
	}
}
