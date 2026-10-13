// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"regexp"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errLefthookConfig is lefthook unable to read its configuration.
var errLefthookConfig = errors.New("couldn't load config: yaml: line 3")

// hookPickerTitle is the hook picker's title, as its border draws it.
const hookPickerTitle = "┏━ Run a hook"

// prePushRun is a run of pre-push, as its border titles it, and askedPrePush
// the call that runs it.
const (
	prePushRun   = "┏━ pre-push"
	askedPrePush = "hook pre-push"
)

func TestHListsTheHooksLefthookRunsByHandAndThoseOnlyGitRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h").View().Content

	// Assert
	requireScreen(t, view, hookPickerTitle, "▸ pre-commit", "  pre-push", "Only git runs these:",
		"commit-msg · "+commitMsgReason)
	requireScreen(t, footerLine(view), "enter run", "esc close")

	if calls := hooked.asked("hook"); len(calls) != 0 {
		t.Errorf("opening the picker ran %q", calls)
	}
}

func TestHThenEnterRunsPreCommitAloneWithoutCommitting(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.commitLines = []string{lintJobStarts, "summary: (done in 1.0 seconds)", "✔️ lint (1.0 seconds)"}
	model := hooked.live(t, 120, 40)

	// Act: run the hook chosen first
	passed := typing(t, model, "3", "h", keyEnter)

	// Assert: it ran, alone, and says how it went
	requireScreen(t, passed.View().Content, "┏━ pre-commit", "● done", "● lint", "r run again")

	if calls := hooked.asked("hook"); !slices.Equal(calls, []string{"hook pre-commit"}) {
		t.Errorf("hook calls = %q", calls)
	}

	if calls := hooked.asked("commit"); len(calls) != 0 {
		t.Errorf("running the hook committed: %q", calls)
	}

	// Act: close the run
	closed := typing(t, passed, keyEsc).View().Content

	// Assert: the keyboard is back on the Commits pane, whose heavy border is on
	// the detail where the cursor is.
	refuseScreen(t, closed, "┏━ pre-commit", hookPickerTitle)
	requireScreen(t, closed, focused("Commits"))
}

func TestChoosingPrePushRunsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h", keyDown, keyEnter).View().Content

	// Assert
	requireScreen(t, view, prePushRun)

	if calls := hooked.asked("hook"); !slices.Equal(calls, []string{askedPrePush}) {
		t.Errorf("hook calls = %q, want pre-push alone", calls)
	}
}

func TestWithoutPreCommitTheFirstHookIsChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.lefthookHooks = hooks.Configured{Runnable: []string{prePushHook, "validate"}, GitOnly: nil}

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h").View().Content

	// Assert
	requireScreen(t, view, "▸ pre-push", "  validate")
	refuseScreen(t, view, "Only git runs these:")
}

func TestAHookOnlyGitRunsCannotBeChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()

	// Act
	typing(t, hooked.live(t, 120, 40), "3", "h", keyDown, keyDown, keyDown, keyEnter)

	// Assert
	if calls := hooked.asked("hook"); !slices.Equal(calls, []string{askedPrePush}) {
		t.Errorf("hook calls = %q, want the choice stopped on pre-push, the last that can run", calls)
	}
}

func TestRunAgainRunsTheSameHook(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	ran := typing(t, hooked.live(t, 120, 40), "3", "h", keyDown, keyEnter)

	// Act
	typing(t, ran, "r")

	// Assert
	if calls := hooked.asked("hook"); !slices.Equal(calls, []string{askedPrePush, askedPrePush}) {
		t.Errorf("hook calls = %q, want pre-push run twice", calls)
	}
}

func TestAFailedHookSaysWhichHookFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.commitErr = gitExited(1)

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h", keyDown, keyEnter).View().Content

	// Assert
	requireScreen(t, view, prePushRun, "✗ the pre-push hook failed")
}

func TestThePickerSaysItIsReadingTheHooks(t *testing.T) {
	t.Parallel()

	// Arrange
	commits := typing(t, newWorld().live(t, 120, 40), "3")

	// Act
	reading, _ := pressed(t, commits, "h")

	// Assert
	view := reading.View().Content
	requireScreen(t, view, hookPickerTitle, "reading lefthook's configuration…")
	refuseScreen(t, footerLine(view), "enter run")
}

func TestThePickerSaysWhyLefthooksConfigurationCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.lefthookErr = errLefthookConfig
	picker := typing(t, hooked.live(t, 120, 40), "3", "h")

	// Act
	pressedEnter := typing(t, picker, keyEnter)

	// Assert
	view := pressedEnter.View().Content
	requireScreen(t, view, hookPickerTitle, "couldn't load config: yaml: line 3")
	requireScreen(t, footerLine(view), "esc close")
	refuseScreen(t, footerLine(view), "enter run")

	if calls := hooked.asked("hook"); len(calls) != 0 {
		t.Errorf("enter ran %q with no hook listed", calls)
	}
}

func TestThePickerSaysWhenNoHookCanBeRunByHand(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.lefthookHooks = hooks.Configured{
		Runnable: nil, GitOnly: []hooks.GitOnly{{Name: commitMsgHook, Reason: commitMsgReason}},
	}

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "None of lefthook's hooks here can be run by hand.", "commit-msg · "+commitMsgReason)

	if calls := hooked.asked("hook"); len(calls) != 0 {
		t.Errorf("enter ran %q with no hook to run by hand", calls)
	}
}

func TestThePickerSaysWhenLefthookConfiguresNoHook(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.lefthookHooks = hooks.Configured{}

	// Act
	view := typing(t, hooked.live(t, 120, 40), "3", "h").View().Content

	// Assert
	requireScreen(t, view, "lefthook configures no hook here.")
	refuseScreen(t, view, "Only git runs these:")
}

func TestClickingAHookChoosesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	picker := typing(t, newWorld().live(t, 120, 40), "3", "h")
	row := screenRow(t, picker.View().Content, "  pre-push")

	// Act
	clicked := click(t, picker, 60, row)

	// Assert
	requireScreen(t, clicked.View().Content, "▸ pre-push")
}

func TestAHookListAnsweredAfterThePickerClosedIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	commits := typing(t, newWorld().live(t, 120, 40), "3")
	reading, read := pressed(t, commits, "h")
	closed := typing(t, reading, keyEsc)

	// Act
	answered, _ := closed.Update(read())

	// Assert
	refuseScreen(t, concrete(t, answered).View().Content, hookPickerTitle)
}

func TestAHookListAnsweringAnEarlierOpeningIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	commits := typing(t, newWorld().live(t, 120, 40), "3")
	first, firstRead := pressed(t, commits, "h")
	reopened, _ := pressed(t, typing(t, first, keyEsc), "h")

	// Act
	answered, _ := reopened.Update(firstRead())

	// Assert
	requireScreen(t, concrete(t, answered).View().Content, "reading lefthook's configuration…")
}

func TestHIsNotOfferedWithoutAWayToListTheHooks(t *testing.T) {
	t.Parallel()

	// Arrange
	unlisted := newWorld()
	deps := unlisted.deps()
	deps.Hooks.Configured = nil
	model := sized(t, tui.New(unlisted.cfg, nil, deps), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "3", "h").View().Content

	// Assert
	refuseScreen(t, footerLine(typing(t, model, "3").View().Content), "run a hook")
	refuseScreen(t, view, hookPickerTitle)

	if calls := unlisted.asked("hook"); len(calls) != 0 {
		t.Errorf("h ran %q with no way to list the hooks", calls)
	}
}

func TestAHookOnlyGitRunsWrapsUnderItsName(t *testing.T) {
	t.Parallel()

	// Arrange
	narrow := newWorld().live(t, 80, 24)

	// Act
	view := plain(typing(t, narrow, "3", "h").View().Content)

	// Assert
	if !wrappedUnderItsName.MatchString(view) {
		t.Errorf("the reason's last word is not set in under commit-msg:\n%s", view)
	}
}

// wrappedUnderItsName matches the line commit-msg's reason wraps onto at 80
// columns, set in two spaces from the pane's edge.
var wrappedUnderItsName = regexp.MustCompile(`(?m)[│┃]  message\s*$`)
