// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// listedTasks is the task list the server answers over the fake, read at
// taskDay's noon.
func listedTasks(t *testing.T, fake *taskFake) api.TaskList {
	t.Helper()

	deps := tasksDeps(fake)
	deps.Clock = func() time.Time { return taskDay().Add(3 * time.Hour) }

	return decode[api.TaskList](t, get(t, serve(t, deps, config.Default()), tasksPath))
}

// taskFacetLines is each facet as kind|value|label, in order.
func taskFacetLines(facets []api.TaskFacet) []string {
	lines := make([]string, 0, len(facets))
	for _, facet := range facets {
		lines = append(lines, string(facet.Kind)+"|"+facet.Value+"|"+facet.Label)
	}

	return lines
}

func TestListTasksSaysWhereEachTaskStandsAsTheListWordsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The plain task is pending, but hidden until after the list is read.
	fake := fakeTaskwarrior()
	waits := plainTask()
	waits.Wait = taskDay().Add(24 * time.Hour)
	fake.pending.Tasks = []taskwarrior.Task{startedTask(), waits}

	// Act
	list := listedTasks(t, fake)

	// Assert
	states := make([]api.TaskState, 0, len(list.Tasks))
	for _, task := range list.Tasks {
		states = append(states, task.State)
	}

	if want := []api.TaskState{api.TaskStateStarted, api.TaskStateWaiting}; !slices.Equal(states, want) {
		t.Errorf("states = %v, want %v", states, want)
	}
}

func TestListTasksDescribesEachTasksFacets(t *testing.T) {
	t.Parallel()

	// Act
	list := listedTasks(t, fakeTaskwarrior())

	// Assert
	want := [][]string{
		{
			"state|started|started", "priority|H|priority H", "project|workflow|project workflow",
			"issue|linked|with issue", "tag|jira|+jira",
		},
		{
			"state|pending|pending", "priority||no priority", "project||no project",
			"issue|unlinked|no issue", "tag||no tag",
		},
	}

	if len(list.Tasks) != len(want) {
		t.Fatalf("tasks = %+v, want %d", list.Tasks, len(want))
	}

	for index, task := range list.Tasks {
		if got := taskFacetLines(task.Facets); !slices.Equal(got, want[index]) {
			t.Errorf("task %s facets = %q, want %q", task.UUID, got, want[index])
		}
	}
}

func TestListTasksRanksEachTaskInEveryOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	// The started task is the more urgent, task 3, tagged, for an issue, at
	// priority H; the plain one is task 7, untagged, unlinked, with none.
	fake := fakeTaskwarrior()
	fake.pending.Tasks = []taskwarrior.Task{plainTask(), startedTask()}

	// Act
	list := listedTasks(t, fake)

	// Assert
	want := []api.TaskRanks{
		{Urgency: 1, State: 1, ID: 1, Tag: 1, Issue: 1, Priority: 1},
		{Urgency: 0, State: 0, ID: 0, Tag: 0, Issue: 0, Priority: 0},
	}

	got := make([]api.TaskRanks, 0, len(list.Tasks))
	for _, task := range list.Tasks {
		got = append(got, task.Ranks)
	}

	if !slices.Equal(got, want) {
		t.Errorf("ranks = %+v, want %+v", got, want)
	}
}

func TestListTasksShipsTheFieldsTypedTextMatchesLowerCased(t *testing.T) {
	t.Parallel()

	// Act
	list := listedTasks(t, fakeTaskwarrior())

	// Assert
	want := []string{"proj-412: fix token redaction", "workflow", "proj-412", "+jira", "#3"}
	if got := list.Tasks[0].Searchable; !slices.Equal(got, want) {
		t.Errorf("searchable = %q, want %q", got, want)
	}
}

func TestListTasksShipsTheOrderTheFilterOffersItsValuesIn(t *testing.T) {
	t.Parallel()

	// Act
	list := listedTasks(t, fakeTaskwarrior())

	// Assert
	want := []string{
		"state|started|started", "state|pending|pending", "state|waiting|waiting", "state|recurring|recurring",
		"state|completed|completed", "state|deleted|deleted", "state|unknown|unknown",
		"priority|H|priority H", "priority|M|priority M", "priority|L|priority L", "priority||no priority",
		"project|workflow|project workflow", "project||no project",
		"tag|jira|+jira", "tag||no tag",
		"issue|linked|with issue", "issue|unlinked|no issue",
	}
	if got := taskFacetLines(list.FacetOrder); !slices.Equal(got, want) {
		t.Errorf("facet_order = %q, want %q", got, want)
	}
}

func TestListTasksOffersNothingWithNoTaskwarrior(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Tasks = seams.Tasks{}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), tasksPath)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if list.Available || list.FacetOrder == nil || len(list.FacetOrder) != 0 {
		t.Errorf("list = %+v, want unavailable with an empty facet_order", list)
	}
}

// ranksOf is the ranks of the task with uuid among tasks, failing the test
// when none has it.
func ranksOf(t *testing.T, tasks []api.Task, uuid string) api.TaskRanks {
	t.Helper()

	at := slices.IndexFunc(tasks, func(task api.Task) bool { return task.UUID == uuid })
	if at < 0 {
		t.Fatalf("tasks = %+v, want one with uuid %s", tasks, uuid)
	}

	return tasks[at].Ranks
}

func TestTheSnapshotRanksTheActiveTaskAmongTheListThatHoldsIt(t *testing.T) {
	t.Parallel()

	// urgent is more urgent than the started task and numbered before it, so
	// among your pending tasks the started one ranks second by both. Among the
	// linked ones, PROJ-7's done task comes before it by issue.
	urgent := plainTask()
	urgent.ID, urgent.Urgency = 1, 20

	cases := map[string]struct {
		pending []taskwarrior.Task
		// heldBy is the ranks the list holding the started task gives it.
		heldBy func(t *testing.T, list api.TaskList, summary api.TasksSummary) api.TaskRanks
	}{
		"your pending tasks": {
			pending: []taskwarrior.Task{urgent, startedTask()},
			heldBy: func(t *testing.T, list api.TaskList, _ api.TasksSummary) api.TaskRanks {
				t.Helper()

				return ranksOf(t, list.Tasks, startedUUID)
			},
		},
		"the linked tasks, when the context hides it": {
			pending: []taskwarrior.Task{urgent},
			heldBy: func(t *testing.T, _ api.TaskList, summary api.TasksSummary) api.TaskRanks {
				t.Helper()

				return ranksOf(t, summary.Linked, startedUUID)
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.pending.Tasks = tt.pending
			deps := tasksDeps(fake)
			deps.Clock = func() time.Time { return taskDay().Add(3 * time.Hour) }
			handler := serve(t, deps, config.Default())
			list := decode[api.TaskList](t, get(t, handler, tasksPath))

			// Act
			summary := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String()).Tasks

			// Assert
			want := tt.heldBy(t, list, summary)
			if summary.Active == nil || summary.Active.Ranks != want {
				t.Errorf("active = %+v, want ranks %+v, as the list holding it ranks it", summary.Active, want)
			}
		})
	}
}

func TestListTasksReadsTheClockOnceForTheTasksAndTheirFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	// The plain task, in the garden project, waits until a minute after the
	// clock's first read: a list read at two moments would word it waiting,
	// yet offer its project, which the filter offers only for a task not
	// waiting.
	first := taskDay().Add(3 * time.Hour)
	waits := plainTask()
	waits.Project, waits.Wait = "garden", first.Add(time.Minute)
	fake := fakeTaskwarrior()
	fake.pending.Tasks = []taskwarrior.Task{waits}
	deps := tasksDeps(fake)

	var reads atomic.Int32

	deps.Clock = func() time.Time {
		if reads.Add(1) == 1 {
			return first
		}

		return first.Add(time.Hour)
	}

	// Act
	list := decode[api.TaskList](t, get(t, serve(t, deps, config.Default()), tasksPath))

	// Assert
	if len(list.Tasks) != 1 || list.Tasks[0].State != api.TaskStateWaiting {
		t.Fatalf("tasks = %+v, want the one, waiting at the clock's first read", list.Tasks)
	}

	if offered := taskFacetLines(list.FacetOrder); slices.Contains(offered, "project|garden|project garden") {
		t.Errorf("the filter offers %q, the project of a task worded waiting", offered)
	}
}
