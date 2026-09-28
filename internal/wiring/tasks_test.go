// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// fakeTaskwarrior is a task program that answers as Taskwarrior version would,
// recording each call's arguments on a line of the file $RECORD names. It
// dispatches on its last argument, which is the command.
func fakeTaskwarrior(version string) string {
	return `#!/bin/sh
printf '%s\n' "$*" >> "$RECORD"
for last in "$@"; do :; done
case "$last" in
_version) echo ` + version + ` ;;
_show) echo data.location=/tmp ;;
export) echo '[{"uuid":"a1b2c3d4-0000-4000-8000-000000000001","id":1,"description":"x","status":"pending",` +
		`"entry":"20260101T000000Z","modified":"20260101T000000Z","urgency":1}]' ;;
esac
`
}

// goTask is the Taskfile runner, which is also called task: it has no
// _version task, and prints a bare version for --version.
const goTask = `#!/bin/sh
for last in "$@"; do :; done
case "$last" in
--version) echo 3.51.1 ;;
*) echo 'task: Task "_version" does not exist' >&2; exit 200 ;;
esac
`

// recording points $RECORD at a new file for a fake task program to write each
// call's arguments to, and returns its path.
func recording(t *testing.T) string {
	t.Helper()

	record := filepath.Join(t.TempDir(), "calls")
	t.Setenv("RECORD", record)

	return record
}

// wiredTasks is the Taskwarrior seams wiring.Deps builds over cfg with PATH set
// to path alone, so no real task program is ever run.
func wiredTasks(t *testing.T, cfg config.Config, path string) seams.Tasks {
	t.Helper()
	t.Setenv("PATH", path)

	return wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Tasks
}

// askedWith reports a recorded call ending with command that also carries
// override.
func askedWith(calls, command, override string) bool {
	return slices.ContainsFunc(strings.Split(calls, "\n"), func(call string) bool {
		return strings.HasSuffix(call, command) && strings.Contains(call, override)
	})
}

// setTaskSeams names every Taskwarrior seam that is not nil.
func setTaskSeams(tasks seams.Tasks) []string {
	var set []string

	for seam, field := range reflect.ValueOf(tasks).Fields() {
		if !field.IsNil() {
			set = append(set, seam.Name)
		}
	}

	return set
}

func TestWithoutATaskProgramNoTaskSeamIsOffered(t *testing.T) {
	// Arrange
	path := pathWithGitAnd(t, "not-task", idleScript)

	// Act
	tasks := wiredTasks(t, config.Default(), path)

	// Assert
	if set := setTaskSeams(tasks); len(set) != 0 {
		t.Errorf("with no task program on PATH, the seams %v are set, want none", set)
	}
}

func TestTaskwarriorDisabledOffersNoTaskSeam(t *testing.T) {
	// Arrange
	path := pathWithGitAnd(t, "task", fakeTaskwarrior("3.5.0"))

	cfg := config.Default()
	cfg.Taskwarrior.Disabled = true

	// Act
	tasks := wiredTasks(t, cfg, path)

	// Assert
	if set := setTaskSeams(tasks); len(set) != 0 {
		t.Errorf("with taskwarrior.disabled, the seams %v are set, want none", set)
	}
}

func TestAFakeTaskwarriorAnswersThroughTheSeams(t *testing.T) {
	// Arrange
	record := recording(t)
	tasks := wiredTasks(t, config.Default(), pathWithGitAnd(t, "task", fakeTaskwarrior("3.5.0")))

	// Act
	install, installErr := tasks.Install()
	list, pendingErr := tasks.Pending()

	// Assert
	if installErr != nil || install.Version != "3.5.0" || install.DataDir != "/tmp" {
		t.Errorf("Install = %+v, %v; want 3.5.0 with its data directory", install, installErr)
	}

	if pendingErr != nil || len(list.Tasks) != 1 || list.Tasks[0].Description != "x" {
		t.Fatalf("Pending = %+v, %v; want the one task", list, pendingErr)
	}

	calls, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("reading what the task program was asked: %v", err)
	}

	if !askedWith(string(calls), "( status:pending or status:waiting ) export", "rc.hooks=off") {
		t.Errorf("the task program was asked:\n%s\nwant a read with hooks off of pending and waiting tasks", calls)
	}

	// The first call settles which program is Taskwarrior, and it is kept.
	if asked := strings.Count(string(calls), "_version"); asked != 1 {
		t.Errorf("the task program was asked its version %d times, want once:\n%s", asked, calls)
	}
}

func TestGoTaskOnPathIsToldApartFromTaskwarrior(t *testing.T) {
	// Arrange
	tasks := wiredTasks(t, config.Default(), pathWithGitAnd(t, "task", goTask))

	// Act
	_, err := tasks.Install()

	// Assert
	if !errors.Is(err, taskwarrior.ErrNotTaskwarrior) {
		t.Errorf("Install with go-task on PATH returned %v, want ErrNotTaskwarrior", err)
	}
}

func TestAConfiguredProgramWinsOverPath(t *testing.T) {
	// Arrange
	recording(t)

	configured := filepath.Join(t.TempDir(), "taskwarrior")
	write(t, configured, fakeTaskwarrior("3.6.0"), 0o700)

	cfg := config.Default()
	cfg.Taskwarrior.Program = configured

	tasks := wiredTasks(t, cfg, pathWithGitAnd(t, "task", fakeTaskwarrior("3.5.0")))

	// Act
	install, err := tasks.Install()

	// Assert
	if err != nil || install.Program != configured || install.Version != "3.6.0" {
		t.Errorf("Install = %+v, %v; want the configured program's 3.6.0", install, err)
	}
}

func TestAConfiguredProgramIsUsedWithNoTaskOnPath(t *testing.T) {
	// Arrange
	recording(t)

	configured := filepath.Join(t.TempDir(), "taskwarrior")
	write(t, configured, fakeTaskwarrior("3.6.0"), 0o700)

	cfg := config.Default()
	cfg.Taskwarrior.Program = configured

	tasks := wiredTasks(t, cfg, pathWithGitAnd(t, "not-task", idleScript))

	// Act
	install, err := tasks.Install()

	// Assert
	if err != nil || install.Program != configured {
		t.Errorf("Install = %+v, %v; want the configured program though no task is on PATH", install, err)
	}
}

func TestEveryTaskSeamSaysWhyTaskwarriorWasNotFound(t *testing.T) {
	calls := map[string]func(seams.Tasks) error{
		"Pending":  func(tasks seams.Tasks) error { return errorOf(tasks.Pending()) },
		"Linked":   func(tasks seams.Tasks) error { return errorOf(tasks.Linked()) },
		"Add":      func(tasks seams.Tasks) error { return errorOf(tasks.Add("x")) },
		"Start":    func(tasks seams.Tasks) error { return tasks.Start(taskUUID) },
		"Stop":     func(tasks seams.Tasks) error { return tasks.Stop(taskUUID) },
		"Done":     func(tasks seams.Tasks) error { return tasks.Done(taskUUID) },
		"Annotate": func(tasks seams.Tasks) error { return tasks.Annotate(taskUUID, "x") },
		"Modify":   func(tasks seams.Tasks) error { return tasks.Modify(taskUUID, "x") },
		"Undo":     func(tasks seams.Tasks) error { return errorOf(tasks.Undo()) },
		"Sync":     func(tasks seams.Tasks) error { return errorOf(tasks.Sync()) },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// Arrange
			tasks := wiredTasks(t, config.Default(), pathWithGitAnd(t, "task", goTask))

			// Act
			err := call(tasks)

			// Assert
			if !errors.Is(err, taskwarrior.ErrNotTaskwarrior) {
				t.Errorf("%s with go-task on PATH returned %v, want ErrNotTaskwarrior", name, err)
			}
		})
	}
}

// errorOf is the error of a call that also answers a value.
func errorOf[T any](_ T, err error) error {
	return err
}

// taskUUID is a task a seam is asked to change.
const taskUUID = "a1b2c3d4-0000-4000-8000-000000000001"
