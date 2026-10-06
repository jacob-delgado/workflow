// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// reboundSentenceKeys moves every action a sentence on screen names to a key
// no default binds, where nothing else in its context answers it.
func reboundSentenceKeys() map[string]string {
	return map[string]string{
		"new-branch": "B", "apply": "ctrl+y", "set-up-lefthook": "L", "open-pull-request": "N",
		editAction: "E", "finish-branch": "X", "edit-body": "ctrl+l",
	}
}

func TestASentenceNamingAKeyNamesTheKeyItIsBoundTo(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		world  func() *world
		keys   []string
		want   string
		refuse string
	}{
		"a detached HEAD offers new-branch": {
			world: func() *world {
				detached := newWorld()
				detached.branch = gitrepo.Branch{Detached: true}

				return detached
			},
			keys:   []string{"2"},
			want:   "Check out a branch, or press B to start one for the selected issue.",
			refuse: "press b to start",
		},
		"a failed fetch offers apply": {
			world: func() *world {
				unfetched := newWorld()
				unfetched.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
				unfetched.fetchErr = errFetchFailed

				return unfetched
			},
			keys:   []string{"b", "ctrl+y"},
			want:   "could not fetch; ctrl+y branches from what you already have",
			refuse: "enter branches",
		},
		"an unmanaged hook offers set-up-lefthook": {
			world: func() *world {
				unmanaged := newWorld()
				unmanaged.gitHooks = legacyHooks()

				return unmanaged
			},
			keys:   []string{"3"},
			want:   "Press L to set up lefthook.",
			refuse: "Press g",
		},
		"no pull request offers open-pull-request": {
			world:  withoutPull,
			keys:   []string{"4"},
			want:   "N opens one from this branch's commits",
			refuse: nOpensOne,
		},
		"an open pull request offers edit": {
			world:  newWorld,
			keys:   []string{"4"},
			want:   "E edits its title and description.",
			refuse: "e edits",
		},
		"a merged pull request offers finish-branch": {
			world:  mergedBranch,
			keys:   []string{"4"},
			want:   "X finishes the branch",
			refuse: "F finishes",
		},
		"a merged pull request offers open-pull-request": {
			world:  mergedBranch,
			keys:   []string{"4"},
			want:   "N opens a new pull request from this branch's commits.",
			refuse: nOpensANewOne,
		},
		"the checks list offers apply": {
			world:  checkedCI,
			keys:   []string{"4", "c"},
			want:   "Open a check's page with ctrl+y.",
			refuse: "with enter",
		},
		"the commit composer offers edit-body": {
			world:  newWorld,
			keys:   []string{"3", "c"},
			want:   "no body yet: ctrl+l writes one in your editor",
			refuse: "ctrl+o writes",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rebound := tt.world()
			rebound.cfg.UI.Keys = reboundSentenceKeys()

			// Act
			view := typing(t, rebound.live(t, 160, 50), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse)
		})
	}
}

func TestTheSentencesReboundSetIsOneTheInterfaceAccepts(t *testing.T) {
	t.Parallel()

	// Act
	err := tui.CheckKeys(reboundSentenceKeys())
	// Assert
	if err != nil {
		t.Errorf("CheckKeys = %v, want the rebound keys the sentence tests use accepted", err)
	}
}
