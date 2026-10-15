// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestDoctorJSONFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	env := clitest.EnvironmentFor(t, clitest.Place{Dir: t.TempDir(), Home: t.TempDir()})

	var stderr bytes.Buffer

	// Act
	err := cli.Execute(strings.Fields("doctor --json"), clitest.RefusingOutput{}, &stderr, clitest.UnusedPrompt(t), env)

	// Assert
	// A script reading the report would otherwise take a report cut short, or
	// none, for a run that succeeded.
	if !errors.Is(err, clitest.ErrOutputClosed) {
		t.Errorf("doctor --json into an output that refuses writes = %v, want that failure returned", err)
	}

	wantExit(t, err, 1)
}
