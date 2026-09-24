// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
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
			prepare: func(w *world) { w.edited, w.commentErr = shortComment, errLongReason },
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
		"messaging preview": {
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
		"worktree creator": {
			prepare: func(w *world) { w.worktreeErr = errLongReason },
			keys:    []string{"2", "b", keyCtrlW, keyEnter},
		},
		"issue linker": {
			prepare: func(w *world) { w.pullFound, w.linkErr = false, errLongReason },
			keys:    []string{"4", "n", keyEnter, keyEnter},
		},
		"pull request editor": {
			prepare: func(w *world) { w.editPullErr = errLongReason },
			keys:    []string{"4", "e", keyEnter},
		},
		"re-run last look": {
			prepare: func(w *world) {
				w.ci, w.rerunErr = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}, errLongReason
			},
			keys: []string{"4", "R", keyEnter},
		},
		"merge picker": {
			prepare: func(w *world) {
				w.pull.Approvals, w.pull.Mergeable, w.mergeErr = 1, forge.MergeClean, errLongReason
			},
			keys: []string{"4", "M", keyEnter},
		},
		"finish preview": {
			prepare: func(w *world) { w.pull.State, w.finishErr = forge.StateMerged, errLongReason },
			keys:    []string{"4", "F", keyEnter},
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

func TestAFailedReEditShowsItsReasonFully(t *testing.T) {
	t.Parallel()

	// Arrange
	commenting := newWorld()
	commenting.edited = shortComment
	preview := typing(t, commenting.live(t, 80, 24), "c")
	commenting.editErr = errLongReason

	// Act
	view := typing(t, preview, "e").View().Content

	// Assert
	requireScreen(t, view, "Comment on "+issueKey, outcomeTail)
}

func TestAFailureAfterItsOverlayClosedOpensNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs past the harness's horizon, so w queues the post and closes the
	// preview; the next CI read passes and the post fails with nothing open.
	refusing := newWorld()
	refusing.ciInterval = 2 * time.Second
	refusing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	refusing.postErr = errNotInChannel

	// Act
	view := typing(t, refusing.live(t, 120, 40), "5", "p", "w").View().Content

	// Assert
	requireScreen(t, view, "✗ the credential was not accepted")
	refuseScreen(t, view, "Announce to")
}
