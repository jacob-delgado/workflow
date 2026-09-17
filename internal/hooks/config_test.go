// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

func TestRunCommandRunsOneHookWithoutATerminal(t *testing.T) {
	t.Parallel()

	// Act
	command := hooks.RunCommand("/work", preCommit)

	// Assert
	if command.Dir != "/work" || command.Name != "lefthook" ||
		!slices.Equal(command.Args, []string{"run", preCommit, "--no-tty"}) {
		t.Errorf("RunCommand = %+v", command)
	}
}
