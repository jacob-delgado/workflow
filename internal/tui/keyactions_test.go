// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The action and the help group the listing's cases read.
const (
	commentAction = "comment"
	issuesGroup   = "Issues"
)

// actionNamed is the listed action of a name, failing the test when none is.
func actionNamed(t *testing.T, listed []seams.KeyAction, name string) seams.KeyAction {
	t.Helper()

	index := slices.IndexFunc(listed, func(action seams.KeyAction) bool { return action.Action == name })
	if index < 0 {
		t.Fatalf("KeyActions lists no %q", name)
	}

	return listed[index]
}

func TestKeyActionsListsAnActionAsTheHelpDoes(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Assert
	got := actionNamed(t, listed, commentAction)

	want := seams.KeyAction{
		Action: commentAction, Help: commentAction, Group: issuesGroup, Shown: "c", Keys: []string{"c"},
	}
	if got.Action != want.Action || got.Help != want.Help || got.Group != want.Group || got.Shown != want.Shown ||
		!slices.Equal(got.Keys, want.Keys) {
		t.Errorf("comment = %+v, want %+v", got, want)
	}
}

func TestKeyActionsAppliesAKeyOverride(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", map[string]string{commentAction: "C"})

	// Assert
	got := actionNamed(t, listed, commentAction)
	if got.Shown != "C" || !slices.Equal(got.Keys, []string{"C"}) {
		t.Errorf("comment = %+v, want it shown and bound on C", got)
	}
}

func TestKeyActionsNamesTheDefaultKeyAnOverrideMoved(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", map[string]string{commentAction: "C"})

	// Assert
	if got := actionNamed(t, listed, commentAction); got.Default != "c" {
		t.Errorf("comment's default = %q, want c, the key it has with no override", got.Default)
	}
}

func TestKeyActionsNamesTheGroupsForTheForgeAndTheService(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("merge request", "Teams", nil)

	// Assert
	opening := actionNamed(t, listed, "open-pull-request")
	if opening.Help != "open merge request" || opening.Group != reviewGroup {
		t.Errorf("open-pull-request = %+v, want it named for the forge, under the Review pane's group", opening)
	}

	posting := actionNamed(t, listed, "post")
	if posting.Help != "announce to Teams" || posting.Group != "Teams" {
		t.Errorf("post = %+v, want it named for the service, under the service's pane's group", posting)
	}
}

func TestKeyActionsListsThePaneNumbersAsOneAction(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Assert
	got := actionNamed(t, listed, "jump-to-pane")
	if got.Shown != "1-9" || len(got.Keys) != 9 || got.Keys[0] != "1" || got.Keys[8] != "9" {
		t.Errorf("jump-to-pane = %+v, want shown as 1-9 and bound on each pane number", got)
	}
}

func TestKeyActionsLeavesOutALineTheHelpDoesNotDraw(t *testing.T) {
	t.Parallel()

	// Act
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Assert
	if slices.ContainsFunc(listed, func(action seams.KeyAction) bool { return action.Action == "cycle-type-right" }) {
		t.Error("KeyActions lists cycle-type-right, which rides cycle-type-left's line in the help")
	}
}
