// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// onlySpaces is a line of nothing but spaces, which is as empty as no line.
const onlySpaces = "   "

// nothingChanged is how Taskwarrior declining to change anything is worded.
const nothingChanged = "Taskwarrior changed nothing: the task is already in that state"

// syncWrite is the write a sync asks Taskwarrior for.
const syncWrite = "task sync"

// taskWrites is every write the world's Taskwarrior was asked for, in order. The
// reads, "tasks" and "tasks linked", are not among them.
func taskWrites(repo *world) []string {
	return repo.asked("task ")
}

// requireTaskWrites fails the test unless Taskwarrior was asked for exactly want,
// in order.
func requireTaskWrites(t *testing.T, repo *world, want ...string) {
	t.Helper()

	if got := taskWrites(repo); !slices.Equal(got, want) {
		t.Errorf("Taskwarrior was asked %q, want %q", got, want)
	}
}

// elsewhereIssue is an issue a task names that the Issues pane does not list.
const elsewhereIssue = "PROJ-777"

// withATaskForAnUnlistedIssue is withTasks with task 20 linked to an issue the
// Issues pane does not list. It is the least urgent, so it is drawn last.
func withATaskForAnUnlistedIssue() *world {
	repo := withTasks()
	unlisted := taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000020", ID: 20, Description: elsewhereIssue + ": Audit the logs",
		Status: taskwarrior.Pending, Urgency: 1, IssueKey: elsewhereIssue,
	}
	repo.tasks.pending = append(repo.tasks.pending, unlisted)
	repo.tasks.linked = append(repo.tasks.linked, unlisted)

	return repo
}

func TestStartStopTogglesOnTheSelectedTask(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys  []string
		write string
		want  string
	}{
		"a started task stops": {
			keys: []string{tasksPane, "s"}, write: "task stop " + activeTaskUUID, want: "● stopped 12",
		},
		"a task not started starts": {
			keys: []string{tasksPane, downAction, "s"}, write: "task start " + trackedTaskUUID, want: "● started 3",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()

			// Act
			view := typing(t, repo.live(t, 120, 40), tt.keys...).View().Content

			// Assert
			requireTaskWrites(t, repo, tt.write)
			requireScreen(t, view, tt.want)
		})
	}
}

func TestDoneCompletesTheSelectedTaskAndReloads(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane, downAction, "d").View().Content

	// Assert
	requireTaskWrites(t, repo, "task done "+trackedTaskUUID)
	requireScreen(t, view, "● marked 3 done")

	// Both lists are read at start, and again once the task is done.
	if got := repo.asked("tasks"); len(got) != 4 {
		t.Errorf("Taskwarrior's lists were read %d times, want each twice: at start and after the write", len(got))
	}
}

func TestAddOpensALineInTaskwarriorsGrammarAndSendsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act: open the line
	opened := typing(t, model, "a")

	// Assert: it asks for the words after task add, and nothing is sent yet
	requireScreen(t, opened.View().Content, "Add a task", "task add …")
	requireTaskWrites(t, repo)

	// Act: type a line and send it
	view := typing(t, opened, append(letters("due:fri Renew the cert"), keyEnter)...).View().Content

	// Assert: Taskwarrior took the line as typed, and the line closed
	requireTaskWrites(t, repo, "task add due:fri Renew the cert")
	requireScreen(t, view, "● added task 5f1c2b3a")
	refuseScreen(t, view, "Add a task")
}

func TestAnEmptyAddLineIsRefusedInPlace(t *testing.T) {
	t.Parallel()

	cases := map[string]string{"nothing typed": "", "only spaces": onlySpaces}

	for name, typed := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			model := typing(t, repo.live(t, 120, 40), append([]string{tasksPane, "a"}, letters(typed)...)...)

			// Act
			view := typing(t, model, keyEnter).View().Content

			// Assert
			requireScreen(t, view, "Add a task", "needs a value")
			requireTaskWrites(t, repo)
		})
	}
}

func TestARefusedAddStaysOpenWithTaskwarriorsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taskwarrior 3.5.0's own words for a date it cannot read.
	words := "'xyz' is not a valid date in the 'Y-M-D' format."
	repo := withTasks()
	repo.tasks.writeErr = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, words)
	model := typing(t, repo.live(t, 160, 40), tasksPane, "a")

	// Act
	view := typing(t, model, append(letters("due:xyz Renew"), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "Add a task", words)
}

func TestAnnotateAndModifyAddressTheSelectedTask(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		verb, typed, write, want string
	}{
		"annotate": {
			verb: "A", typed: "waiting on QA", write: "task annotate " + trackedTaskUUID + " waiting on QA",
			want: "● annotated 3",
		},
		"modify": {verb: "e", typed: "due:mon", write: "task modify " + trackedTaskUUID + " due:mon", want: "● modified 3"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			model := typing(t, repo.live(t, 120, 40), tasksPane, downAction, tt.verb)

			// Act
			view := typing(t, model, append(letters(tt.typed), keyEnter)...).View().Content

			// Assert
			requireTaskWrites(t, repo, tt.write)
			requireScreen(t, view, tt.want)
		})
	}
}

func TestTheAnnotateAndModifyLinesNameTheTask(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		verb string
		want []string
	}{
		"annotate": {verb: "A", want: []string{"Annotate 3", "task 3 annotate …"}},
		"modify":   {verb: "e", want: []string{"Modify 3", "task 3 modify …"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := typing(t, withTasks().live(t, 120, 40), tasksPane, downAction)

			// Act
			view := typing(t, model, tt.verb).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}

func TestUndoAndSyncRunAtOnceAndSayWhatTaskwarriorSaid(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		key, said, write, want, refuse string
	}{
		"an undo says how much it reverted": {
			key: "u", said: "reverted 1 operation", write: "task undo", want: "● undone: reverted 1 operation",
		},
		// A sync that succeeds prints nothing, so nothing follows the verb.
		"a quiet sync says only that it synced": {key: "S", write: syncWrite, want: "● synced", refuse: "synced:"},
		"a sync that says something says it": {
			key: "S", said: "Sync done", write: syncWrite, want: "● synced: Sync done",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.install.SyncConfigured = true
			repo.tasks.undoSaid, repo.tasks.syncSaid = tt.said, tt.said

			// Act
			view := typing(t, repo.live(t, 120, 40), tasksPane, tt.key).View().Content

			// Assert
			requireTaskWrites(t, repo, tt.write)
			requireScreen(t, view, tt.want)

			if tt.refuse != "" {
				refuseScreen(t, view, tt.refuse)
			}
		})
	}
}

func TestSyncIsOfferedOnlyWithASyncBackend(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured bool
		writes     []string
	}{
		"with a sync backend":    {configured: true, writes: []string{syncWrite}},
		"without a sync backend": {configured: false, writes: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.install.SyncConfigured = tt.configured
			model := typing(t, repo.live(t, 200, 40), tasksPane)

			// Act
			footer := footerLine(typing(t, model, "S").View().Content)

			// Assert
			requireTaskWrites(t, repo, tt.writes...)

			if offered := strings.Contains(footer, "S sync"); offered != tt.configured {
				t.Errorf("the footer offers a sync: %v, want %v:\n%s", offered, tt.configured, footer)
			}
		})
	}
}

func TestNothingChangedIsWordedNotRaw(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		want   string
		refuse []string
	}{
		// Another terminal started task 3 since the pane was read.
		"a start": {keys: []string{tasksPane, downAction, "s"}, want: "✗ " + nothingChanged},
		// No task was written, so the sentence for one would mislead, and
		// nothing failed.
		"an undo with none to undo": {
			keys: []string{tasksPane, "u"}, want: "Taskwarrior has nothing to undo.",
			refuse: []string{"✗", "changed nothing"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			repo.tasks.writeErr = taskwarrior.ErrNothingChanged

			// Act
			view := typing(t, repo.live(t, 160, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

func TestAFailedWriteReadsTheTasksAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.writeErr = fmt.Errorf("%w: database is locked", taskwarrior.ErrRefused)

	// Act
	typing(t, repo.live(t, 120, 40), tasksPane, "s")

	// Assert
	// A refusal can mean the pane is behind Taskwarrior, so both lists are read
	// again, as after a write that succeeds.
	if got := repo.asked("tasks"); len(got) != 4 {
		t.Errorf("Taskwarrior's lists were read %d times, want each twice: at start and after the refusal", len(got))
	}
}

func TestEnterOnALinkedTaskGoesToItsIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange []string
		want    []string
		refuse  []string
		// reads is how often the issue task 12 is for was read in full.
		reads int
	}{
		// PROJ-388 is selected on the Issues pane; task 12 is for PROJ-412, which
		// is read again once selected.
		"a linked task": {
			arrange: []string{downAction, tasksPane},
			want:    []string{focused(issuesPane), "▸ ◐◐ " + issueKey}, refuse: []string{"▸ ○○ " + secondIssue},
			reads: 2,
		},
		// Task 9 is for no issue.
		"a task for no issue": {
			arrange: []string{tasksPane, downAction, downAction},
			want:    []string{focused(tasksTitle), "▸ ○   9 Renew the cert"}, refuse: []string{focused(issuesPane)},
			reads: 1,
		},
		// Task 20 is for an issue the Issues pane does not list, so there is no
		// row to go to.
		"a task for an issue not listed": {
			arrange: []string{tasksPane, downAction, downAction, downAction},
			want:    []string{focused(tasksTitle), "▸ ○  20 " + elsewhereIssue}, refuse: []string{focused(issuesPane)},
			reads: 1,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withATaskForAnUnlistedIssue()
			model := typing(t, repo.live(t, 120, 40), tt.arrange...)

			// Act
			view := typing(t, model, keyEnter).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refuse...)

			if got := repo.asked("issue " + issueKey); len(got) != tt.reads {
				t.Errorf("%s was read %d times, want %d", issueKey, len(got), tt.reads)
			}
		})
	}
}

func TestTheTasksFooterOffersTheVerbsForTheSelectedTask(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		want   []string
		refuse []string
	}{
		"a started task for an issue": {
			keys: []string{tasksPane},
			want: []string{
				"s stop", offersDone, "a add", "A annotate", "e modify", "u undo", "enter go to issue", "o open",
				"y copy url", "r refresh",
			},
			refuse: []string{"s start", "sync"},
		},
		"a task for no issue, not started": {
			keys: []string{tasksPane, downAction, downAction}, want: []string{"s start", "d mark done", "r refresh"},
			refuse: []string{"go to issue", "open", "copy url"},
		},
		// Its page still opens, though no row on the Issues pane is there to go to.
		"a task for an issue not listed": {
			keys: []string{tasksPane, downAction, downAction, downAction}, want: []string{"o open", "y copy url"},
			refuse: []string{"go to issue"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			footer := footerLine(typing(t, withATaskForAnUnlistedIssue().live(t, 200, 40), tt.keys...).View().Content)

			// Assert
			requireScreen(t, footer, tt.want...)
			refuseScreen(t, footer, tt.refuse...)
		})
	}
}

func TestLinksOnTheTasksPaneAreTheIssuesPage(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 3 is for PROJ-388 while the Issues pane has PROJ-412 selected, and its
	// jiraurl names some other page: the tracker's page for the issue wins.
	repo := withTasks()
	changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.IssueURL = "https://old.example.com/388" })
	model := typing(t, repo.live(t, 120, 40), tasksPane, downAction)

	// Act
	typing(t, model, "o", "y")

	// Assert
	page := "https://jira.example.com/browse/" + secondIssue
	if got := slices.Concat(repo.asked("browse"), repo.asked("copy")); !slices.Equal(got,
		[]string{"browse " + page, "copy " + page}) {
		t.Errorf("the links acted on were %q, want task 3's issue's page opened and copied", got)
	}
}

func TestDryRunHoldsBackTaskActionsAndSaysSo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"stopping":   {keys: []string{tasksPane, "s"}, want: "dry run: would stop task 12"},
		"starting":   {keys: []string{tasksPane, downAction, "s"}, want: "dry run: would start task 3"},
		"completing": {keys: []string{tasksPane, "d"}, want: "dry run: would mark task 12 done"},
		"adding": {
			keys: append([]string{tasksPane, "a"}, append(letters("Renew the cert"), keyEnter)...),
			want: "dry run: task add Renew the cert",
		},
		"annotating": {
			keys: append([]string{tasksPane, "A"}, append(letters("see QA"), keyEnter)...),
			want: "dry run: task 12 annotate see QA",
		},
		"modifying": {
			keys: append([]string{tasksPane, downAction, "e"}, append(letters("due:mon"), keyEnter)...),
			want: "dry run: task 3 modify due:mon",
		},
		"undoing": {keys: []string{tasksPane, "u"}, want: "dry run: would undo Taskwarrior's last change"},
		"syncing": {keys: []string{tasksPane, "S"}, want: "dry run: would sync Taskwarrior"},
		// The line is sent, then the task is annotated with the issue's page.
		"tracking an issue": {
			keys: append(selectTheUntrackedIssue(), "T", keyEnter),
			want: "dry run: task add " + untrackedLine + " then annotate it with " + untrackedPage,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withAnUntrackedIssue()
			repo.issues[2].Priority = untrackedPriority
			repo.tasks.install.SyncConfigured = true
			model := sized(t, dryInterface(repo), 240, 40)

			// Act
			view := typing(t, drain(t, model, model.Init()), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
			requireTaskWrites(t, repo)
		})
	}
}

func TestATaskWithNoIdIsNamedByItsUUID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want []string
	}{
		"a dry-run start":   {keys: []string{"s"}, want: []string{"dry run: would start task 5f1c2b3a"}},
		"the annotate line": {keys: []string{"A"}, want: []string{"Annotate 5f1c2b3a", "task 5f1c2b3a annotate …"}},
		"the modify line":   {keys: []string{"e"}, want: []string{"Modify 5f1c2b3a", "task 5f1c2b3a modify …"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.ID = 0 })
			model := sized(t, dryInterface(repo), 160, 40)
			model = typing(t, drain(t, model, model.Init()), tasksPane, downAction)

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, "task 0")
		})
	}
}
