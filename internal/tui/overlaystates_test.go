// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"net/http"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// typed is text typed a character a step, followed by more steps.
func typed(text string, after ...string) []string {
	return append(letters(text), after...)
}

// downBy is the down action pressed rows times.
func downBy(rows int) []string {
	return slices.Repeat([]string{downAction}, rows)
}

// withChannels is a world whose messaging offers a second channel to cycle
// to.
func withChannels(made func() *world) func() *world {
	return func() *world {
		repo := made()
		repo.cfg.Messaging.Channels = []string{teamChannel}

		return repo
	}
}

// editedTo is a world whose editor hands back a comment.
func editedTo() *world {
	repo := newWorld()
	repo.edited = shortComment

	return repo
}

// postWaitingForCI is a pull request whose CI still runs, so an
// announcement can wait for it.
func postWaitingForCI() *world {
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}

	return repo
}

// unreadableLocalData is a store whose directory cannot be read.
func unreadableLocalData() *world {
	repo := newWorld()
	repo.localDataErr = errStoreUnreadable

	return repo
}

// unreadableSettings is a configuration that cannot be read.
func unreadableSettings() *world {
	repo := newWorld()
	repo.readSettingsErr = errStoreUnreadable

	return repo
}

// switchingThreeWays is a clean tree with three other task branches, so the
// switcher's cursor can stand between two.
func switchingThreeWays() *world {
	switching := switchingTwoWays()
	switching.branches = append(switching.branches, "fix/PROJ-501-fix-metrics")

	return switching
}

// twoTemplates is a branch with no pull request, whose repository offers two
// pull request templates.
func twoTemplates() *world {
	templated := templatedWorld()
	templated.templates = append(templated.templates, forge.Template{Name: "regression", Body: "## What broke\n"})

	return templated
}

// fieldfulStatus is an issue whose one move asks for a user, a date and fix
// versions.
func fieldfulStatus() *world {
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}

	return moving
}

// hookRunning is the pre-commit hook, chosen in the hook picker, running and
// not yet done: its start is answered, and its output never comes.
func hookRunning(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model {
	t.Helper()

	repo := newWorld()
	repo.runBlocks = true
	repo.cfg.UI.Keys = keysOf(moved)
	picking := stepping(t, repo.live(t, overlayWidth, overlayHeight), moved, "3", "run-hook")
	starting, start := picking.Update(moved[applyAction])
	running, _ := finish(t, concrete(t, starting), start)

	return running
}

// withNoFile is a session with no configuration file, which offers to set
// one up.
func withNoFile(t *testing.T, moved map[string]tea.KeyPressMsg) tui.Model {
	t.Helper()

	return newFirstRun(t, http.StatusOK).keyed(t, keysOf(moved))
}

// committedX is the steps that open the commit composer, type x and commit
// it.
func committedX(after ...string) []string {
	return slices.Concat([]string{"3", commitAction}, typed("x", applyAction), after)
}

// writtenOnTwoLines is the steps that open the comment box, write two lines
// and go back to normal mode, the cursor on the last character.
func writtenOnTwoLines(after ...string) []string {
	return slices.Concat([]string{"1", commentAction, "insert"}, typed("ab", keyEnter), typed("cd", keyEsc), after)
}

// settingsAt is the steps that open Settings with the cursor on a row.
func settingsAt(row int, after ...string) []string {
	return slices.Concat([]string{reposKey, "settings"}, downBy(row), after)
}

// overlayStates are every overlay, by the name of its key context, in each
// state that answers an action its others leave out. A list stands between
// its ends in one state or at either end in two, so every way it moves
// shows.
func overlayStates() map[string][]overlayState {
	return statesOf(
		named("the amend preview", stateAt(unpushedWorld, "3", "amend")),
		named("the branch creator", stateAt(newWorld, "2", "new-branch")),
		named("the link form",
			stateAt(linkedBranch, "2", "link-issue"),
			stateAt(func() *world { return onOffConventionBranch(true) }, "2", "link-issue")),
		named("the branch picker", stateAt(switchingThreeWays, "2", "switch-branch", downAction)),
		named("the calendar", stateAt(summaryWorld, summaryKey, "calendar")),
		named("a checklist", stateAt(placesWorld, "1", "filter-issues", downAction)),
		named("the checks list",
			stateAt(checkedCI, "4", "checks"),
			stateAt(checkedCI, "4", "checks", downAction)),
		named("a running command",
			stateFrom(hookRunning),
			stateAt(failingLint, committedX()...),
			stateAt(failingLint, committedX(downAction)...)),
		named("the comment composer",
			stateAt(newWorld, "1", commentAction),
			stateAt(newWorld, "1", commentAction, "insert"),
			stateAt(newWorld, writtenOnTwoLines()...),
			stateAt(newWorld, writtenOnTwoLines(upAction, "cursor-left")...)),
		named("the comment preview", stateAt(editedTo, "1", commentAction, "edit-body", applyAction)),
		named("the commit composer", stateAt(newWorld, "3", commitAction, "next-field")),
		named("the directory prompt", stateAt(reposWorld, reposKey, "go-to-directory")),
		named("the finish preview", stateAt(mergedBranch, "4", "finish-branch")),
		named("the fixup picker",
			stateAt(twoUnpushedWorld, "3", "fixup"),
			stateAt(twoUnpushedWorld, "3", "fixup", downAction)),
		named("the help",
			stateAt(newWorld, "toggle-help"),
			stateAt(newWorld, "toggle-help", downAction, "scroll-down")),
		named("the lefthook offer", stateAt(foldingWithHooks, "3", "set-up-lefthook")),
		named("the hook picker",
			stateAt(newWorld, "3", "run-hook"),
			stateAt(newWorld, "3", "run-hook", downAction)),
		named("the Jira link offer", stateAt(unlistedIssue, "4", "open-pull-request", applyAction)),
		named("the assign or log-work form", stateAt(newWorld, "1", "assign")),
		named("a job's log",
			stateAt(withALongLog, "4", "checks", downAction, "show-log"),
			stateAt(withALongLog, "4", "checks", downAction, "show-log", "first")),
		named("a last look", stateAt(unpushedWorld, "2", pushAction)),
		named("Local data",
			stateAt(newWorld, reposKey, "local-data"),
			stateAt(unreadableLocalData, reposKey, "local-data")),
		named("the merge picker",
			stateAt(mergeableTwoWays, "4", mergeAction),
			stateAt(mergeableTwoWays, "4", mergeAction, downAction)),
		named("the announcement preview",
			stateAt(withChannels(taggingWorld), "5", "post", downAction),
			stateAt(withChannels(taggingWorld), slices.Concat([]string{"5", "post"}, downBy(4))...)),
		named("the owner picker", stateAt(taggingWorld, "5", "post", "link-to-slack")),
		named("People and groups",
			stateAt(taggingWorld, "5", "people-and-groups", downAction),
			stateAt(taggingWorld, "5", "people-and-groups", "next-field")),
		named("the pull request composer", stateAt(twoTemplates, "4", "open-pull-request")),
		named("the pull request editor", stateAt(newWorld, "4", editAction)),
		named("the quit guard", stateAt(postWaitingForCI, "5", "post", postWhenGreenAction, "quit")),
		named("Settings",
			stateAt(newWorld, settingsAt(tokenRow)...),
			stateAt(newWorld, settingsAt(markdownRow)...),
			stateAt(newWorld, settingsAt(messagingKindRow)...),
			stateAt(newWorld, settingsAt(tokenRow, applyAction)...),
			stateAt(unreadableSettings, reposKey, "settings")),
		named("the setup form",
			stateFrom(withNoFile, applyAction),
			stateFrom(withNoFile, applyAction, downAction),
			stateFrom(withNoFile, applyAction, applyAction)),
		named("the status picker",
			stateAt(choosingStatus, "1", "change-status", downAction),
			stateAt(resolvingIssue, "1", "change-status", applyAction),
			stateAt(fieldfulStatus, slices.Concat([]string{"1", "change-status", applyAction},
				typed("fred", applyAction), typed("2026-09-21", applyAction))...)),
		named("the summary's post preview", stateAt(withChannels(summaryWorld), summaryKey, "post-summary")),
		named("a task form", stateAt(withTasks, tasksPane, addTaskAction)),
	)
}

// namedStates is an overlay's states, by the name of its key context.
type namedStates struct {
	name   string
	states []overlayState
}

// named is the states of the overlay whose key context is name.
func named(name string, states ...overlayState) namedStates {
	return namedStates{name: name, states: states}
}

// statesOf is each overlay's states by the name of its key context.
func statesOf(overlays ...namedStates) map[string][]overlayState {
	byName := make(map[string][]overlayState, len(overlays))
	for _, overlay := range overlays {
		byName[overlay.name] = append(byName[overlay.name], overlay.states...)
	}

	return byName
}
