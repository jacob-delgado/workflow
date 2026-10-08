// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// errRefused stands in for how a drained program ended.
var errRefused = errors.New("git: exit status 1")

// written is output a program has finished writing, with how it ended.
func written(lines []string, waitErr error) proc.Output {
	channel := make(chan string, len(lines))
	for _, line := range lines {
		channel <- line
	}

	close(channel)

	return proc.Output{Lines: channel, Wait: func() error { return waitErr }, Stop: func() {}}
}

func TestDrainReadsEveryLineNeutralizedThenHowTheProgramEnded(t *testing.T) {
	t.Parallel()

	// Arrange
	// What a hook or a remote writes reaches every surface through Drain, so it
	// comes back as text: colors stripped whole, a control made visible, a tab
	// kept.
	output := written([]string{"remote: \x1b]0;title\a", "\x1b[31m✗ lint\x1b[0m", "\tdetail", "bell\x07"}, errRefused)

	// Act
	lines, err := output.Drain()

	// Assert
	want := []string{"remote: ", "✗ lint", "\tdetail", "bell�"}
	if !errors.Is(err, errRefused) || !slices.Equal(lines, want) {
		t.Errorf("Drain = %q, %v; want %q and the program's own ending", lines, err, want)
	}
}

func TestDrainOfAProgramThatSucceededReportsNoFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	output := written([]string{"To origin"}, nil)

	// Act
	lines, err := output.Drain()

	// Assert
	if err != nil || !slices.Equal(lines, []string{"To origin"}) {
		t.Errorf("Drain = %q, %v; want its line and no failure", lines, err)
	}
}
