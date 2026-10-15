// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// logOption is the option that names the request log, and relativeLog a log
// named relatively, which is read from the working directory.
const (
	logOption   = "--request-log"
	relativeLog = "requests.log"
)

func TestACommandFromARemovedDirectorySaysItCannotNameIt(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"reading the status":                   strings.Fields("status"),
		"reading a directory named relatively": strings.Fields("status ."),
		"showing the configuration":            strings.Fields("config show"),
		"writing a template":                   strings.Fields("config init --template"),
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), args...)

			// Assert
			if err == nil || !strings.Contains(err.Error(), clitest.WorkdirUnread) {
				t.Errorf("%s from a removed directory = %v, want it to say it cannot name the directory (%+v)",
					name, err, printed)
			}
		})
	}
}

func TestALogNamedRelativelyFromARemovedDirectorySaysItCannotBeOpened(t *testing.T) {
	t.Parallel()

	// Act
	// doctor opens the log before anything else reads the directory, so only
	// the log's own reading of it can fail first.
	printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), logOption, relativeLog, doctorCommand)

	// Assert
	if err == nil || !strings.HasPrefix(err.Error(), "opening the request log: "+clitest.WorkdirUnread) {
		t.Errorf("--request-log requests.log from a removed directory = %v, want it to say the log cannot be opened "+
			"for the directory it cannot name (%+v)", err, printed)
	}
}

func TestBranchCompletionFromARemovedDirectoryOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The home's configuration names a tracker with an issue to offer, but with
	// no directory to start from, completion looks for nothing.
	home := t.TempDir()
	server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-7"), new(atomic.Bool))
	clitest.WriteConfigFor(t, home, server.URL)

	// Act
	printed, err := clitest.RunFromARemovedDirectory(t, home, "__complete", "branch", "")

	// Assert
	// A failure would spill into the shell; the shell is told only that there is
	// nothing to offer, not even a file name.
	if err != nil || printed.Stdout != ":4\n" {
		t.Errorf("completion from a removed directory = %v, offering %q; want nothing offered and no failure",
			err, printed.Stdout)
	}
}

// A command's sections, its request log and each directory it names relatively
// must all be read from one directory, so a command reads it once: a second
// read could find another, or none, as a shell left standing in a directory
// `git worktree remove` deletes does.
func TestACommandReadsTheDirectoryItWasRunFromOnce(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"doctor, logging to a file named relatively":     {logOption, relativeLog, doctorCommand},
		"the status of two directories named relatively": {statusCommand, ".", "."},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
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
		})
	}
}
