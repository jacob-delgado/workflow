// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// errOutputClosed is a stdout whose reader has gone, as when a pipe's far end
// exits early.
var errOutputClosed = errors.New("output closed")

// The commands the refused-output tests run, each named more than once in the
// package's tests.
const (
	statusCommand  = "status"
	reviewsCommand = "reviews"
	doctorCommand  = "doctor"
)

// refusingOutput fails every write.
type refusingOutput struct{}

var _ io.Writer = refusingOutput{}

func (refusingOutput) Write([]byte) (int, error) { return 0, errOutputClosed }

func TestDoctorJSONFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	env := environmentFor(t, place{dir: t.TempDir(), home: t.TempDir()})

	var stderr bytes.Buffer

	// Act
	err := cli.Execute(strings.Fields("doctor --json"), refusingOutput{}, &stderr, unusedPrompt(t), env)

	// Assert
	// A script reading the report would otherwise take a report cut short, or
	// none, for a run that succeeded.
	if !errors.Is(err, errOutputClosed) {
		t.Errorf("doctor --json into an output that refuses writes = %v, want that failure returned", err)
	}

	wantExit(t, err, 1)
}

func TestProseFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Each command, by name, and where it runs.
	cases := map[string]func(t *testing.T) string{
		statusCommand: featureRepo,
		reviewsCommand: func(t *testing.T) string {
			t.Helper()
			fakeGh(t, ghResponses{search: reviewSearch(reviewItem(1, "ana", "ex/repo", time.Now()))})

			return reviewsRepo(t)
		},
		doctorCommand: func(t *testing.T) string {
			t.Helper()

			return t.TempDir()
		},
	}

	for command, where := range cases {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env := environmentFor(t, place{dir: where(t), home: t.TempDir()})

			// Act
			err := cli.Execute([]string{command}, refusingOutput{}, io.Discard, unusedPrompt(t), env)

			// Assert
			if !errors.Is(err, errOutputClosed) {
				t.Errorf("%s into an output that refuses writes = %v, want that failure returned", command, err)
			}

			wantExit(t, err, 1)
		})
	}
}
