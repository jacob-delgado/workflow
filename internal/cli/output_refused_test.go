// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// errOutputClosed is a stdout whose reader has gone, as when a pipe's far end
// exits early.
var errOutputClosed = errors.New("output closed")

// refusingOutput fails every write.
type refusingOutput struct{}

var _ io.Writer = refusingOutput{}

func (refusingOutput) Write([]byte) (int, error) { return 0, errOutputClosed }

func TestDoctorJSONFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	// Arrange
	for name, value := range isolatedEnvironment(t.TempDir()) {
		t.Setenv(name, value)
	}

	t.Chdir(t.TempDir())

	var stderr bytes.Buffer

	// Act
	err := cli.Execute(strings.Fields("doctor --json"), refusingOutput{}, &stderr, unusedPrompt(t))

	// Assert
	// A script reading the report would otherwise take a report cut short, or
	// none, for a run that succeeded.
	if !errors.Is(err, errOutputClosed) {
		t.Errorf("doctor --json into an output that refuses writes = %v, want that failure returned", err)
	}

	wantExit(t, err, 1)
}
