// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// The titles of the offers each moment of the loop makes.
const (
	startLook     = "Start the task"
	switchLook    = "Switch the task"
	noteLook      = "Note the pull request"
	completeLook  = "Mark the task done"
	trackAndStart = "Track and start "
)

// stopTwelve is how an offer that stops task 12, started on PROJ-412, names it.
const stopTwelve = "Stop task 12 '" + issueKey + ": " + issueSummary + "'"

// startThree is how an offer that starts task 3, which tracks PROJ-388, names it.
const startThree = "start task 3 '" + secondIssue + ": Add retries'"

// hookRefusal is the words of a refusal by one of Taskwarrior's hooks.
const hookRefusal = "Hook refused: timewarrior is not installed"

// hookRefused is Taskwarrior refusing a change, in hookRefusal's words.
func hookRefused() error {
	return fmt.Errorf("%w: %s", taskwarrior.ErrRefused, hookRefusal)
}

// offerTitles is every offer's title, for a test that none opens.
func offerTitles() []string {
	return []string{startLook, switchLook, noteLook, completeLook, trackAndStart}
}

// withTasksToStart is withTasks with Jira offering the moves that start an
// issue, so a branch for PROJ-388, not started yet, opens the status picker.
func withTasksToStart() *world {
	repo := withTasks()
	repo.moves = startTransitions()

	return repo
}

// stopTheStartedTask leaves no task started: task 12 is stopped.
func stopTheStartedTask(repo *world) {
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Start = time.Time{} })
}

// startTheLooseTask starts task 9, which is linked to no issue.
func startTheLooseTask(repo *world) {
	changeTask(repo, looseTaskUUID, func(task *taskwarrior.Task) { task.Start = testNow().Add(-time.Hour) })
}

// branchTheSecondIssue is the keys that create a branch for PROJ-388, the
// second issue listed.
func branchTheSecondIssue() []string {
	return []string{downAction, "b", keyEnter}
}

// readyToMerge makes the world's pull request green, approved and clean.
func readyToMerge(repo *world) {
	repo.pull.Approvals = 1
	repo.pull.Mergeable = forge.MergeClean
}

// switchableTo is a clean tree with one other branch to switch to.
func switchableTo(repo *world, branch string) {
	repo.changes = nil
	repo.branches = []string{featureName, branch}
}

// moment is a moment of the loop that offers a change of a task: how the world
// is set up to reach it, the keys that reach it and leave whatever the loop
// itself asks there, and a call that shows it was reached.
type moment struct {
	arrange func(repo *world)
	keys    []string
	reached string
}

// loopMoments is every moment of the loop that offers a change of a task.
func loopMoments() map[string]moment {
	return map[string]moment{
		"creating a branch": {
			arrange: func(repo *world) { repo.moves = startTransitions() },
			keys:    append(branchTheSecondIssue(), keyEsc), reached: "transitions " + secondIssue,
		},
		"switching branches": {
			arrange: func(repo *world) { switchableTo(repo, otherTaskBranch) },
			keys:    []string{"2", "s", keyEnter}, reached: "checkout " + otherTaskBranch,
		},
		"opening a pull request": {
			arrange: func(repo *world) { repo.pullFound = false },
			keys:    []string{"4", "n", keyEnter, keyEsc}, reached: "open ",
		},
		"merging": {arrange: readyToMerge, keys: []string{"4", "M", keyEnter}, reached: "merge 42"},
		"moving the issue to done": {
			arrange: func(repo *world) { repo.moves = workflowMoves() },
			keys:    []string{"t", downAction, keyEnter}, reached: "transition " + issueKey + " 31",
		},
	}
}

func TestCreatingABranchForATrackedIssueOffersToStartItsTask(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is started, on PROJ-412: one task runs at a time, so it is stopped
	// before task 3 starts.
	repo := withTasksToStart()
	model := repo.live(t, 200, 40)

	// Act: create a branch for PROJ-388, which task 3 tracks
	branched := typing(t, model, branchTheSecondIssue()...)

	// Assert: the status picker has the keys, and the offer waits behind it
	requireScreen(t, branched.View().Content, pickerTitle)
	refuseScreen(t, branched.View().Content, switchLook)

	// Act: back out of the status picker
	offered := typing(t, branched, keyEsc)

	// Assert: the offer names both tasks, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+" and "+startThree+"?")
	requireTaskWrites(t, repo)

	// Act: switch
	started := typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, then task 3 started, and the offer closes saying
	// so
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID, "task start "+trackedTaskUUID)
	requireScreen(t, started.View().Content, "stopped 12 and started 3")
	refuseScreen(t, started.View().Content, switchLook)
}

func TestBackingOutOfAnOfferWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasksToStart()
	model := repo.live(t, 200, 40)

	// Act: create a branch for PROJ-388 and back out of the status picker
	offered := typing(t, model, append(branchTheSecondIssue(), keyEsc)...)

	// Assert: the offer is open
	requireScreen(t, offered.View().Content, switchLook)

	// Act: back out of the offer
	view := typing(t, offered, keyEsc).View().Content

	// Assert: it closes, and nothing is written
	refuseScreen(t, view, switchLook)
	requireTaskWrites(t, repo)
}

func TestCreatingABranchForAnUntrackedIssueOffersToTrackAndStart(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is started, on PROJ-412, so it is stopped before the new task is.
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	repo.moves = startTransitions()
	model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

	// Act: create its branch and back out of the status picker
	offered := typing(t, model, "b", keyEnter, keyEsc)

	// Assert: the offer is to stop task 12 first, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+", then track and start "+untrackedIssue+"?")
	requireTaskWrites(t, repo)

	// Act: go ahead
	tracking := typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, and the line that tracks the issue opens,
	// prefilled in Taskwarrior's grammar
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
	requireScreen(t, tracking.View().Content, trackAndStart+untrackedIssue, "task add …", untrackedLine)

	// Act: send it
	sent := typing(t, tracking, keyEnter)

	// Assert: the task is added, annotated with the issue's page, then started,
	// and the notice says so
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID,
		"task add "+untrackedLine, "task annotate "+addedTaskUUID+" "+untrackedPage, "task start "+addedTaskUUID)
	requireScreen(t, sent.View().Content, "added and started task "+addedTaskUUID[:8])
}

func TestSwitchingTasksOffersToStopOneAndStartTheOther(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	switching := loopMoments()["switching branches"]
	switching.arrange(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-388's branch while task 12, on PROJ-412, is started
	offered := typing(t, model, switching.keys...)

	// Assert: the offer names both tasks, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+" and "+startThree+"?")
	requireTaskWrites(t, repo)

	// Act: switch
	typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, then task 3 started
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID, "task start "+trackedTaskUUID)
}

func TestOpeningAPullRequestOffersToAnnotateTheTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.pullFound = false
	model := repo.live(t, 200, 40)

	// Act: open the pull request
	linking := typing(t, model, "4", "n", keyEnter)

	// Assert: the issue linker has the keys, and the offer waits behind it
	requireScreen(t, linking.View().Content, "Link on "+issueKey)
	refuseScreen(t, linking.View().Content, noteLook)

	// Act: skip the link
	offered := typing(t, linking, keyEsc)

	// Assert: the offer names task 12 and the pull request, and nothing is
	// written yet
	requireScreen(t, offered.View().Content, noteLook, "Annotate task 12 with #42 and its URL?")
	requireTaskWrites(t, repo)

	// Act: annotate
	typing(t, offered, keyEnter)

	// Assert: task 12 is annotated with the pull request and its URL
	requireTaskWrites(t, repo, "task annotate "+activeTaskUUID+" #42 "+pullURL)
}

func TestMergingOffersToCompleteTheTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	readyToMerge(repo)
	model := repo.live(t, 200, 40)

	// Act: merge the pull request for PROJ-412
	offered := typing(t, model, "4", "M", keyEnter)

	// Assert: the offer names task 12, and nothing is written yet
	requireScreen(t, offered.View().Content, completeLook, "Mark task 12 '"+issueKey+": "+issueSummary+"' done?")
	requireTaskWrites(t, repo)

	// Act: complete it
	typing(t, offered, keyEnter)

	// Assert: task 12 is completed
	requireTaskWrites(t, repo, "task done "+activeTaskUUID)
}

func TestMovingTheIssueToDoneOffersToCompleteTheTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.moves = workflowMoves()
	model := repo.live(t, 200, 40)

	// Act: move PROJ-412 to Done
	offered := typing(t, model, "t", downAction, keyEnter)

	// Assert: the offer names task 12, and nothing is written yet
	requireScreen(t, offered.View().Content, completeLook, "Mark task 12 '"+issueKey+": "+issueSummary+"' done?")
	requireTaskWrites(t, repo)

	// Act: complete it
	typing(t, offered, keyEnter)

	// Assert: task 12 is completed
	requireTaskWrites(t, repo, "task done "+activeTaskUUID)
}

func TestMovingTheIssueElsewhereOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.moves = workflowMoves()
	model := repo.live(t, 200, 40)

	// Act
	view := typing(t, model, "t", keyEnter).View().Content

	// Assert
	// PROJ-412 moved to In Review, which is not done.
	requireScreen(t, view, issueKey+" is now In Review")
	refuseScreen(t, view, offerTitles()...)
}

func TestNothingIsOfferedWhereNoTaskWouldChange(t *testing.T) {
	t.Parallel()

	cases := map[string]moment{
		// Tracking a branch for no issue would add a task for nothing.
		"a branch for no issue": {
			arrange: func(repo *world) { repo.issues = nil },
			keys:    append(append([]string{"2", "b"}, letters("chore/tidy")...), keyEnter), reached: "create chore/tidy",
		},
		// PROJ-412's task, 12, is the one started.
		"a branch for the started task's issue": {
			arrange: func(*world) {}, keys: []string{"b", keyEnter}, reached: "create ",
		},
		"a switch to the started task's issue": {
			arrange: func(repo *world) { switchableTo(repo, "feat/"+issueKey+"-follow-up") },
			keys:    []string{"2", "s", keyEnter}, reached: "checkout feat/" + issueKey + "-follow-up",
		},
		// Task 45 tracks PROJ-388 beside task 3, and is the one started, though the
		// Tasks pane lists only task 3: the issue's own work is running already.
		"a branch for an issue another of whose tasks is started": {
			arrange: startASecondTaskOnTheSecondIssue, keys: append(branchTheSecondIssue(), keyEsc), reached: "create ",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			tt.arrange(repo)
			model := repo.live(t, 200, 40)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			if len(repo.asked(tt.reached)) == 0 {
				t.Fatalf("%q was never asked for: the moment was not reached", tt.reached)
			}

			refuseScreen(t, view, offerTitles()...)
			requireTaskWrites(t, repo)
		})
	}
}

// startASecondTaskOnTheSecondIssue stops task 12 and starts task 45, which
// tracks PROJ-388 beside task 3 and which the Tasks pane does not list.
func startASecondTaskOnTheSecondIssue(repo *world) {
	stopTheStartedTask(repo)
	repo.tasks.linked = append(repo.tasks.linked, taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000045", ID: 45, Description: secondIssue + ": Review the retries",
		Status: taskwarrior.Pending, Start: testNow().Add(-time.Hour), IssueKey: secondIssue,
	})
}

func TestNoOfferIsMadeWithoutTaskwarrior(t *testing.T) {
	t.Parallel()

	for name, tt := range loopMoments() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.none = true
			tt.arrange(repo)
			model := repo.live(t, 200, 40)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			if len(repo.asked(tt.reached)) == 0 {
				t.Fatalf("%q was never asked for: the moment was not reached", tt.reached)
			}

			refuseScreen(t, view, offerTitles()...)
			requireTaskWrites(t, repo)
		})
	}
}

func TestNoOfferIsMadeBeforeTaskwarriorHasAnswered(t *testing.T) {
	t.Parallel()

	for name, tt := range loopMoments() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The linked read failed, so which issues are tracked is not known:
			// offering to track PROJ-388 would add a second task for it.
			repo := withTasks()
			repo.tasks.linkedErr = fmt.Errorf("%w: database is locked", taskwarrior.ErrRefused)
			tt.arrange(repo)
			model := repo.live(t, 200, 40)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			if len(repo.asked(tt.reached)) == 0 {
				t.Fatalf("%q was never asked for: the moment was not reached", tt.reached)
			}

			refuseScreen(t, view, offerTitles()...)
			requireTaskWrites(t, repo)
		})
	}
}

func TestOffersAreHeldBackUnderDryRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange func(repo *world)
		// issue is the keys that select the issue whose branch is made.
		issue []string
		look  string
		want  string
	}{
		"with another issue's task started": {
			arrange: func(*world) {}, issue: []string{downAction},
			look: switchLook, want: "dry run: would stop task 12 and start task 3",
		},
		"with no task started": {
			arrange: stopTheStartedTask, issue: []string{downAction}, look: startLook, want: "dry run: would start task 3",
		},
		"with two other tasks started": {
			arrange: startTheLooseTask, issue: []string{downAction},
			look: switchLook, want: "dry run: would stop task 12 and task 9 and start task 3",
		},
		"with two other tasks started, for an untracked issue": {
			arrange: startTheLooseTask, issue: selectTheUntrackedIssue(),
			look: switchLook, want: "dry run: would stop task 12 and task 9",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Every moment of the loop is held back under dry run before it can
			// offer anything, so dry run is turned on as the branch is created: what
			// is under test is the offer's own guard. With no base to fetch, enter's
			// command creates the branch.
			repo := withAnUntrackedIssue()
			repo.moves = startTransitions()
			repo.branch.Base = ""
			tt.arrange(repo)
			model := typing(t, repo.live(t, 200, 40), append(tt.issue, "b")...)
			creating, create := pressed(t, model, keyEnter)
			offered := typing(t, drain(t, creating.WithDryRun(), create), keyEsc)

			// Act
			view := typing(t, offered, keyEnter).View().Content

			// Assert
			// Going ahead says what it would do, closes the look, and writes nothing.
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.look)
			requireTaskWrites(t, repo)
		})
	}
}
