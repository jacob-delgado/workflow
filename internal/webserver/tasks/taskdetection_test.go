// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks_test

// The stream's summary and the Tasks list say alike whether Taskwarrior can
// be asked.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestTheStreamFindsTaskwarriorOnceTheListHasFoundIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The stream's search found no Taskwarrior, which it holds for a minute;
	// Taskwarrior is then installed, and the Tasks list finds it.
	fake := webtest.FakeTaskwarrior()
	fake.Fail["install"] = taskwarrior.ErrNotInstalled
	handler := webtest.Serve(t, tasksDeps(fake), config.Default())
	webtest.StreamOnce(t, handler, "/api/events")

	delete(fake.Fail, "install")

	if listed := webtest.Decode[api.TaskList](t, webtest.Get(t, handler, webtest.TasksPath)); !listed.Available {
		t.Fatalf("the list says %q, want Taskwarrior found", listed.Reason)
	}

	// Act
	frame := webtest.FirstSnapshot(t, webtest.StreamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !frame.Tasks.Available {
		t.Errorf("the frame after the list found Taskwarrior says %q, want it available as the list said",
			frame.Tasks.Reason)
	}
}
