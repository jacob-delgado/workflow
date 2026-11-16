// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldOnto is deps whose read of the branch onto a base waits on held.
func heldOnto(deps *tui.Deps, held *hold) {
	read := deps.Git.BranchOnto
	deps.Git.BranchOnto = func(ctx context.Context, base string) (gitrepo.Branch, error) {
		held.wait()

		return read(ctx, base)
	}
}

// heldNothing is deps left as they are, for a composer whose reads all
// answer.
func heldNothing(*tui.Deps, *hold) {}

func TestTheComposerShowsEachOfItsStages(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		world          func() *world
		wrap           func(deps *tui.Deps, held *hold)
		keys           []string
		want, unwanted []string
		footer         string
	}{
		"reading what it starts from": {
			world: func() *world { return ownedBy("ana") }, wrap: heldOwners, keys: []string{"4", "n"},
			want: []string{"reading what the pull request starts from"}, unwanted: []string{"> main"},
			footer: escCloses,
		},
		"asking where it goes": {
			world: withoutPull, wrap: heldNothing, keys: []string{"4", "n"},
			want: []string{"Base", "> main"}, unwanted: []string{"Reviewers"}, footer: "enter next",
		},
		"reading the branch onto the base chosen": {
			world: ontoRelease, wrap: heldOnto, keys: append(baseKeys("release"), keyEnter),
			want: []string{"reading what the pull request starts from"}, unwanted: []string{"Title"},
			footer: escCloses,
		},
		"asking what it says": {
			world: withoutPull, wrap: heldNothing, keys: []string{"4", "n", keyEnter},
			want: []string{"Title", "Reviewers"}, unwanted: []string{"reading what"}, footer: "enter open",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			held := newHold(t)
			started := heldLive(t, held, test.world(), test.wrap)
			held.armed.Store(true)

			// Act
			view := holding(t, held, started, test.keys...).View().Content

			// Assert
			requireScreen(t, view, test.want...)
			refuseScreen(t, view, test.unwanted...)

			if footer := footerLine(view); !strings.Contains(footer, test.footer) {
				t.Errorf("the footer = %q, want %q", footer, test.footer)
			}
		})
	}
}

func TestAPasteWhileTheBranchIsReadOntoItsBaseChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHold(t)
	started := heldLive(t, held, ontoRelease(), heldOnto)
	held.armed.Store(true)
	reading := holding(t, held, started, append(baseKeys("release"), keyEnter)...)
	held.requireReached(t)

	// Act
	pastedOver := holding(t, held, reading, pasted("-typed-while-reading"))

	// Assert
	view := pastedOver.View().Content
	requireScreen(t, view, "→ release", "reading what the pull request starts from")
	refuseScreen(t, view, "typed-while-reading")
}
