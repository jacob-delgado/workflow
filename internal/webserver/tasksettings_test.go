// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// settingsChanged is the task list's reason once the taskwarrior settings in
// effect are not the ones its Taskwarrior was bound with at start.
const settingsChanged = "Taskwarrior settings changed; restart workflow to apply."

// savedSettings is a server started over start, with tasks as its Taskwarrior,
// once Settings has saved change over the taskwarrior settings.
func savedSettings(
	t *testing.T, tasks seams.Tasks, start config.Taskwarrior, change func(*config.Taskwarrior),
) http.Handler {
	t.Helper()

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	cfg.Taskwarrior = start

	deps := filledDeps()
	deps.Tasks = tasks
	handler := serve(t, deps, cfg)

	saved := cfg
	change(&saved.Taskwarrior)

	recorder := putConfig(t, handler, marshal(t, saved))
	if recorder.Code != http.StatusOK {
		t.Fatalf("saving the settings answered %d: %s", recorder.Code, recorder.Body.String())
	}

	return handler
}

func TestTheTaskListSaysSavedTaskwarriorSettingsApplyAtTheNextStart(t *testing.T) {
	t.Parallel()

	// A found go-task: the search at start answered that task on PATH was not
	// Taskwarrior.
	goTaskFound := func() seams.Tasks {
		fake := fakeTaskwarrior()
		fake.fail["install"] = fmt.Errorf("%w: tried /usr/local/bin/task", taskwarrior.ErrNotTaskwarrior)

		return fake.seams()
	}
	cases := map[string]struct {
		tasks  seams.Tasks
		start  config.Taskwarrior
		change func(*config.Taskwarrior)
	}{
		"turned back on": {
			tasks: seams.Tasks{}, start: config.Taskwarrior{Disabled: true},
			change: func(settings *config.Taskwarrior) { settings.Disabled = false },
		},
		"a program named": {
			tasks:  goTaskFound(),
			change: func(settings *config.Taskwarrior) { settings.Program = taskwarriorProgram },
		},
		"turned off": {
			tasks:  fakeTaskwarrior().seams(),
			change: func(settings *config.Taskwarrior) { settings.Disabled = true },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := savedSettings(t, tt.tasks, tt.start, tt.change)

			// Act
			recorder := get(t, handler, tasksPath)

			// Assert
			list := decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || list.Available || list.Reason != settingsChanged {
				t.Errorf("answer = %d %+v, want 200, not available, reason %q", recorder.Code, list, settingsChanged)
			}

			if list.ReasonCode == nil || *list.ReasonCode != api.Unavailable {
				t.Errorf("reason_code = %v, want %q", list.ReasonCode, api.Unavailable)
			}
		})
	}
}

func TestTheSnapshotSaysSavedTaskwarriorSettingsApplyAtTheNextStart(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := savedSettings(t, seams.Tasks{}, config.Taskwarrior{Disabled: true},
		func(settings *config.Taskwarrior) { settings.Disabled = false })

	// Act
	body := streamOnce(t, handler, "/api/events").Body.String()

	// Assert
	if tasks := firstSnapshot(t, body).Tasks; tasks.Available || tasks.Reason != settingsChanged {
		t.Errorf("tasks = %+v, want not available, reason %q", tasks, settingsChanged)
	}
}

func TestSettingsSavedBackAsTheyWereAtStartApply(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := savedSettings(t, fakeTaskwarrior().seams(), config.Taskwarrior{Program: taskwarriorProgram},
		func(*config.Taskwarrior) {})

	// Act
	recorder := get(t, handler, tasksPath)

	// Assert
	if list := decode[api.TaskList](t, recorder); !list.Available || list.Reason != "" {
		t.Errorf("answer = %d %+v, want Taskwarrior available with no reason", recorder.Code, list)
	}
}
