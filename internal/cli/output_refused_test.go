// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// doctorCommand is the command the refused-output, help and removed-directory
// tests run.
const doctorCommand = "doctor"

func TestProseFailsWhenItsOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Each command, by name, and where it runs.
	cases := map[string]func(t *testing.T) string{
		"status": clitest.FeatureRepo,
		"reviews": func(t *testing.T) string {
			t.Helper()
			clitest.FakeGh(t, clitest.GhResponses{Search: reviewSearch(reviewItem(1, "ana", "ex/repo", time.Now()))})

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
			env := clitest.EnvironmentFor(t, clitest.Place{Dir: where(t), Home: t.TempDir()})

			// Act
			err := cli.Execute([]string{command}, clitest.RefusingOutput{}, io.Discard, clitest.UnusedPrompt(t), env)

			// Assert
			if !errors.Is(err, clitest.ErrOutputClosed) {
				t.Errorf("%s into an output that refuses writes = %v, want that failure returned", command, err)
			}

			wantExit(t, err, 1)
		})
	}
}
