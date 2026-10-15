// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver/tasks"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// This file is also the one that imports package tasks itself, which gobco's
// bridge for black-box tests needs: without it gobco cannot measure the
// package.

// TestDeclinedStartReachesTheKitsFaultAsErrStartDeclined pins what the
// server's fault classes word: a start Taskwarrior declined is handed to the
// kit's Fault as ErrStartDeclined, with Taskwarrior's own decline still in it.
func TestDeclinedStartReachesTheKitsFaultAsErrStartDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	var heard error

	handlers := tasks.New(tasks.Seams{Tasks: seams.Tasks{
		Start: func(_ context.Context, _ string) error { return taskwarrior.ErrNothingChanged },
	}}, config.Taskwarrior{}, tasks.Kit{
		Fault: func(err error) api.Problem {
			heard = err

			return api.Problem{Detail: "declined"}
		},
		Config: func() config.Taskwarrior { return config.Taskwarrior{} },
		Now:    time.Now,
	})

	// Act
	_, err := handlers.StartTask(t.Context(), api.StartTaskRequestObject{UUID: webtest.PlainUUID})
	// Assert
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if !errors.Is(heard, tasks.ErrStartDeclined) || !errors.Is(heard, taskwarrior.ErrNothingChanged) {
		t.Errorf("Fault heard %v, want ErrStartDeclined wrapping ErrNothingChanged", heard)
	}
}
