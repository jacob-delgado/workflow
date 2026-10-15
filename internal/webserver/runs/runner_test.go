// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestARunnerCountsTheUnmanagedHooksItsSeamsFind(t *testing.T) {
	t.Parallel()

	// Arrange
	var indexWrites sync.Mutex

	found := webtest.OldHooks()
	given := runs.Seams{Hooks: seams.Hooks{
		Existing: func(context.Context) ([]hooks.GitHook, bool) { return found, false },
	}}
	runner := runs.New(given, func(error) {}, &indexWrites)

	// Act
	counted := runner.HooksUnmanaged(t.Context())

	// Assert
	if counted != len(found) {
		t.Errorf("HooksUnmanaged() = %d, want %d", counted, len(found))
	}
}
