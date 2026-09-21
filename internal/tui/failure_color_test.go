// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// Errors the failure-color cases surface.
var (
	errForgeBoom  = errors.New("boom")
	errJiraReject = errors.New("jira rejected the request")
)

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestEveryFailureGlyphRendersRed(t *testing.T) {
	defer forceANSI(t)()

	open := redOpen()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
	}{
		"the spine's failed review stage": {
			prepare: func(w *world) { w.ci = []forge.CI{{State: forge.CIFailed}} },
		},
		"the review rail could not reach the forge": {
			prepare: func(w *world) { w.pullFound, w.pullErr = false, errForgeBoom },
			keys:    []string{"4"},
		},
		"a refused status change in the picker": {
			prepare: func(w *world) { w.moves, w.transitionErr = workflowMoves(), errJiraReject },
			keys:    []string{"t", keyEnter},
		},
		"a failed commit run": {
			prepare: func(w *world) { w.commitErr = errHookFailed },
			keys:    commitKeys("x"),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			faked := newWorld()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			total, red := strings.Count(view, failGlyph), strings.Count(view, open+failGlyph)
			if total != red {
				t.Errorf("%d of %d %q glyphs are not inside a red run:\n%s", total-red, total, failGlyph, view)
			}
		})
	}
}
