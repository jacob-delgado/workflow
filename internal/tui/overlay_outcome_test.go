// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// outcomeTail ends the long failure reason, far enough in that it is only seen
// if the reason is wrapped rather than clipped.
const outcomeTail = "SEEMEATTHEEND"

// errLongReason is a refusal long enough to be clipped or pushed below the fold
// when drawn on one line at the bottom of an overlay.
var errLongReason = errors.New(
	"git: exit status 128: fatal: not a git repository (or any of the parent directories) " + outcomeTail)

// twoHooks is a repository with two unmanaged hooks, enough for the offer to
// have a body that could push a failure below the fold.
func twoHooks() []hooks.GitHook {
	return []hooks.GitHook{
		{Name: "commit-msg", Script: "#!/bin/sh\nmake test\n"},
		{Name: "pre-push", Script: "#!/bin/sh\nmake lint\n"},
	}
}

func TestAnOverlayShowsAFailureFully(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prepare func(*world)
		keys    []string
	}{
		"comment preview": {
			prepare: func(w *world) { w.edited, w.commentErr = "a short comment", errLongReason },
			keys:    []string{"c", keyEnter},
		},
		"branch creator": {
			prepare: func(w *world) { w.createErr = errLongReason },
			keys:    []string{"2", "b", keyEnter},
		},
		"pull request composer": {
			prepare: func(w *world) { w.pullFound, w.openErr = false, errLongReason },
			keys:    []string{"4", "n", keyEnter},
		},
		"slack preview": {
			prepare: func(w *world) { w.postErr = errLongReason },
			keys:    []string{"5", "p", keyEnter},
		},
		"hookgen offer": {
			prepare: func(w *world) { w.gitHooks, w.writeErr = twoHooks(), errLongReason },
			keys:    []string{"3", "g", keyEnter},
		},
		"commit composer": {
			prepare: func(w *world) { w.editErr = errLongReason },
			keys:    []string{"3", "c", keyCtrlO},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			tt.prepare(faked)

			// Act
			view := typing(t, faked.live(t, 80, 24), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, outcomeTail)
		})
	}
}
