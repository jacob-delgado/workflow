// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestTheComposerOpensWithReviewersAssigneesAndLabels(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	// Two tabs reach the reviewers field, then each field is filled in turn.
	keys := slices.Concat(
		[]string{"4", "n", keyTab, keyTab}, letters("ana, ben"),
		[]string{keyTab}, letters("cass"),
		[]string{keyTab}, letters("bug"), []string{keyEnter},
	)

	// Act
	typing(t, model, keys...)

	// Assert
	calls := opening.asked("open ")
	if len(calls) != 1 || !strings.Contains(calls[0], "reviewers=ana,ben assignees=cass labels=bug") {
		t.Errorf("open calls = %q, want the reviewers, assignees and labels", calls)
	}
}

func TestTheComposerSuggestsReviewersFromCodeOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.codeOwners = []string{"ana", "ben"}
	model := opening.live(t, 120, 40)

	// Act
	composer := typing(t, model, "4", "n")

	// Assert
	requireScreen(t, composer.View().Content, "reviewers > ana, ben")

	if calls := opening.asked("code-owners"); len(calls) != 1 {
		t.Errorf("CODEOWNERS reads = %q, want one", calls)
	}
}

func TestAPullThatOpensButCannotAddReviewersIsNotLost(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull opens, but the forge knows no reviewer by that name.
	opening := withoutPull()
	opening.reviewerErr = fmt.Errorf("%w: ana", forge.ErrNoUser)
	model := opening.live(t, 200, 40)

	keys := append([]string{"4", "n", keyTab, keyTab}, letters("ana")...)

	// Act
	opened := typing(t, model, append(keys, keyEnter)...)

	// Assert
	requireScreen(t, opened.View().Content, "opened #42",
		"could not add every reviewer, assignee or label: the forge has no such user")
	refuseScreen(t, opened.View().Content, "┏━ Open pull request")
}

func TestShiftTabStepsBackwardThroughTheComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	// Act
	composer := typing(t, model, "4", "n", keyShiftTab)

	// Assert
	requireScreen(t, composer.View().Content, "▸ labels")
}
