// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// keyAloneLine is the line that tracks PROJ-500 where the Issues pane has not
// read it: its key, its page and the link tag, with no summary or priority.
const keyAloneLine = "jiraid:" + untrackedIssue + " jiraurl:" + untrackedPage + " +jira -- " + untrackedIssue + ":"

// untrackedBranch is PROJ-500's branch.
const untrackedBranch = "feat/" + untrackedIssue + "-rotate"

// looseTaskNamed is how an offer that stops task 9, linked to no issue, names it.
const looseTaskNamed = "task 9 'Renew the cert'"

// switchToTheUntrackedIssue is a clean tree with a branch for PROJ-500, which the
// Issues pane does not list, to switch to; and the keys that switch to it.
func switchToTheUntrackedIssue(repo *world) []string {
	switchableTo(repo, untrackedBranch)

	return []string{"2", "s", keyEnter}
}

func TestSwitchingWithNoTaskStartedOffersToStartOne(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	switching := loopMoments()["switching tasks"]
	switching.arrange(repo)
	stopTheStartedTask(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-388's branch with no task started
	offered := typing(t, model, switching.keys...)

	// Assert: the offer is to start task 3, and nothing is written yet
	requireScreen(t, offered.View().Content, startLook, "Start task 3 '"+secondIssue+": Add retries'")
	refuseScreen(t, offered.View().Content, switchLook)
	requireTaskWrites(t, repo)

	// Act: start it
	typing(t, offered, keyEnter)

	// Assert: task 3 is started
	requireTaskWrites(t, repo, "task start "+trackedTaskUUID)
}

func TestARefusedSwitchStaysOpenAndStartsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	switching := loopMoments()["switching tasks"]
	switching.arrange(repo)
	repo.tasks.writeErr = hookRefused()
	offered := typing(t, repo.live(t, 200, 40), switching.keys...)

	// Act
	view := typing(t, offered, keyEnter).View().Content

	// Assert
	// Task 3 is started only once task 12 has stopped, so both are never started.
	// The refusal is pinned in the look, as well as told in the footer.
	requireScreen(t, view, switchLook)

	if told := strings.Count(plain(view), hookRefusal); told < 2 {
		t.Errorf("the refusal is told %d times, want it pinned in the look and in the footer:\n%s", told, plain(view))
	}

	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
}

func TestARetriedSwitchStartsTheTaskItsLastTryStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 stops but task 3 is refused; tried again, 12 is stopped already.
	repo := withTasks()
	switching := loopMoments()["switching tasks"]
	switching.arrange(repo)
	repo.tasks.answers = []error{nil, hookRefused(), taskwarrior.ErrNothingChanged, nil}
	refused := typing(t, typing(t, repo.live(t, 200, 40), switching.keys...), keyEnter)

	// Act
	view := typing(t, refused, keyEnter).View().Content

	// Assert
	stop, start := "task stop "+activeTaskUUID, "task start "+trackedTaskUUID
	requireTaskWrites(t, repo, stop, start, stop, start)
	requireScreen(t, view, "stopped 12 and started 3")
	refuseScreen(t, view, switchLook, nothingChanged)
}

func TestSwitchingToAnUntrackedIssueOffersToStopTheTaskThenTrackIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is started, on PROJ-412. PROJ-500 is not among the issues the Issues
	// pane lists, so the line that tracks it knows it by its key alone.
	repo := withTasks()
	keys := switchToTheUntrackedIssue(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-500's branch
	offered := typing(t, model, keys...)

	// Assert: the offer is to stop task 12 first, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+", then track and start "+untrackedIssue+"?")
	requireTaskWrites(t, repo)

	// Act: go ahead
	tracking := typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, and the line that tracks the issue opens
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
	requireScreen(t, tracking.View().Content, trackAndStart+untrackedIssue, keyAloneLine)
	refuseScreen(t, tracking.View().Content, switchLook)

	// Act: send it
	typing(t, tracking, keyEnter)

	// Assert: the task is added, annotated with the issue's page, then started
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID,
		"task add "+keyAloneLine, "task annotate "+addedTaskUUID+" "+untrackedPage, "task start "+addedTaskUUID)
}

func TestSwitchingToAListedIssueTracksItWithItsSummary(t *testing.T) {
	t.Parallel()

	// Arrange
	// No task is started, and the Issues pane lists PROJ-500.
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	keys := switchToTheUntrackedIssue(repo)
	stopTheStartedTask(repo)

	// Act
	view := typing(t, repo.live(t, 200, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, trackAndStart+untrackedIssue, untrackedLine)
}

func TestARefusedStopOpensNoTrackLine(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	keys := switchToTheUntrackedIssue(repo)
	repo.tasks.writeErr = hookRefused()
	offered := typing(t, repo.live(t, 200, 40), keys...)

	// Act: go ahead, and Taskwarrior refuses to stop task 12
	refused := typing(t, offered, keyEnter)

	// Assert: the look stays open with the reason, and nothing is tracked
	requireScreen(t, refused.View().Content, switchLook, hookRefusal)
	refuseScreen(t, refused.View().Content, trackAndStart)

	// Act: back out
	view := typing(t, refused, keyEsc).View().Content

	// Assert: the look closes, and the line that tracks the issue never opens
	refuseScreen(t, view, switchLook, trackAndStart)
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
}

func TestATaskStoppedAlreadyStillOpensTheTrackLine(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 was stopped elsewhere since the tasks were read, so stopping it
	// changes nothing: it is stopped all the same.
	repo := withTasks()
	keys := switchToTheUntrackedIssue(repo)
	repo.tasks.writeErr = taskwarrior.ErrNothingChanged
	offered := typing(t, repo.live(t, 200, 40), keys...)

	// Act
	view := typing(t, offered, keyEnter).View().Content

	// Assert
	requireScreen(t, view, trackAndStart+untrackedIssue, keyAloneLine)
	refuseScreen(t, view, switchLook)
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
}

func TestEveryOtherStartedTaskIsStoppedFirst(t *testing.T) {
	t.Parallel()

	// Task 9 is linked to no issue; task 12 is PROJ-412's.
	onlyNine := func(repo *world) { stopTheStartedTask(repo); startTheLooseTask(repo) }
	cases := map[string]struct {
		arrange func(repo *world)
		branch  string
		look    string
		writes  []string
		told    []string
	}{
		"a task for no issue, before task 3 starts": {
			arrange: onlyNine, branch: otherTaskBranch,
			look:   "Stop " + looseTaskNamed + " and " + startThree + "?",
			writes: []string{"task stop " + looseTaskUUID, "task start " + trackedTaskUUID},
			told:   []string{"stopped 9 and started 3"},
		},
		"a task for no issue, before PROJ-500 is tracked": {
			arrange: onlyNine, branch: untrackedBranch,
			look:   "Stop " + looseTaskNamed + ", then track and start " + untrackedIssue + "?",
			writes: []string{"task stop " + looseTaskUUID},
			told:   []string{"stopped 9", trackAndStart + untrackedIssue},
		},
		"two tasks, before task 3 starts": {
			arrange: startTheLooseTask, branch: otherTaskBranch,
			look:   stopTwelve + " and " + looseTaskNamed + " and " + startThree + "?",
			writes: []string{"task stop " + activeTaskUUID, "task stop " + looseTaskUUID, "task start " + trackedTaskUUID},
			told:   []string{"stopped 12 and 9 and started 3"},
		},
		"two tasks, before PROJ-500 is tracked": {
			arrange: startTheLooseTask, branch: untrackedBranch,
			look:   stopTwelve + " and " + looseTaskNamed + ", then track and start " + untrackedIssue + "?",
			writes: []string{"task stop " + activeTaskUUID, "task stop " + looseTaskUUID},
			told:   []string{"stopped 12 and 9", trackAndStart + untrackedIssue},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			tt.arrange(repo)
			switchableTo(repo, tt.branch)
			model := repo.live(t, 240, 40)

			// Act: switch to the issue's branch
			offered := typing(t, model, "2", "s", keyEnter)

			// Assert: the look names every started task, and nothing is written yet
			requireScreen(t, offered.View().Content, switchLook, tt.look)
			requireTaskWrites(t, repo)

			// Act: go ahead
			view := typing(t, offered, keyEnter).View().Content

			// Assert: each is stopped in turn before the issue's task starts, or before
			// the line that tracks the issue opens
			requireTaskWrites(t, repo, tt.writes...)
			requireScreen(t, view, tt.told...)
		})
	}
}

func TestStoppingThenTrackingIsHeldBackUnderDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// Dry run is turned on as PROJ-500's branch is created, as above, with task 12
	// started.
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	repo.moves = startTransitions()
	repo.branch.Base = ""
	model := typing(t, repo.live(t, 240, 40), append(selectTheUntrackedIssue(), "b")...)
	creating, create := pressed(t, model, keyEnter)
	offered := typing(t, drain(t, creating.WithDryRun(), create), keyEsc)

	// Act: go ahead
	tracking := typing(t, offered, keyEnter)

	// Assert: it says it would stop task 12, and the line that tracks the issue
	// opens all the same
	requireScreen(t, tracking.View().Content, "dry run: would stop task 12", trackAndStart+untrackedIssue)
	requireTaskWrites(t, repo)

	// Act: send the line
	view := typing(t, tracking, keyEnter).View().Content

	// Assert: it says it would add the task, annotate it and start it, and writes
	// nothing
	requireScreen(t, view,
		"dry run: task add "+untrackedLine+" then annotate it with "+untrackedPage+" and start it")
	requireTaskWrites(t, repo)
}
