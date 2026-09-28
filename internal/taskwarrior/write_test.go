// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskUUID names the task every write in these tests addresses.
const taskUUID = "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"

// The write command words the fake task program answers.
const (
	addWord  = "add"
	undoWord = "undo"
	syncWord = "sync"
)

// undone is what Taskwarrior 3.5.0 prints when undo reverts an add: how many
// operations it reverted, then each of them.
const undone = `The following 6 operations would be reverted:
Uuid                                 Modification
44af4dc3-c43e-42a2-8628-1b68f40dc776 Create task
                                     Add property 'modified' with value
                                     '1790586839'
                                     Add property 'description' with value
                                     'Renew the cert'
                                     Add property 'entry' with value
                                     '1790586839'
                                     Update property 'modified' from
                                     '1790586839' to '1790586839'
                                     Add property 'status' with value 'pending'

`

// writeOverrides are the overrides every write must lead with: nothing to
// confirm, one JSON array, no color, and the link UDAs defined. Hooks are left
// as the taskrc has them, so a timewarrior hook still fires.
func writeOverrides() []string {
	return []string{
		"rc.verbose=nothing", "rc.confirmation=off", "rc.recurrence.confirmation=no",
		"rc.dependency.confirmation=off", "rc.bulk=0", "rc.color=off", "rc.detection=off", "rc.json.array=on",
		"rc.uda.jiraid.type=string", "rc.uda.jiraid.label=Jira",
		"rc.uda.jiraurl.type=string", "rc.uda.jiraurl.label=Jira URL",
	}
}

// changing is the arguments of a write to the task taskUUID names: the write
// overrides, the context bypassed so its read filter cannot hide the task, the
// uuid, then words.
func changing(words ...string) []string {
	return append(writeOverrides(), append([]string{"rc.context=", "uuid:" + taskUUID}, words...)...)
}

// writes are the client's writes to one task, each reduced to its error.
func writes() map[string]func(context.Context, taskwarrior.Client) error {
	return map[string]func(context.Context, taskwarrior.Client) error{
		"start": func(ctx context.Context, client taskwarrior.Client) error { return client.Start(ctx, taskUUID) },
		"stop":  func(ctx context.Context, client taskwarrior.Client) error { return client.Stop(ctx, taskUUID) },
		"done":  func(ctx context.Context, client taskwarrior.Client) error { return client.Done(ctx, taskUUID) },
		"annotate": func(ctx context.Context, client taskwarrior.Client) error {
			return client.Annotate(ctx, taskUUID, "see #42")
		},
		"modify": func(ctx context.Context, client taskwarrior.Client) error {
			return client.Modify(ctx, taskUUID, "due:fri")
		},
	}
}

func TestAddSendsTheLineAsWordsAfterAdd(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(addWord, "Created task "+taskUUID+".\n")

	// Act
	_, err := fake.client().Add(t.Context(), "due:fri +urgent Renew the cert")

	// Assert
	if err != nil || len(fake.calls) != 1 {
		t.Fatalf("Add ran %+v and returned %v, want one run", fake.calls, err)
	}

	args := fake.calls[0].args

	want := append(writeOverrides(), "rc.verbose=new-uuid", addWord, "due:fri", "+urgent", "Renew", "the", "cert")
	if !slices.Equal(args, want) {
		t.Errorf("Add ran task %q, want %q", args, want)
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "rc.hooks") {
			t.Errorf("Add sent %s, want hooks left as the taskrc has them", arg)
		}

		if strings.HasPrefix(arg, "rc.context") {
			t.Errorf("Add sent %s, want the context's write attributes applied", arg)
		}
	}
}

func TestAddReturnsTheUUIDTaskwarriorPrints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		printed string
	}{
		{name: "a task", printed: "Created task " + taskUUID + ".\n"},
		{name: "a recurring task", printed: "Created task " + taskUUID + " (recurrence template).\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := answering(addWord, test.printed)

			// Act
			uuid, err := fake.client().Add(t.Context(), "Renew the cert")

			// Assert
			if err != nil || uuid != taskUUID {
				t.Errorf("Add = %q, %v; want %q", uuid, err, taskUUID)
			}
		})
	}
}

func TestAddRefusesABlankLine(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(addWord, "Created task "+taskUUID+".\n")

	// Act
	_, err := fake.client().Add(t.Context(), "  ")

	// Assert
	if !errors.Is(err, taskwarrior.ErrRefused) {
		t.Errorf("Add returned %v, want ErrRefused", err)
	}

	if len(fake.calls) != 0 {
		t.Errorf("Add ran %+v, want nothing run", fake.calls)
	}
}

func TestAddPassesOnTaskwarriorsOwnRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	const reason = "'xyz' is not a valid date in the 'Y-M-D' format."

	fake := &fakeTask{replies: map[string]reply{addWord: {err: exited(t, 2, reason)}}}

	// Act
	_, err := fake.client().Add(t.Context(), "due:xyz Renew the cert")

	// Assert
	if !errors.Is(err, taskwarrior.ErrRefused) {
		t.Fatalf("Add returned %v, want ErrRefused", err)
	}

	if want := taskwarrior.ErrRefused.Error() + ": " + reason; err.Error() != want {
		t.Errorf("Add's error = %q, want %q", err, want)
	}
}

func TestAddWithoutTheCreatedLineIsBadOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(addWord, "")

	// Act
	_, err := fake.client().Add(t.Context(), "Renew the cert")

	// Assert
	if !errors.Is(err, taskwarrior.ErrBadOutput) {
		t.Errorf("Add returned %v, want ErrBadOutput", err)
	}
}

func TestStartStopDoneAddressTheTaskByUUID(t *testing.T) {
	t.Parallel()

	for _, verb := range []string{"start", "stop", "done"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := answering(verb, "")

			// Act
			err := writes()[verb](t.Context(), fake.client())

			// Assert
			if err != nil || len(fake.calls) != 1 {
				t.Fatalf("%s ran %+v and returned %v, want one run", verb, fake.calls, err)
			}

			if want := changing(verb); !slices.Equal(fake.calls[0].args, want) {
				t.Errorf("%s ran task %q, want %q", verb, fake.calls[0].args, want)
			}
		})
	}
}

func TestAWriteTaskwarriorDeclinesIsNothingChanged(t *testing.T) {
	t.Parallel()

	for verb, write := range writes() {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{verb: {err: exited(t, 1, "")}}}

			// Act
			err := write(t.Context(), fake.client())

			// Assert
			if !errors.Is(err, taskwarrior.ErrNothingChanged) {
				t.Errorf("%s returned %v, want ErrNothingChanged", verb, err)
			}
		})
	}
}

func TestAnnotatePutsTheTextAfterTheDoubleDash(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering("annotate", "")

	// Act
	err := fake.client().Annotate(t.Context(), taskUUID, "see #42 https://jira.example.com/browse/PROJ-42")

	// Assert
	if err != nil || len(fake.calls) != 1 {
		t.Fatalf("Annotate ran %+v and returned %v, want one run", fake.calls, err)
	}

	want := changing("annotate", "--", "see", "#42", "https://jira.example.com/browse/PROJ-42")
	if !slices.Equal(fake.calls[0].args, want) {
		t.Errorf("Annotate ran task %q, want %q", fake.calls[0].args, want)
	}
}

func TestModifySendsTheLineAsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering("modify", "")

	// Act
	err := fake.client().Modify(t.Context(), taskUUID, "due:fri +urgent")

	// Assert
	if err != nil || len(fake.calls) != 1 {
		t.Fatalf("Modify ran %+v and returned %v, want one run", fake.calls, err)
	}

	want := changing("modify", "due:fri", "+urgent")
	if !slices.Equal(fake.calls[0].args, want) {
		t.Errorf("Modify ran task %q, want %q", fake.calls[0].args, want)
	}
}

func TestUndoAndSyncReturnWhatTaskwarriorSaid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		canSync    bool
		say        func(taskwarrior.Client, context.Context) (string, error)
		word       string
		printed    string
		refusal    string
		want       string
		wantErr    error
		wantInvoke []string
	}{
		{
			name:       undoWord,
			say:        taskwarrior.Client.Undo,
			word:       undoWord,
			printed:    undone,
			want:       "reverted 6 operations",
			wantInvoke: append(writeOverrides(), undoWord),
		},
		{
			name:       "an undo of one operation",
			say:        taskwarrior.Client.Undo,
			word:       undoWord,
			printed:    "The following 1 operations would be reverted:\n",
			want:       "reverted 1 operation",
			wantInvoke: append(writeOverrides(), undoWord),
		},
		{
			name:       "an undo that prints no count",
			say:        taskwarrior.Client.Undo,
			word:       undoWord,
			printed:    undone[strings.Index(undone, "\n")+1:],
			want:       "",
			wantInvoke: append(writeOverrides(), undoWord),
		},
		{
			name:       "nothing to undo",
			say:        taskwarrior.Client.Undo,
			word:       undoWord,
			printed:    "No operations to undo.\nCould not undo: other operations have occurred.",
			wantErr:    taskwarrior.ErrNothingChanged,
			wantInvoke: append(writeOverrides(), undoWord),
		},
		{
			name:       syncWord,
			canSync:    true,
			say:        taskwarrior.Client.Sync,
			word:       syncWord,
			printed:    "",
			want:       "",
			wantInvoke: append(writeOverrides(), syncWord),
		},
		{
			name:       "a sync that says what it did",
			canSync:    true,
			say:        taskwarrior.Client.Sync,
			word:       syncWord,
			printed:    "\x1b[32mSynced 3 changes\x1b[0m\n",
			want:       "Synced 3 changes",
			wantInvoke: append(writeOverrides(), syncWord),
		},
		{
			name:       "a refused undo",
			say:        taskwarrior.Client.Undo,
			word:       undoWord,
			refusal:    "No undo information available.",
			wantErr:    taskwarrior.ErrRefused,
			wantInvoke: append(writeOverrides(), undoWord),
		},
		{
			name:       "a refused sync",
			canSync:    true,
			say:        taskwarrior.Client.Sync,
			word:       syncWord,
			refusal:    "Could not connect to the sync server.",
			wantErr:    taskwarrior.ErrRefused,
			wantInvoke: append(writeOverrides(), syncWord),
		},
		{
			name:    "sync with no backend",
			say:     taskwarrior.Client.Sync,
			word:    syncWord,
			printed: "Synced 3 changes\n",
			wantErr: taskwarrior.ErrNoSync,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := answering(test.word, test.printed)
			if test.refusal != "" {
				fake.replies[test.word] = reply{err: exited(t, 2, test.refusal)}
			}

			client := taskwarrior.New(fake.run, taskwarrior.Install{Program: taskProgram, SyncConfigured: test.canSync})

			// Act
			said, err := test.say(client, t.Context())

			// Assert
			if said != test.want || !errors.Is(err, test.wantErr) {
				t.Errorf("%s = %q, %v; want %q, %v", test.name, said, err, test.want, test.wantErr)
			}

			var invoked []string
			if len(fake.calls) == 1 {
				invoked = fake.calls[0].args
			}

			if len(fake.calls) > 1 || !slices.Equal(invoked, test.wantInvoke) {
				t.Errorf("%s ran %+v, want task %q", test.name, fake.calls, test.wantInvoke)
			}
		})
	}
}

func TestSyncGetsTheLongBound(t *testing.T) {
	t.Parallel()

	detect := func(ctx context.Context, fake *fakeTask) error {
		_, err := taskwarrior.Detect(ctx, taskProgram, nil, fake.run)

		return err
	}

	tests := []struct {
		name string
		act  func(context.Context, *fakeTask) error
		word string // the command word of the run whose bound is checked
		want time.Duration
	}{
		{
			name: syncWord,
			act: func(ctx context.Context, fake *fakeTask) error {
				_, err := fake.client().Sync(ctx)

				return err
			},
			word: syncWord,
			want: 2 * time.Minute,
		},
		{
			name: addWord,
			act: func(ctx context.Context, fake *fakeTask) error {
				_, err := fake.client().Add(ctx, "Renew the cert")

				return err
			},
			word: addWord,
			want: 30 * time.Second,
		},
		{
			name: exportWord,
			act: func(ctx context.Context, fake *fakeTask) error {
				_, err := fake.client().Linked(ctx)

				return err
			},
			word: exportWord,
			want: 10 * time.Second,
		},
		{
			name: "pending's " + showWord,
			act: func(ctx context.Context, fake *fakeTask) error {
				_, err := fake.client().Pending(ctx)

				return err
			},
			word: showWord,
			want: 10 * time.Second,
		},
		{name: "detect's " + versionWord, act: detect, word: versionWord, want: 10 * time.Second},
		{name: "detect's " + showWord, act: detect, word: showWord, want: 10 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{
				syncWord: {}, addWord: {stdout: "Created task " + taskUUID + ".\n"}, exportWord: {stdout: noTasks},
				versionWord: {stdout: versionAnswer}, showWord: {},
			}}

			// Act
			err := test.act(t.Context(), fake)

			// Assert
			bound := slices.IndexFunc(fake.calls, func(run call) bool { return slices.Contains(run.args, test.word) })
			if err != nil || bound < 0 {
				t.Fatalf("%s ran %+v and returned %v, want a run of %s", test.name, fake.calls, err, test.word)
			}

			if got := fake.calls[bound].timeout; got != test.want {
				t.Errorf("%s was bound by %s, want %s", test.name, got, test.want)
			}
		})
	}
}
