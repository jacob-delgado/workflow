// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// workdirUnread is what a command says when it cannot name the directory it was
// run from.
const workdirUnread = "determining the working directory"

// errDirectoryGone is a working directory that cannot be named, as a shell is
// left standing in one when `git worktree remove` deletes the directory it is
// in.
var errDirectoryGone = errors.New("getwd: no such file or directory")

// runFromARemovedDirectory runs the command tree from a working directory that
// cannot be named. The home is the test's, for a configuration put there first.
func runFromARemovedDirectory(t *testing.T, home string, args ...string) (streams, error) {
	t.Helper()

	env := environmentFor(t, place{dir: t.TempDir(), home: home})
	env.WorkingDir = func() (string, error) { return "", errDirectoryGone }

	var stdout, stderr bytes.Buffer

	err := cli.Execute(args, &stdout, &stderr, unusedPrompt(t), env)

	return streams{stdout: stdout.String(), stderr: stderr.String()}, err
}

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
			printed, err := runFromARemovedDirectory(t, t.TempDir(), args...)

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
	printed, err := runFromARemovedDirectory(t, t.TempDir(), "--log", "requests.log", "doctor")

	// Assert
	if err == nil || !strings.HasPrefix(err.Error(), "workflow: opening the request log: "+workdirUnread) {
		t.Errorf("--log requests.log from a removed directory = %v, want it to say the log cannot be opened "+
			"for the directory it cannot name (%+v)", err, printed)
	}
}

func TestDoctorFromARemovedDirectoryStillReports(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runFromARemovedDirectory(t, t.TempDir(), "doctor")

	// Assert
	// doctor is what someone runs to find out what is wrong, so the report still
	// comes, naming the directory it could not read in place of a repository.
	got := fieldValue(printed.stdout, "Repository")
	if !strings.HasPrefix(got, "(none — cannot read the working directory") {
		t.Errorf("Repository = %q, want it to say the working directory cannot be read:\n%s", got, printed.stdout)
	}

	if err == nil || !strings.Contains(err.Error(), workdirUnread) {
		t.Errorf("doctor from a removed directory = %v, want it to fail naming the directory it could not read", err)
	}
}

func TestDoctorJSONFromARemovedDirectoryReportsNoWorkTree(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runFromARemovedDirectory(t, t.TempDir(), "doctor", "--json")

	// Assert
	report := decodeReport(t, printed.stdout)

	repository, ok := report["repository"].(map[string]any)
	if !ok || repository["inside_work_tree"] != false {
		t.Errorf("repository = %v, want no work tree for a directory that is gone", report["repository"])
	}

	if problem, _ := report["config_problem"].(string); !strings.Contains(problem, workdirUnread) {
		t.Errorf("config_problem = %q, want the directory that could not be read", problem)
	}

	wantExit(t, err, 1)
}

func TestBranchCompletionFromARemovedDirectoryOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The home's configuration names a tracker with an issue to offer, but with
	// no directory to start from, completion looks for nothing.
	home := t.TempDir()
	server := jiraServer(t, http.StatusOK, searchFixture("PROJ-7"), new(atomic.Bool))
	writeConfigFor(t, home, server.URL)

	// Act
	printed, err := runFromARemovedDirectory(t, home, "__complete", "branch", "")

	// Assert
	// A failure would spill into the shell; the shell is told only that there is
	// nothing to offer, not even a file name.
	if err != nil || printed.stdout != ":4\n" {
		t.Errorf("completion from a removed directory = %v, offering %q; want nothing offered and no failure",
			err, printed.stdout)
	}
}
