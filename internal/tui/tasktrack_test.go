// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// untrackedPage is the untracked issue's page in the tracker.
const untrackedPage = "https://jira.example.com/browse/" + untrackedIssue

// untrackedLine is the line that tracks the untracked issue, its priority High.
const untrackedLine = "jiraid:" + untrackedIssue + " jiraurl:" + untrackedPage + " +jira priority:H -- " +
	untrackedIssue + ": Rotate the keys"

// untrackedPriority is the untracked issue's priority where a test gives it one,
// which untrackedLine carries as Taskwarrior's H.
const untrackedPriority = "High"

// trackHelp is what the track key's help says it does on an untracked issue.
const trackHelp = "track in Taskwarrior"

// untrackedTaskUUID is the uuid of task 44, which a test links to the untracked
// issue.
const untrackedTaskUUID = "5f1c2b3a-7d4e-4f60-8a9b-000000000044"

// taskForTheUntrackedIssue is task 44, pending and linked to the untracked
// issue, for a test to change and place among the world's tasks.
func taskForTheUntrackedIssue() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: untrackedTaskUUID, ID: 44, Description: untrackedIssue + ": Rotate the keys", Status: taskwarrior.Pending,
		Urgency: 3, IssueKey: untrackedIssue, IssueURL: untrackedPage,
	}
}

// selectTheUntrackedIssue is the keys that move the Issues pane's selection onto
// the untracked issue, the third listed.
func selectTheUntrackedIssue() []string {
	return []string{downAction, downAction}
}

func TestTrackOnAnUntrackedIssueOpensThePrefilledLine(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

	// Act: press T on the untracked issue
	opened := typing(t, model, "T")

	// Assert: the line is prefilled in Taskwarrior's grammar, and nothing is sent
	requireScreen(t, opened.View().Content, "Track "+untrackedIssue, "task add …", untrackedLine)
	requireTaskWrites(t, repo)

	// Act: send it
	typing(t, opened, keyEnter)

	// Assert: the task is added, then annotated with the issue's page
	requireTaskWrites(t, repo, "task add "+untrackedLine, "task annotate "+addedTaskUUID+" "+untrackedPage)
}

func TestTrackOnAnIssueWhoseKeyIsNotOneWordOpensNoLine(t *testing.T) {
	t.Parallel()

	// Arrange
	// The key stands before the line's --, so a word after it would reach
	// Taskwarrior as an rc override of its own.
	repo := withAnUntrackedIssue()
	repo.issues[2].Key = untrackedIssue + " rc.hooks=on"
	model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

	// Act
	view := typing(t, model, "T").View().Content

	// Assert
	requireScreen(t, view, "not one word")
	refuseScreen(t, view, "task add …")
	requireTaskWrites(t, repo)
}

func TestTrackOnATrackedIssueGoesToItsTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), downAction)

	// Act
	view := typing(t, model, "T").View().Content

	// Assert
	requireScreen(t, view, focused(tasksTitle), "▸ ○   3 "+secondIssue)
	requireTaskWrites(t, repo)
}

func TestTrackScrollsTheTasksPaneToTheTrackingTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()

	// Forty tasks for PROJ-412, more urgent than task 3, list it far below what
	// 24 rows show.
	for index := range 40 {
		filler := taskwarrior.Task{
			UUID: fmt.Sprintf("5f1c2b3a-7d4e-4f60-8a9b-0000000001%02d", index), ID: 100 + index,
			Description: fmt.Sprintf("Filler %d", index), Status: taskwarrior.Pending, Urgency: 50, IssueKey: issueKey,
		}
		repo.tasks.pending = append(repo.tasks.pending, filler)
		repo.tasks.linked = append(repo.tasks.linked, filler)
	}

	model := typing(t, repo.live(t, 80, 24), downAction)

	// Act
	view := typing(t, model, "T").View().Content

	// Assert
	requireScreen(t, view, "▸ ○   3 ")
}

func TestTrackOnAnIssueWhoseTaskIsNotListedSaysWhy(t *testing.T) {
	t.Parallel()

	// Two hours east of UTC, 22:00 UTC is midnight of the next day.
	east := time.FixedZone("CEST", int((2 * time.Hour).Seconds()))

	cases := map[string]struct {
		arrange func(repo *world, task taskwarrior.Task)
		want    string
		refuse  string
	}{
		"it waits": {
			arrange: func(repo *world, task taskwarrior.Task) {
				task.Wait = time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)
				repo.tasks.pending = append(repo.tasks.pending, task)
				repo.tasks.linked = append(repo.tasks.linked, task)
			},
			want: untrackedIssue + " is tracked by task 44, which waits until 2026-10-02", refuse: "2026-10-01",
		},
		// Pending, yet not among the pending tasks read: the context hides it.
		"it is outside the active context": {
			arrange: func(repo *world, task taskwarrior.Task) {
				repo.tasks.context = "home"
				repo.tasks.linked = append(repo.tasks.linked, task)
			},
			want: untrackedIssue + " is tracked by task 44, outside context home",
		},
		// Pending, and no context hides it, yet not among the pending tasks
		// read: the two lists are read one after the other, and it changed
		// between them.
		"it was not among the tasks read": {
			arrange: func(repo *world, task taskwarrior.Task) {
				repo.tasks.linked = append(repo.tasks.linked, task)
			},
			want: untrackedIssue + " is tracked by task 44, not among the tasks just read; " +
				"r in the Tasks pane reads them again",
			refuse: "outside context",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			tt.arrange(repo, taskForTheUntrackedIssue())

			deps := repo.deps()
			deps.Clock = func() time.Time { return testNow().In(east) }
			model := sized(t, tui.New(repo.cfg, nil, deps), 200, 40)
			model = typing(t, drain(t, model, model.Init()), selectTheUntrackedIssue()...)

			// Act
			view := typing(t, model, "T").View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, focused(tasksTitle), "Track "+untrackedIssue)

			if tt.refuse != "" {
				refuseScreen(t, view, tt.refuse)
			}
		})
	}
}

func TestATaskStillToDoTracksItsIssueWhateverItsStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status taskwarrior.Status
		want   string
	}{
		// A recurring task's id means nothing, so it goes by its uuid.
		"recurring": {
			status: taskwarrior.Recurring, want: untrackedIssue + " is tracked by task 5f1c2b3a, a recurring template",
		},
		"waiting": {
			status: taskwarrior.Waiting, want: untrackedIssue + " is tracked by task 44, which waits until 2026-09-19",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			task := taskForTheUntrackedIssue()
			task.Status, task.Wait = tt.status, testNow().Add(72*time.Hour)
			repo.tasks.linked = append(repo.tasks.linked, task)
			model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

			// Act
			view := typing(t, model, "T").View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, "Track "+untrackedIssue)
		})
	}
}

func TestTrackGoesToTheListedTaskOfTheIssuesTasks(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 44 waits, and so is not listed; task 45, for the same issue, is.
	repo := withAnUntrackedIssue()
	waiting := taskForTheUntrackedIssue()
	waiting.Wait = testNow().Add(72 * time.Hour)
	listed := taskForTheUntrackedIssue()
	listed.UUID, listed.ID, listed.Description = "5f1c2b3a-7d4e-4f60-8a9b-000000000045", 45, "Rotate the staging keys"
	repo.tasks.pending = append(repo.tasks.pending, waiting, listed)
	repo.tasks.linked = append(repo.tasks.linked, waiting, listed)
	model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

	// Act
	view := typing(t, model, "T").View().Content

	// Assert
	requireScreen(t, view, focused(tasksTitle), "▸ ○  45 Rotate the staging keys")
	refuseScreen(t, view, "is tracked by task")
}

func TestTrackOnAnIssueWhoseTasksAreAllCompletedOpensTheLine(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	done := taskForTheUntrackedIssue()
	done.Status, done.ID = taskwarrior.Completed, 0
	repo.tasks.linked = append(repo.tasks.linked, done)
	model := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)

	// Act
	view := typing(t, model, "T").View().Content

	// Assert
	requireScreen(t, view, "Track "+untrackedIssue, "task add …")
}

func TestTheTrackKeyReadsGoToTaskOnATrackedIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		want   string
		refuse string
	}{
		"a tracked issue":   {keys: []string{downAction}, want: "T go to task", refuse: trackHelp},
		"an untracked one":  {keys: selectTheUntrackedIssue(), want: "T " + trackHelp, refuse: "go to task"},
		"the started issue": {want: "T go to task", refuse: trackHelp},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := withAnUntrackedIssue().live(t, 240, 40)

			// Act
			footer := footerLine(typing(t, model, tt.keys...).View().Content)

			// Assert
			requireScreen(t, footer, tt.want)
			refuseScreen(t, footer, tt.refuse)
		})
	}
}

func TestTrackWaitsForTaskwarriorToAnswer(t *testing.T) {
	t.Parallel()

	cases := map[string]func(tasks *taskWorld){
		"the linked read failed": func(tasks *taskWorld) {
			tasks.linkedErr = fmt.Errorf("%w: database is locked", taskwarrior.ErrRefused)
		},
		"go-task is on PATH": func(tasks *taskWorld) { tasks.installErr = taskwarrior.ErrNotTaskwarrior },
	}

	for name, unanswered := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			unanswered(repo.tasks)
			model := typing(t, repo.live(t, 240, 40), selectTheUntrackedIssue()...)

			// Act
			view := typing(t, model, "T", keyEnter).View().Content

			// Assert
			requireTaskWrites(t, repo)
			refuseScreen(t, view, "Track "+untrackedIssue)
			refuseScreen(t, footerLine(view), trackHelp, "go to task")
		})
	}
}

// startedWithoutTaskwarriorsAnswer runs Init's loads one at a time and finishes
// each but Taskwarrior's read, whose answer is dropped: the Issues pane is
// loaded, and Taskwarrior has not answered yet. The read is found by the calls
// it records, not by its place in the batch.
func startedWithoutTaskwarriorsAnswer(t *testing.T, repo *world, model tui.Model) tui.Model {
	t.Helper()

	loads, ok := model.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("Init did not start its loads as a batch")
	}

	for _, load := range loads {
		if load == nil {
			continue
		}

		readsBefore := len(repo.asked("tasks"))
		msg := load()

		if len(repo.asked("tasks")) == readsBefore {
			model = drain(t, model, func() tea.Msg { return msg })
		}
	}

	if len(repo.asked("tasks")) == 0 {
		t.Fatal("Init never read Taskwarrior's tasks")
	}

	return model
}

func TestTrackWaitsForTaskwarriorsFirstAnswer(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		selection, pressed, refuse []string
	}{
		// Had T opened a line, enter would send it.
		"an untracked issue": {
			selection: selectTheUntrackedIssue(), pressed: []string{"T", keyEnter},
			refuse: []string{"Track " + untrackedIssue},
		},
		// Task 3 tracks it, though Taskwarrior has not said so yet.
		"a tracked issue": {
			selection: []string{downAction}, pressed: []string{"T"},
			refuse: []string{focused(tasksTitle), "Track " + secondIssue},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			model := startedWithoutTaskwarriorsAnswer(t, repo, sized(t, tui.New(repo.cfg, nil, repo.deps()), 240, 40))
			model = typing(t, model, tt.selection...)

			// Act
			view := typing(t, model, tt.pressed...).View().Content

			// Assert
			requireTaskWrites(t, repo)
			refuseScreen(t, view, tt.refuse...)
			refuseScreen(t, footerLine(view), trackHelp, "go to task")
		})
	}
}

func TestWithoutTaskwarriorTheTaskKeysDoNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	view := typing(t, model, "T", tasksPane, "a", "x", keyEnter, "u").View().Content

	// Assert
	requireScreen(t, view, focused(tasksTitle), "Taskwarrior is not installed")
	refuseScreen(t, view, "Track ", "Add a task")
}

func TestTheTrackLineKeepsItsCursorInView(t *testing.T) {
	t.Parallel()

	// At 80 columns the prefilled line runs past the overlay's edge, so its end,
	// where the cursor is, shows only if the line scrolls.
	cases := map[string]struct {
		typed []string
		want  string
	}{
		"as it opens":    {want: untrackedIssue + ": Rotate the keys"},
		"as it is typed": {typed: letters(" ZZZ"), want: "ZZZ"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := typing(t, withAnUntrackedIssue().live(t, 80, 24), selectTheUntrackedIssue()...)

			// Act
			view := typing(t, model, append([]string{"T"}, tt.typed...)...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestTheTrackLineScrollsToWhatIsTypedAfterTheTerminalNarrows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Opened at 200 columns the whole line fits; at 80 it no longer does.
	opened := typing(t, withAnUntrackedIssue().live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	narrowed := sized(t, opened, 80, 24)

	// Act
	view := typing(t, narrowed, letters(" ZZZ")...).View().Content

	// Assert
	requireScreen(t, view, "ZZZ")
}

func TestTheTrackLineShowsItsEndAndCursorOnceTheTerminalNarrows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Opened at 200 columns the whole line fits; at 80 it no longer does.
	opened := typing(t, withAnUntrackedIssue().live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	end := untrackedIssue + ": Rotate the keys"

	// Act
	view := sized(t, opened, 80, 24).View().Content

	// Assert
	// The cursor is reverse video, which v2 may combine with other attributes,
	// so it is matched by the reverse SGR's start.
	withCursor := slices.ContainsFunc(strings.Split(view, "\n"), func(row string) bool {
		return strings.Contains(plain(row), end) && strings.Contains(row, "\x1b[7")
	})
	if !withCursor {
		t.Errorf("no row shows the line's end, %q, and the cursor:\n%s", end, plain(view))
	}
}

func TestTrackingAnIssueWithoutAPageAddsNoAnnotation(t *testing.T) {
	t.Parallel()

	// Arrange
	// A tracker with no page for an issue, as the forge's issues have none here.
	repo := withAnUntrackedIssue()
	deps := repo.deps()
	deps.Jira.BrowseURL = nil
	model := sized(t, tui.New(repo.cfg, nil, deps), 200, 40)

	// Act
	typing(t, drain(t, model, model.Init()), append(selectTheUntrackedIssue(), "T", keyEnter)...)

	// Assert
	requireTaskWrites(t, repo, "task add jiraid:"+untrackedIssue+" jiraurl: +jira -- "+untrackedIssue+": Rotate the keys")
}

func TestATrackedTaskThatCannotBeAnnotatedIsKeptAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	repo.tasks.annotateErr = fmt.Errorf("%w: annotation refused", taskwarrior.ErrRefused)
	model := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)

	// Act
	view := typing(t, model, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "✗ The task was created but the issue's link could not be added as an annotation")
	refuseScreen(t, view, "Track "+untrackedIssue)
}

func TestTrackIsOfferedOnlyWhereATaskCanBeAdded(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		withoutAdd bool
		want       []string
		refuse     []string
	}{
		"with Taskwarrior": {want: []string{"T tracks it in Taskwarrior.", "T " + trackHelp}},
		"without a way to add a task": {
			withoutAdd: true, refuse: []string{"T tracks it in Taskwarrior.", trackHelp},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			deps := repo.deps()

			if tt.withoutAdd {
				deps.Tasks.Add = nil
			}

			model := sized(t, tui.New(repo.cfg, nil, deps), 200, 40)

			// Act
			view := typing(t, drain(t, model, model.Init()), selectTheUntrackedIssue()...).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

func TestTheTrackHintNamesTheTrackKey(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	repo.cfg.UI.Keys = map[string]string{"track-issue": "X"}

	// Act
	view := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...).View().Content

	// Assert
	requireScreen(t, view, "X tracks it in Taskwarrior.")
	refuseScreen(t, view, "T tracks it")
}
