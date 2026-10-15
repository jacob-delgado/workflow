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

// workdirUnread is what a command says when it cannot name the directory it was
// run from.
const workdirUnread = "determining the working directory"

// logOption is the option that names the request log, and relativeLog a log
// named relatively, which is read from the working directory.
const (
	logOption   = "--request-log"
	relativeLog = "requests.log"
)

func TestACommandFromARemovedDirectorySaysItCannotNameIt(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"showing the configuration": strings.Fields("config show"),
		"writing a template":        strings.Fields("config init --template"),
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), args...)

			// Assert
			if err == nil || !strings.Contains(err.Error(), workdirUnread) {
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
	if err == nil || !strings.HasPrefix(err.Error(), "opening the request log: "+workdirUnread) {
		t.Errorf("--request-log requests.log from a removed directory = %v, want it to say the log cannot be opened "+
			"for the directory it cannot name (%+v)", err, printed)
	}
}

func TestDoctorFromARemovedDirectoryStillReports(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), "doctor")

	// Assert
	// doctor is what someone runs to find out what is wrong, so the report still
	// comes, naming the directory it could not read in place of a repository.
	got := fieldValue(printed.Stdout, "Repository")
	if !strings.HasPrefix(got, "(none — "+workdirUnread) {
		t.Errorf("Repository = %q, want it to say the working directory cannot be named:\n%s", got, printed.Stdout)
	}

	if err == nil || !strings.Contains(err.Error(), workdirUnread) {
		t.Errorf("doctor from a removed directory = %v, want it to fail naming the directory it could not read", err)
	}
}

func TestDoctorJSONFromARemovedDirectoryReportsNoWorkTree(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), "doctor", "--json")

	// Assert
	report := decodeReport(t, printed.Stdout)

	repository, ok := report["repository"].(map[string]any)
	if !ok || repository["inside_work_tree"] != false {
		t.Errorf("repository = %v, want no work tree for a directory that is gone", report["repository"])
	}

	if problem, _ := repository["problem"].(string); !strings.HasPrefix(problem, workdirUnread) {
		t.Errorf("repository problem = %q, want the directory that could not be named", problem)
	}

	if problem, _ := report["config_problem"].(string); !strings.Contains(problem, workdirUnread) {
		t.Errorf("config_problem = %q, want the directory that could not be read", problem)
	}

	wantExit(t, err, 1)
}

func TestDoctorReportsOnTheOneReadingOfItsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// The directory is removed once doctor has read it: every section reports
	// on that one reading, so none of them finds it gone.
	dir := t.TempDir()
	env := clitest.EnvironmentFor(t, clitest.Place{Dir: dir, Home: t.TempDir()})

	var reads atomic.Int32

	env.WorkingDir = func() (string, error) {
		if reads.Add(1) > 1 {
			return "", clitest.ErrDirectoryGone
		}

		return dir, nil
	}

	var stdout, stderr bytes.Buffer

	// Act
	err := cli.Execute([]string{doctorCommand}, &stdout, &stderr, clitest.UnusedPrompt(t), env)

	// Assert
	want := "(none — " + dir + " is not in a git work tree)"
	if got := fieldValue(stdout.String(), "Repository"); got != want {
		t.Errorf("Repository = %q, want %q, from the one reading of the directory:\n%s", got, want, stdout.String())
	}

	if err == nil || strings.Contains(err.Error(), workdirUnread) {
		t.Errorf("doctor = %v, want it to fail on the missing configuration, not on the directory", err)
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
func TestDoctorLoggingRelativelyReadsTheDirectoryItWasRunFromOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	args := []string{logOption, relativeLog, doctorCommand}
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
