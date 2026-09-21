// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errAmendExit is git failing the amend with a bare exit code.
var errAmendExit = errors.New("exit status 1")

// unpushedWorld is a repository with a staged change and one unpushed commit, so
// the amend and fixup actions are offered.
func unpushedWorld() *world {
	world := newWorld()
	world.branch.Ahead = 1

	return world
}

// twoUnpushedWorld has two unpushed commits, so the fixup picker has a choice.
func twoUnpushedWorld() *world {
	world := newWorld()
	world.branch.Commits = []gitrepo.Commit{
		{Hash: "aaa1111", Subject: "feat: the older change"},
		{Hash: "bbb2222", Subject: "feat: the newer change"},
	}
	world.branch.Ahead = 2

	return world
}

func TestAmendPreviewsBeforeItFolds(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A").View().Content

	// Assert
	// The preview names the commit and nothing is folded until it is confirmed.
	requireScreen(t, view, "Amend the last commit", "fold the staged changes into "+pullTitle)

	if calls := world.asked("amend"); len(calls) != 0 {
		t.Errorf("amended before the preview was confirmed: %q", calls)
	}
}

func TestAmendFoldsTheStagedChangesIntoTheLastCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "amended "+pullTitle)

	if calls := world.asked("amend"); len(calls) != 1 || calls[0] != "amend" {
		t.Errorf("amend calls = %q, want one amend", calls)
	}
}

func TestLeavingTheAmendPreviewFoldsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A", keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Amend the last commit")

	if calls := world.asked("amend"); len(calls) != 0 {
		t.Errorf("leaving the preview amended: %q", calls)
	}
}

func TestARefusedAmendLeadsWithThatStep(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()
	world.amendErr = errAmendExit

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "the amend was refused")
}

func TestFixupRecordsAFixupOfTheChosenCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "f", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "recorded a fixup! of "+pullTitle)

	if calls := world.asked("fixup"); len(calls) != 1 || calls[0] != "fixup 1a2b3c4" {
		t.Errorf("fixup calls = %q, want one fixup of the chosen commit", calls)
	}
}

func TestTheFixupPickerListsTheUnpushedCommitsNewestFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	world := twoUnpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "f").View().Content

	// Assert
	requireScreen(t, view, "Fold the staged changes into which commit?",
		"bbb2222 feat: the newer change", "aaa1111 feat: the older change")
}

func TestFixupChoosesTheOlderCommitByMovingDown(t *testing.T) {
	t.Parallel()

	// Arrange
	// The picker lists the commits newest first, so moving down once reaches the
	// older one.
	world := twoUnpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "f", "down", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "recorded a fixup! of feat: the older change")

	if calls := world.asked("fixup"); len(calls) != 1 || calls[0] != "fixup aaa1111" {
		t.Errorf("fixup calls = %q, want a fixup of the older commit", calls)
	}
}

func TestTheFixupPickerMovesBackUp(t *testing.T) {
	t.Parallel()

	// Arrange
	world := twoUnpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "f", "down", "up", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "recorded a fixup! of feat: the newer change")

	if calls := world.asked("fixup"); len(calls) != 1 || calls[0] != "fixup bbb2222" {
		t.Errorf("fixup calls = %q, want a fixup of the newest commit", calls)
	}
}

func TestLeavingTheFixupPickerRecordsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "f", keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Fold the staged changes into")

	if calls := world.asked("fixup"); len(calls) != 0 {
		t.Errorf("leaving the picker recorded a fixup: %q", calls)
	}
}

func TestAmendAndFixupAreNotOfferedOnAPushedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// newWorld is a pushed branch (nothing ahead), so its commit is not local to
	// rewrite.
	world := newWorld()

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A", "f").View().Content

	// Assert
	refuseScreen(t, view, "Amend the last commit", "Fold the staged changes into")

	if calls := append(world.asked("amend"), world.asked("fixup")...); len(calls) != 0 {
		t.Errorf("a pushed branch rewrote history: %q", calls)
	}
}

func TestAmendAndFixupNeedTheirSeams(t *testing.T) {
	t.Parallel()

	// Arrange
	world := unpushedWorld()
	world.noAmend = true
	world.noFixup = true

	// Act
	view := typing(t, world.live(t, 120, 40), "3", "A", "f").View().Content

	// Assert
	refuseScreen(t, view, "Amend the last commit", "Fold the staged changes into")
}

func TestADryRunAmendsAndFixesUpNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"amending":  {keys: []string{"3", "A", keyEnter}, want: "dry run: would amend " + pullTitle},
		"fixing up": {keys: []string{"3", "f", keyEnter}, want: "dry run: would fix up " + pullTitle},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dry := unpushedWorld()
			model := sized(t, dryInterface(dry), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, testCase.keys...).View().Content

			// Assert
			requireScreen(t, view, testCase.want)

			if calls := append(dry.asked("amend"), dry.asked("fixup")...); len(calls) != 0 {
				t.Errorf("a dry run rewrote history: %q", calls)
			}
		})
	}
}
