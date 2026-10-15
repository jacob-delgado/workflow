// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestStatusFromARemovedDirectorySaysItCannotNameIt(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"reading the status":                   strings.Fields("status"),
		"reading a directory named relatively": strings.Fields("status ."),
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), args...)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "determining the working directory") {
				t.Errorf("%s from a removed directory = %v, want it to say it cannot name the directory (%+v)",
					name, err, printed)
			}
		})
	}
}

// Each directory status names relatively must be read from one directory, so
// it reads the working directory once: a second read could find another, or
// none, as a shell left standing in a directory `git worktree remove` deletes
// does.
func TestStatusOfTwoDirectoriesNamedRelativelyReadsTheWorkingDirectoryOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	args := []string{statusCommand, ".", "."}
	env := clitest.EnvironmentFor(t, clitest.Place{Dir: t.TempDir(), Home: t.TempDir()})
	read := env.WorkingDir

	var reads atomic.Int32

	env.WorkingDir = func() (string, error) {
		reads.Add(1)

		return read()
	}

	var stdout, stderr bytes.Buffer

	// Act
	_ = cli.Execute(args, &stdout, &stderr, clitest.UnusedPrompt(t), env)

	// Assert
	if got := reads.Load(); got != 1 {
		t.Errorf("%v read the working directory %d times, want once", args, got)
	}
}

func TestStatusRefusesAConfigurationItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file that exists but does not parse is not replaced by the defaults:
	// status read on settings nobody chose would read the wrong things.
	dir := t.TempDir()
	clitest.WriteFile(t, dir, "{not json")

	// Act
	_, err := clitest.Run(t, dir, statusCommand)

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("status with a malformed configuration = %v, want it refused", err)
	}

	wantExit(t, err, 3)
}
