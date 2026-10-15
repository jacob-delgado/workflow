// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestDoctorJSONFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no configuration, the report would fail on that too, but a report
	// nobody received has no verdict to give.
	env := clitest.EnvironmentFor(t, clitest.Place{Dir: t.TempDir(), Home: t.TempDir()})

	// Act
	err := cli.Execute(strings.Fields("doctor --json"), clitest.RefusingOutput{}, io.Discard, clitest.UnusedPrompt(t), env)

	// Assert
	// A script reading the report would otherwise take a report cut short, or
	// none, for a run that succeeded.
	if !errors.Is(err, clitest.ErrOutputClosed) || errors.Is(err, config.ErrNotFound) {
		t.Errorf("doctor --json into an output that refuses writes = %v, want that failure and no other", err)
	}

	wantExit(t, err, 1)
}
