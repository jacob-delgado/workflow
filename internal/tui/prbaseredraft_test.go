// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// releaseOnly is the subject of the one commit the branch holds past release.
const releaseOnly = "fix: only past release"

// ontoRelease is a world whose branch, read onto release, holds one commit of
// its own, and whose code owners own every path.
func ontoRelease() *world {
	opening := withoutPull()
	opening.codeOwners = []string{"ana"}

	onto := opening.branch
	onto.Base = "origin/release"
	onto.Commits = []gitrepo.Commit{{Hash: "9z8y7x6", Subject: releaseOnly}}
	opening.branchesOnto = map[string]gitrepo.Branch{"release": onto}

	return opening
}

func TestTheComposerDraftsAgainWhenItsBaseChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	// Act
	view := typing(t, opening.live(t, 120, 40), append(baseKeys("release"), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "> "+releaseOnly)

	if !slices.Equal(opening.asked("branch-onto"), []string{"branch-onto release"}) ||
		!slices.Contains(opening.asked("changed-paths"), "changed-paths release") {
		t.Errorf("the composer asked %q, want the branch read onto release and its owners", opening.calls)
	}
}

func TestATitleTypedOverStaysWhenTheBaseChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	// The title is typed over, then shift+tab goes back to the base.
	keys := []string{"4", "n", keyEnter, keyBackspace, "!", keyShiftTab, keyClearLine, pasted("release"), keyEnter}

	// Act
	view := typing(t, opening.live(t, 120, 40), keys...).View().Content

	// Assert
	typed := pullTitle[:len(pullTitle)-1] + "!"
	requireScreen(t, view, "> "+typed)
	refuseScreen(t, view, "> "+releaseOnly)
}

func TestTheComposerKeepsItsDraftOntoABaseNothingHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	// Act
	view := typing(t, opening.live(t, 120, 40), append(baseKeys("nonesuch"), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "> "+pullTitle, "→ nonesuch")
}

func TestLeavingTheBaseUnchangedReadsNothingAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	// Act
	typing(t, opening.live(t, 120, 40), "4", "n", keyEnter)

	// Assert
	if asked := opening.asked("branch-onto"); len(asked) != 0 {
		t.Errorf("the composer read the branch again, %q, though its base never changed", asked)
	}
}

func TestAKeptDraftReopensOntoTheBaseItChose(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	keys := append(baseKeys("release"), keyEnter, keyEsc, "n", keyEnter)

	// Act
	view := typing(t, opening.live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, "> "+releaseOnly)

	if asked := opening.asked("branch-onto"); len(asked) != 1 {
		t.Errorf("the kept draft read the branch onto its base again, %q, though it was drafted onto it", asked)
	}
}

func TestReviewersTypedStayWhenTheBaseChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := ontoRelease()

	// Past the base, tab twice reaches the reviewers; shift+tab three times
	// comes back to the base.
	keys := []string{
		"4", "n", keyEnter, keyTab, keyTab, pasted("zed"), keyShiftTab, keyShiftTab, keyShiftTab,
		keyClearLine, pasted("release"), keyEnter,
	}

	// Act
	view := typing(t, opening.live(t, 120, 40), keys...).View().Content

	// Assert
	// The owners filled ana, and zed was typed after them.
	requireScreen(t, view, "> anazed", "> "+releaseOnly)
}
