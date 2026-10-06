// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// tasksPane is the key that jumps to the Tasks pane, the seventh.
const tasksPane = "7"

// tasksTitle is the Tasks pane's name, and the heading of an issue's tasks.
const tasksTitle = "Tasks"

// untrackedIssue is an issue no task is linked to.
const untrackedIssue = "PROJ-500"

// withAnUntrackedIssue is withTasks with a third issue listed that no task names.
func withAnUntrackedIssue() *world {
	repo := withTasks()
	repo.issues = append(repo.issues, jira.Issue{
		Key: untrackedIssue, Summary: "Rotate the keys", Status: "Open", StatusCategory: jira.CategoryNew,
	})

	return repo
}

// changeTask changes a task wherever the world's Taskwarrior holds it: in the
// pending list, the linked tasks, or both.
func changeTask(repo *world, uuid string, change func(*taskwarrior.Task)) {
	for index := range repo.tasks.pending {
		if repo.tasks.pending[index].UUID == uuid {
			change(&repo.tasks.pending[index])
		}
	}

	for index := range repo.tasks.linked {
		if repo.tasks.linked[index].UUID == uuid {
			change(&repo.tasks.linked[index])
		}
	}
}

// onlyTasks is the tasks among tasks with one of the uuids.
func onlyTasks(tasks []taskwarrior.Task, uuids ...string) []taskwarrior.Task {
	var kept []taskwarrior.Task

	for _, task := range tasks {
		for _, uuid := range uuids {
			if task.UUID == uuid {
				kept = append(kept, task)
			}
		}
	}

	return kept
}

// requireInOrder fails the test unless the screen shows every one of want, each
// below or after the one before it.
func requireInOrder(t *testing.T, view string, want ...string) {
	t.Helper()

	rest := plain(view)

	for _, each := range want {
		_, after, found := strings.Cut(rest, each)
		if !found {
			t.Fatalf("the screen does not show %q after %q:\n%s", each, want, plain(view))
		}

		rest = after
	}
}

func TestTheTasksPaneListsPendingTasksByUrgencyInTwoGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	// The tasks for listed issues come first, most urgent first, then the others
	// under their heading, then how many wait; a waiting task is not listed. Each
	// row's faint tail is its issue, when it is due, and its urgency.
	requireInOrder(t, view, "▸ ◐  12 "+issueKey+": "+issueSummary+"  "+issueKey+" · 14.2 ",
		"  ○   3 "+secondIssue+": Add retries  "+secondIssue+" · 8.1", "Other",
		"  ○   9 Renew the cert  due in 2d 2h · 2.0", "1 waiting")
	refuseScreen(t, view, "Water the plants")
}

func TestOneGroupHasNoHeadingAndNoWaitingCount(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kept []string
		want string
	}{
		"only tasks for listed issues": {kept: []string{activeTaskUUID, trackedTaskUUID}, want: "▸ ◐  12 " + issueKey},
		"only other tasks":             {kept: []string{looseTaskUUID}, want: "▸ ○   9 Renew the cert"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.pending = onlyTasks(repo.tasks.pending, tt.kept...)

			// Act
			view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "Other", "0 waiting")
		})
	}
}

func TestATaskForAnIssueTheIssuesPaneDoesNotListIsAnOtherTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.pending = append(repo.tasks.pending, taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000030", ID: 30, Description: "Chase the vendor",
		Status: taskwarrior.Pending, IssueKey: "PROJ-999", Urgency: 10,
	})

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireInOrder(t, view, "Other", " 30 Chase the vendor  PROJ-999 · 10.0")
}

func TestAnOverdueTaskSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, looseTaskUUID, func(task *taskwarrior.Task) { task.Due = testNow().Add(-time.Hour) })

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "  ○   9 Renew the cert  overdue · 2.0")
}

func TestWhenEveryPendingTaskWaitsTheListSaysHowMany(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.pending = onlyTasks(repo.tasks.pending, waitingTaskUUID)

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireInOrder(t, view, "No pending tasks.", "1 waiting")
}

func TestTheTasksRailCountsPendingAndActive(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		idle   bool
		want   string
		refuse string
	}{
		// With no context, the count names none.
		"one started":  {want: "3 pending · 1 active", refuse: "ctx"},
		"none started": {idle: true, want: "3 pending", refuse: "active"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			if tt.idle {
				changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Start = time.Time{} })
			}

			// Act
			view := repo.live(t, 120, 40).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse)
		})
	}
}

func TestAnActiveContextNamesThePaneAndTheRail(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		want   []string
		refuse []string
	}{
		"in the rail, and on no other pane": {
			want: []string{"7 Tasks · work", "ctx work"}, refuse: []string{"Issues · work"},
		},
		// Focused, the list is drawn in the detail, whose heavy border names it.
		"on the focused detail": {keys: []string{tasksPane}, want: []string{"━ Tasks · work "}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.context = "work"

			// Act
			view := typing(t, repo.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

func TestWithoutTaskwarriorTheTasksPaneSaysSoAndOffersNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		turnedOff    bool
		keys         []string
		want         string
		refuse       string
		footerRefuse []string
	}{
		"the rail says so": {keys: nil, want: "not installed", refuse: "turned off"},
		"the detail names the setting, and nothing is offered": {
			keys: []string{tasksPane}, want: "○ Taskwarrior is not installed", refuse: "taskwarrior.disabled",
			footerRefuse: []string{"refresh", "start", "done", "add"},
		},
		// Turned off on purpose, Taskwarrior is not missing.
		"turned off, the rail says so": {turnedOff: true, want: "turned off", refuse: "not installed"},
		"turned off, the detail names the setting": {
			turnedOff: true, keys: []string{tasksPane}, want: "Turned off by taskwarrior.disabled.",
			refuse: "Install Taskwarrior", footerRefuse: []string{"refresh"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.none = true
			repo.cfg.Taskwarrior.Disabled = tt.turnedOff

			// Act
			view := typing(t, repo.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse)
			refuseScreen(t, footerLine(view), tt.footerRefuse...)
		})
	}
}

func TestAGoTaskOnPathIsExplainedInTheTasksPane(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.installErr = taskwarrior.ErrNotTaskwarrior

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "another program (go-task, most likely)", "Set taskwarrior.program")
}

func TestAGoTaskOnPathIsExplainedInASCII(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.installErr = taskwarrior.ErrNotTaskwarrior

	// Act
	view := typing(t, asciiInterface(t, repo, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "(go-task, most likely)")

	for _, character := range view {
		if character > 0x7e {
			t.Fatalf("an ASCII screen drew %q:\n%s", character, view)
		}
	}
}

func TestATaskwarriorThatCannotBeUsedIsExplainedInTheTasksPane(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		installErr error
		want       []string
	}{
		// A malformed taskrc is refused at detection in fixed words, shown as any
		// refusal is: in Taskwarrior's own.
		"a malformed taskrc": {
			installErr: fmt.Errorf("%w: taskrc has a malformed line", taskwarrior.ErrRefused),
			want:       []string{"✗ taskwarrior refused the command: taskrc has a malformed line"},
		},
		"a Taskwarrior too old": {
			installErr: fmt.Errorf("%w: 3.4.0 at /usr/bin/task; 3.5.0 or newer is needed", taskwarrior.ErrTooOld),
			want:       []string{"Taskwarrior is too old", "3.5.0 or newer is needed; `workflow doctor`"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.installErr = tt.installErr

			// Act
			view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}

func TestTheSelectedTaskShowsItsIssueBranchAndPull(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()

	// Act
	view := typing(t, repo.live(t, 160, 40), tasksPane).View().Content

	// Assert
	// Task 12 is selected first; its issue is the one the checked-out branch
	// names, whose pull request the Review pane holds, its CI passed.
	requireScreen(t, view, issueKey+" "+issueSummary+" · "+statusInProgress+" · "+featureName+" · #42 ●",
		"started 1h12m ago · workflow · +jira · urgency 14.2 · #12", "Annotations", "2026-09-16 "+pullURL)
}

func TestEachSelectedTaskShowsOnlyItsOwnFacts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		want   string
		refuse []string
	}{
		// The checked-out branch names PROJ-412, not PROJ-388.
		"a task for another issue names no branch": {
			keys: []string{tasksPane, downAction}, want: secondIssue + " Add retries · To Do",
			refuse: []string{"To Do · ", "started"},
		},
		"a task not started shows when it is due": {
			keys: []string{tasksPane, downAction, downAction}, want: "due 2026-09-18 · urgency 2.0 · #9",
			refuse: []string{"started"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()

			// Act
			view := typing(t, repo.live(t, 160, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

func TestATasksDatesShowInTheClocksZone(t *testing.T) {
	t.Parallel()

	// Two hours east of UTC, 22:00 UTC is midnight of the next day.
	east := time.FixedZone("CEST", int((2 * time.Hour).Seconds()))
	lateInTheDay := func(day int) time.Time { return time.Date(2026, 9, day, 22, 0, 0, 0, time.UTC) }

	cases := map[string]struct {
		change func(*taskwarrior.Task)
		uuid   string
		keys   []string
		want   string
		refuse string
	}{
		"a due date": {
			uuid: looseTaskUUID, change: func(task *taskwarrior.Task) { task.Due = lateInTheDay(18) },
			keys: []string{tasksPane, downAction, downAction}, want: "due 2026-09-19 ·", refuse: "2026-09-18",
		},
		"an annotation's day": {
			uuid: activeTaskUUID, change: func(task *taskwarrior.Task) { task.Annotations[0].Entry = lateInTheDay(15) },
			keys: []string{tasksPane}, want: "2026-09-16 " + pullURL, refuse: "2026-09-15",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			changeTask(repo, tt.uuid, tt.change)

			deps := repo.deps()
			deps.Clock = func() time.Time { return testNow().In(east) }
			model := sized(t, tui.New(repo.cfg, nil, deps), 160, 40)

			// Act
			view := typing(t, drain(t, model, model.Init()), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse)
		})
	}
}

func TestADryRunTasksPaneStillListsTheTasks(t *testing.T) {
	t.Parallel()

	// Arrange
	// A dry run holds back Taskwarrior's writes; its reads still answer.
	repo := withTasks()
	model := sized(t, dryInterface(repo), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), tasksPane).View().Content

	// Assert
	requireScreen(t, view, " 12 "+issueKey+": "+issueSummary, "  3 "+secondIssue+": Add retries")
}

func TestMovingDownTheTasksSelectsTheNext(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane, downAction).View().Content

	// Assert
	requireScreen(t, view, "▸ ○   3 "+secondIssue)
}

// withChores adds thirty pending tasks linked to nothing, 40 to 69, each less
// urgent than the one before and than task 9, so they are listed last, in
// order, and run past the bottom of the screen.
func withChores(repo *world) {
	const first, count = 40, 30

	for id := first; id < first+count; id++ {
		repo.tasks.pending = append(repo.tasks.pending, taskwarrior.Task{
			UUID: fmt.Sprintf("5f1c2b3a-7d4e-4f60-8a9b-%012d", id), ID: id, Description: "Chore " + strconv.Itoa(id),
			Status: taskwarrior.Pending, Urgency: 1.9 - float64(id-first)/100,
		})
	}
}

func TestTheSelectedTaskStaysOnScreen(t *testing.T) {
	t.Parallel()

	// Twenty-five rows down from task 12, past 3, 9 and the heading, is Chore 62,
	// well below the bottom of an 80x24 detail.
	moves := slices.Repeat([]string{downAction}, 25)

	cases := map[string][]string{
		"as the selection moves": slices.Concat([]string{tasksPane}, moves),
		"after a refresh":        slices.Concat([]string{tasksPane}, moves, []string{"r"}),
	}

	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			withChores(repo)

			// Act
			view := typing(t, repo.live(t, 80, 24), keys...).View().Content

			// Assert
			requireScreen(t, view, "▸ ○  62 Chore 62")
		})
	}
}

func TestClickingATaskBelowTheHeadingSelectsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Focused, the list is drawn in the detail from its top row: 12, 3, the
	// heading, then 9 on row 5.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act
	view := click(t, model, 60, 5).View().Content

	// Assert
	requireScreen(t, view, "▸ ○   9 Renew the cert")
}

func TestClickingTheHeadingKeepsTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// The heading is on row 4, below task 3; clicking it selects neither.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act
	view := click(t, model, 60, 4).View().Content

	// Assert
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
}

func TestRefreshingKeepsTheSelectedTaskWhenTheListReorders(t *testing.T) {
	t.Parallel()

	// Arrange
	// Select task 3, then let a more urgent task come back above it, so the rows
	// shift under the cursor.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane, downAction)
	repo.tasks.pending = append(repo.tasks.pending, taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000031", ID: 31, Description: "Answer the review",
		Status: taskwarrior.Pending, IssueKey: issueKey, Urgency: 30,
	})

	// Act
	view := typing(t, model, "r").View().Content

	// Assert
	requireInOrder(t, view, " 31 Answer", " 12 "+issueKey, "▸ ○   3 "+secondIssue)

	if got := repo.asked("tasks"); len(got) != 4 {
		t.Errorf("Taskwarrior's lists were read %d times, want each twice: at start and on refresh", len(got))
	}
}

func TestTheSelectionStaysOnItsTaskWhenTheIssuesChange(t *testing.T) {
	t.Parallel()

	// Task 3 is selected second, below task 12 for the listed PROJ-412; once
	// PROJ-412 leaves the Issues pane, task 12 joins the others and 3 is first.
	cases := map[string]struct {
		arrange func(*world)
		keys    []string
	}{
		"the Issues pane refreshed": {
			arrange: func(repo *world) { repo.issues = repo.issues[1:] },
			keys:    []string{"1", "r", tasksPane},
		},
		"another view": {
			arrange: func(repo *world) {
				repo.viewIssues = map[string][]jira.Issue{scopedSprintJQL: repo.issues[1:]}
			},
			keys: []string{"1", "v", tasksPane},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.cfg = twoViewConfig()
			model := typing(t, repo.live(t, 120, 40), tasksPane, downAction)
			tt.arrange(repo)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			requireInOrder(t, view, "▸ ○   3 "+secondIssue, "Other", " 12 "+issueKey)
		})
	}
}

func TestAFailedTaskReadShowsTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.err = taskwarrior.NeverRunError{Program: "/opt/homebrew/bin/task"}

	// Act
	view := typing(t, repo.live(t, 200, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "Taskwarrior has never run",
		"run it once in a terminal so it creates its configuration;",
		"/opt/homebrew/bin/task")
	refuseScreen(t, view, "Run `task`")
}

func TestAFailedLinkedReadShowsTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.linkedErr = fmt.Errorf("%w: unexpected end of JSON input", taskwarrior.ErrBadOutput)

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "unreadable answer", "Taskwarrior answered with something other than its JSON; `task export`")
}

func TestAClickOnAFailedReadSelectsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pending list answered and the linked read failed, so the failure is
	// drawn where the list would be, task 9 on row 5 among it.
	repo := withTasks()
	repo.tasks.linkedErr = fmt.Errorf("%w: unexpected end of JSON input", taskwarrior.ErrBadOutput)
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act: click where task 9 would be
	clicked := click(t, model, 60, 5)

	// Assert: the failure is still what the pane shows
	requireScreen(t, clicked.View().Content, "unreadable answer")

	// Act: read again, and this time Taskwarrior answers
	repo.tasks.linkedErr = nil
	view := typing(t, clicked, "r").View().Content

	// Assert: the click chose no task, so the first is still selected
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
	refuseScreen(t, view, "▸ ○   9")
}

func TestAnEmptyTaskListSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.pending, repo.tasks.linked = nil, nil

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "No pending tasks.", "0 pending")
	refuseScreen(t, view, "0 waiting")
}
