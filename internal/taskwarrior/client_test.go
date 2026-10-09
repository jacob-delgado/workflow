// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// baseReadOverrides are the overrides every read must lead with: nothing
// printed but the answer, no hook, garbage collection or recurrence, one JSON
// array. _show runs with these alone, since it echoes an override back as
// though the taskrc set it.
func baseReadOverrides() []string {
	return []string{
		"rc.verbose=nothing", "rc.hooks=off", "rc.gc=off", "rc.recurrence=off",
		"rc.json.array=on", "rc.color=off", "rc.detection=off", "rc.confirmation=off",
	}
}

// readOverrides are baseReadOverrides and the link UDAs defined.
func readOverrides() []string {
	return append(
		baseReadOverrides(),
		"rc.uda.jiraid.type=string", "rc.uda.jiraid.label=Jira",
		"rc.uda.jiraurl.type=string", "rc.uda.jiraurl.label=Jira URL",
	)
}

// reading is one read the fake task program saw: the read overrides, then
// words.
func reading(words ...string) call {
	return call{program: taskProgram, args: append(readOverrides(), words...)}
}

// showing is the _show Pending reads the active context from: the base read
// overrides alone, since _show echoes an override back as a setting.
func showing() call {
	return call{program: taskProgram, args: append(baseReadOverrides(), showWord)}
}

// The client's reads, by name, and the runs they make that can fail.
const (
	pendingRead = "pending"
	linkedRead  = "linked"
	touchedRead = "touched"

	pendingContextRun = "pending's context"
	pendingExportRun  = "pending's export"
	linkedExportRun   = "linked's export"
	touchedContextRun = "touched's context"
)

// The filter Pending adds to the export — status:pending alone leaves out a
// task that waits — and what _show prints for the work context active with its
// read filter.
const (
	pendingFilter = "( status:pending or status:waiting )"
	workContext   = "context=work\ncontext.work.read=project:Work\n"
)

// nonJoiner is U+200C, which Persian writes inside a word: a format character,
// so a line made safe to show loses it, and a filter must not.
const nonJoiner = string(rune(0x200C))

// reads are the client's reads, each reduced to its error.
func reads() map[string]func(context.Context, taskwarrior.Client) error {
	return map[string]func(context.Context, taskwarrior.Client) error{
		touchedRead: func(ctx context.Context, client taskwarrior.Client) error {
			_, err := client.Touched(ctx, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))

			return err
		},
		pendingRead: func(ctx context.Context, client taskwarrior.Client) error {
			_, err := client.Pending(ctx)

			return err
		},
		linkedRead: func(ctx context.Context, client taskwarrior.Client) error {
			_, err := client.Linked(ctx)

			return err
		},
	}
}

func TestPendingAsksForPendingTasksWithReadOverrides(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeTask{replies: map[string]reply{exportWord: {stdout: noTasks}, showWord: {stdout: ""}}}

	// Act
	_, err := fake.client().Pending(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Pending returned %v", err)
	}

	checkRuns(t, fake.calls, []call{showing(), reading(pendingFilter, exportWord)})
}

func TestPendingOrdersByUrgencyThenIdThenUUID(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeTask{replies: map[string]reply{
		exportWord: {stdout: `[
			{"id": 3, "uuid": "a", "urgency": 8.1},
			{"id": 0, "uuid": "e", "urgency": 2.5},
			{"id": 12, "uuid": "b", "urgency": 14.2},
			{"id": 0, "uuid": "d", "urgency": 2.5},
			{"id": 1, "uuid": "c", "urgency": 8.1}
		]`},
		showWord: {stdout: ""},
	}}

	// Act
	list, err := fake.client().Pending(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Pending returned %v", err)
	}

	ids, uuids := make([]int, 0, len(list.Tasks)), make([]string, 0, len(list.Tasks))
	for _, task := range list.Tasks {
		ids, uuids = append(ids, task.ID), append(uuids, task.UUID)
	}

	if want := []int{12, 1, 3, 0, 0}; !slices.Equal(ids, want) {
		t.Errorf("Pending ordered ids %v, want %v", ids, want)
	}

	if want := []string{"b", "c", "a", "d", "e"}; !slices.Equal(uuids, want) {
		t.Errorf("Pending ordered uuids %q, want %q", uuids, want)
	}
}

func TestPendingCarriesTheActiveContext(t *testing.T) {
	t.Parallel()

	const work = "work"

	everything := reading(pendingFilter, exportWord)
	atWork := reading("( project:Work )", pendingFilter, exportWord)

	tests := []struct {
		name   string
		shown  string // what _show prints
		want   string
		export call
	}{
		{
			name:  "a context and its read filter",
			shown: workContext + "context.work.write=project:Work\nsync.encryption_secret=" + taskrcSecret + "\n",
			want:  work, export: atWork,
		},
		{
			name:  "a read filter over the legacy definition",
			shown: "context=work\ncontext.work=project:Home\ncontext.work.read=project:Work\n", want: work, export: atWork,
		},
		{
			name:  "an empty read filter over the legacy definition",
			shown: "context=work\ncontext.work=project:Home\ncontext.work.read=\n", want: work, export: everything,
		},
		{
			name:  "a context defined the legacy way",
			shown: "context=work\ncontext.work=project:Work\n", want: work, export: atWork,
		},
		{name: "a context that filters nothing", shown: "context=work\n", want: work, export: everything},
		{name: "no context", shown: "color=on\ncontext.work.read=project:Work\n", export: everything},
		{name: "a context set to nothing", shown: "context=\ncontext.work.read=project:Work\n", export: everything},
		{
			name:  "a name with a space",
			shown: "context=my work\ncontext.my work.read=project:Work\n", want: "my work", export: atWork,
		},
		{
			name:  "a name that would read as an override",
			shown: "context=a=b\ncontext.a=b.read=project:Work\n", want: "a=b", export: everything,
		},
		{
			name:  "only the active context's filter",
			shown: "context=work\ncontext.home.read=project:Home\ncontext.workshop.read=project:Shop\n",
			want:  work, export: everything,
		},
		{
			name:   "a name carrying controls, shown safe",
			shown:  "context=work\x1b[31m\ncontext.work\x1b[31m.read=project:Work\n",
			want:   work,
			export: atWork,
		},
		{
			name:   "a filter holding a joiner, sent as printed",
			shown:  "context=fa\ncontext.fa.read=project:mi" + nonJoiner + "xah\n",
			want:   "fa",
			export: reading("( project:mi"+nonJoiner+"xah )", pendingFilter, exportWord),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{showWord: {stdout: test.shown}, exportWord: {stdout: noTasks}}}

			// Act
			list, err := fake.client().Pending(t.Context())

			// Assert
			if err != nil || list.Context != test.want {
				t.Errorf("Pending = %+v, %v; want context %q", list, err, test.want)
			}

			checkRuns(t, fake.calls, []call{showing(), test.export})

			for _, run := range fake.calls {
				if slices.ContainsFunc(run.args, func(arg string) bool { return strings.Contains(arg, taskrcSecret) }) {
					t.Errorf("Pending ran task %q, repeating the sync secret", run.args)
				}
			}

			if seen := fmt.Sprintf("%+v %v", list, err); strings.Contains(seen, taskrcSecret) {
				t.Errorf("Pending kept the sync secret: %s", seen)
			}
		})
	}
}

func TestLinkedBypassesTheContextAndKeepsCompletedTasks(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(exportWord, `[{"uuid": "u", "status": "completed", "jiraid": "PROJ-42"}]`)

	// Act
	tasks, err := fake.client().Linked(t.Context())

	// Assert
	if err != nil || len(tasks) != 1 || tasks[0].Status != taskwarrior.Completed || !tasks[0].Linked() {
		t.Errorf("Linked = %+v, %v; want the completed task, linked", tasks, err)
	}

	want := append(readOverrides(), "rc.context=", "jiraid.any:", "status.not:deleted", exportWord)
	if len(fake.calls) != 1 || !slices.Equal(fake.calls[0].args, want) {
		t.Errorf("Linked ran %+v, want task %q", fake.calls, want)
	}
}

func TestAnExportOfNothingIsAnEmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(exportWord, noTasks)

	// Act
	tasks, err := fake.client().Linked(t.Context())

	// Assert
	if err != nil || len(tasks) != 0 {
		t.Errorf("Linked = %+v, %v; want no tasks and no error", tasks, err)
	}

	if len(fake.calls) != 1 {
		t.Errorf("Linked ran %d commands, want the export", len(fake.calls))
	}
}

func TestReadsMapATaskwarriorNeverRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		read    string
		failing string
		stderr  string
		want    error
	}{
		{
			name: pendingExportRun, read: pendingRead, failing: exportWord,
			stderr: noRCFile, want: taskwarrior.ErrNotConfigured,
		},
		{
			name: linkedExportRun, read: linkedRead, failing: exportWord,
			stderr: noRCFile, want: taskwarrior.ErrNotConfigured,
		},
		{
			name: pendingContextRun, read: pendingRead, failing: showWord,
			stderr: noRCFile, want: taskwarrior.ErrNotConfigured,
		},
		{
			name: "an export refused for want of a report", read: linkedRead, failing: exportWord,
			stderr: "Unable to find report that matches 'status:pending'.", want: taskwarrior.ErrRefused,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{showWord: {stdout: workContext}, exportWord: {stdout: noTasks}}}
			fake.replies[test.failing] = reply{err: exited(t, 2, test.stderr)}

			// Act
			err := reads()[test.read](t.Context(), fake.client())

			// Assert
			if !errors.Is(err, test.want) {
				t.Errorf("%s returned %v, want %v", test.read, err, test.want)
			}

			if errors.Is(err, taskwarrior.ErrNotConfigured) != errors.Is(test.want, taskwarrior.ErrNotConfigured) {
				t.Errorf("%s returned %v, mistaking whether Taskwarrior was ever run", test.read, err)
			}

			var never taskwarrior.NeverRunError
			if errors.As(err, &never) != errors.Is(test.want, taskwarrior.ErrNotConfigured) ||
				(errors.As(err, &never) && never.Program != taskProgram) {
				t.Errorf("%s returned %v, want a never-run Taskwarrior named %q", test.read, err, taskProgram)
			}
		})
	}
}

func TestAReadThatExitsOneIsARefusalInItsWords(t *testing.T) {
	t.Parallel()

	const words = "The filter could not be read."

	tests := []struct {
		name    string
		read    string
		failing string
	}{
		{name: pendingContextRun, read: pendingRead, failing: showWord},
		{name: pendingExportRun, read: pendingRead, failing: exportWord},
		{name: linkedExportRun, read: linkedRead, failing: exportWord},
		{name: touchedContextRun, read: touchedRead, failing: showWord},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{showWord: {stdout: workContext}, exportWord: {stdout: noTasks}}}
			fake.replies[test.failing] = reply{err: exited(t, 1, words)}

			// Act
			err := reads()[test.read](t.Context(), fake.client())

			// Assert
			if !errors.Is(err, taskwarrior.ErrRefused) || errors.Is(err, taskwarrior.ErrNothingChanged) {
				t.Fatalf("%s returned %v, want ErrRefused and not ErrNothingChanged", test.read, err)
			}

			if !strings.Contains(err.Error(), words) {
				t.Errorf("%s returned %q, want Taskwarrior's words %q", test.read, err, words)
			}
		})
	}
}

func TestPendingKeepsNoSecretFromAFailedShow(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeTask{replies: map[string]reply{
		showWord: {
			stdout: workContext + "sync.encryption_secret=" + taskrcSecret + "\n",
			err:    exited(t, 2, "Configuration error."),
		},
		exportWord: {stdout: noTasks},
	}}

	// Act
	list, err := fake.client().Pending(t.Context())

	// Assert
	if !errors.Is(err, taskwarrior.ErrRefused) {
		t.Errorf("Pending returned %v, want ErrRefused", err)
	}

	if seen := fmt.Sprintf("%+v %v", list, err); strings.Contains(seen, taskrcSecret) {
		t.Errorf("Pending kept the sync secret: %s", seen)
	}
}

func TestReadsRefuseOutputThatIsNotJSON(t *testing.T) {
	t.Parallel()

	const report = "Unable to find report that has the name 'status:pending'.\n"

	tests := []struct {
		name    string
		read    string
		printed string
	}{
		{name: "a report error to pending", read: pendingRead, printed: report},
		{name: "a report error to linked", read: linkedRead, printed: report},
		{name: "a date it does not write", read: linkedRead, printed: `[{"uuid": "u", "due": "tomorrow"}]`},
		{
			name:    "an annotation dated as it does not write",
			read:    linkedRead,
			printed: `[{"uuid": "u", "annotations": [{"entry": "yesterday", "description": "see #42"}]}]`,
		},
		{name: "depends neither a list nor a string", read: linkedRead, printed: `[{"uuid": "u", "depends": 42}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := answering(exportWord, test.printed)
			fake.replies[showWord] = reply{}

			// Act
			err := reads()[test.read](t.Context(), fake.client())

			// Assert
			if !errors.Is(err, taskwarrior.ErrBadOutput) {
				t.Errorf("%s returned %v, want ErrBadOutput", test.read, err)
			}
		})
	}
}

func TestReadsPassThroughAStoppedOrMissingProgram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sentinel    error
		failing     string
		canceled    bool   // before Pending runs
		cancelAfter string // the command word whose answer cancels the context
		runs, live  int    // at most runs, and exactly live of them on a live context
	}{
		{name: "an export stopped at its bound", sentinel: proc.ErrTimedOut, failing: exportWord, runs: 2, live: 2},
		{name: "an export of a missing program", sentinel: proc.ErrNotFound, failing: exportWord, runs: 2, live: 2},
		{name: "the context asked of a stopped program", sentinel: proc.ErrTimedOut, failing: showWord, runs: 1, live: 1},
		{name: "a read whose context is canceled", sentinel: context.Canceled, canceled: true, runs: 1},
		{
			name: "a cancel once the context is read reaches the export", sentinel: context.Canceled,
			cancelAfter: showWord, runs: 2, live: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{exportWord: {stdout: noTasks}, showWord: {stdout: workContext}}}
			if test.failing != "" {
				fake.replies[test.failing] = reply{err: fmt.Errorf("task: %w", test.sentinel)}
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if test.canceled {
				cancel()
			}

			client := taskwarrior.New(fake.cancelingAfter(test.cancelAfter, cancel), taskwarrior.Install{Program: taskProgram})

			// Act
			_, err := client.Pending(ctx)

			// Assert
			if !errors.Is(err, test.sentinel) {
				t.Errorf("Pending returned %v, want %v", err, test.sentinel)
			}

			if len(fake.calls) > test.runs || liveRuns(fake.calls) != test.live {
				t.Errorf("Pending ran %+v, want at most %d runs, %d of them on a live context", fake.calls, test.runs, test.live)
			}
		})
	}
}
