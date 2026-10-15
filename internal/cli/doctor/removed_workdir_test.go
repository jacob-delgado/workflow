// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"bytes"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestDoctorFromARemovedDirectoryStillReports(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunFromARemovedDirectory(t, t.TempDir(), "doctor")

	// Assert
	// doctor is what someone runs to find out what is wrong, so the report still
	// comes, naming the directory it could not read in place of a repository.
	got := fieldValue(printed.Stdout, "Repository")
	if !strings.HasPrefix(got, "(none — "+clitest.WorkdirUnread) {
		t.Errorf("Repository = %q, want it to say the working directory cannot be named:\n%s", got, printed.Stdout)
	}

	if err == nil || !strings.Contains(err.Error(), clitest.WorkdirUnread) {
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

	if problem, _ := repository["problem"].(string); !strings.HasPrefix(problem, clitest.WorkdirUnread) {
		t.Errorf("repository problem = %q, want the directory that could not be named", problem)
	}

	if problem, _ := report["config_problem"].(string); !strings.Contains(problem, clitest.WorkdirUnread) {
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
	err := cli.Execute([]string{"doctor"}, &stdout, &stderr, clitest.UnusedPrompt(t), env)

	// Assert
	want := "(none — " + dir + " is not in a git work tree)"
	if got := fieldValue(stdout.String(), "Repository"); got != want {
		t.Errorf("Repository = %q, want %q, from the one reading of the directory:\n%s", got, want, stdout.String())
	}

	if err == nil || strings.Contains(err.Error(), clitest.WorkdirUnread) {
		t.Errorf("doctor = %v, want it to fail on the missing configuration, not on the directory", err)
	}
}
