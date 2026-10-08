// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// completeConfig is a configuration doctor finds nothing wrong with, so its
// verdict is about the tooling alone.
const completeConfig = `{"jira": {"base_url": "https://jira.example.com", "token": "t"},` +
	` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`

// fakeTaskwarrior puts a task program first on PATH that answers _version with
// version and _show with show, and returns its path. An empty version answers
// as go-task does: it has no _version task.
func fakeTaskwarrior(t *testing.T, version, show string) string {
	t.Helper()

	if version == "" {
		return fakeTaskProgram(t, `echo 'task: Task "_version" does not exist' >&2; exit 200`, show)
	}

	return fakeTaskProgram(t, "echo "+version, show)
}

// fakeTaskProgram puts a task program first on PATH that runs versionAnswer, a
// line of shell, for _version and answers _show with show, and returns its path.
func fakeTaskProgram(t *testing.T, versionAnswer, show string) string {
	t.Helper()

	program := filepath.Join(programsOf(t), "task")

	writeExecutable(t, program, "#!/bin/sh\n"+
		"for last in \"$@\"; do :; done\n"+
		"case \"$last\" in\n"+
		"_version) "+versionAnswer+" ;;\n"+
		"_show) printf '%s\\n' '"+show+"' ;;\n"+
		"esac\n", 0o755)

	return program
}

// taskwarriorRow is the line doctor's tooling section prints for Taskwarrior.
func taskwarriorRow(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if row, found := strings.CutPrefix(strings.TrimSpace(line), "taskwarrior "); found {
			return strings.TrimSpace(row)
		}
	}

	return ""
}

func TestDoctorReportsTaskwarriorWhenItAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	program := fakeTaskwarrior(t, "3.5.0", "uda.jiraid.type=string")

	// Act
	output, _ := run(t, t.TempDir(), "doctor", "--json")

	// Assert
	report := decodeReport(t, output)

	tooling, _ := report["tooling"].([]any)
	for _, entry := range tooling {
		if fact, _ := entry.(map[string]any); fact["name"] == "taskwarrior" {
			if want := "3.5.0 at " + program; fact["found"] != true || fact["detail"] != want {
				t.Errorf("the taskwarrior row = %v, want found with detail %q", fact, want)
			}

			return
		}
	}

	t.Errorf("doctor --json has no taskwarrior row:\n%s", output)
}

func TestDoctorPrintsTheTaskwarriorItFound(t *testing.T) {
	t.Parallel()

	// Arrange
	program := fakeTaskwarrior(t, "3.5.0", "uda.jiraid.type=string")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if got, want := taskwarriorRow(output), "found — 3.5.0 at "+program; got != want {
		t.Errorf("the taskwarrior row = %q, want %q:\n%s", got, want, output)
	}
}

func TestDoctorTellsGoTaskFromTaskwarrior(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeTaskwarrior(t, "", "")

	dir := t.TempDir()
	writeFile(t, dir, completeConfig)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if row := taskwarriorRow(output); !strings.HasPrefix(row, "not usable — task on PATH is not Taskwarrior") ||
		!strings.Contains(row, "taskwarrior.program") {
		t.Errorf("the taskwarrior row = %q, want it told apart from go-task and taskwarrior.program named", row)
	}

	if err != nil {
		t.Errorf("doctor = %v, want success: Taskwarrior is optional", err)
	}
}

func TestDoctorHintsTheUDALinesWhenTaskrcLacksThem(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeTaskwarrior(t, "3.5.0", "data.location=/tmp")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if row := taskwarriorRow(output); !strings.HasPrefix(row, "found — 3.5.0") ||
		!strings.Contains(row, "uda.jiraid.type=string") {
		t.Errorf("the taskwarrior row = %q, want it found and the UDA lines named", row)
	}
}

func TestDoctorSaysTaskwarriorIsDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeTaskwarrior(t, "3.5.0", "")

	dir := t.TempDir()
	writeFile(t, dir, `{"taskwarrior": {"disabled": true}}`)

	// Act
	output, _ := run(t, dir, "doctor")

	// Assert
	if row := taskwarriorRow(output); row != "not usable — disabled by taskwarrior.disabled" {
		t.Errorf("the taskwarrior row = %q, want it reported disabled", row)
	}
}

func TestDoctorSaysTaskrcIsMalformedWithoutQuotingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taskwarrior quotes the line it cannot parse, secret and all.
	fakeTaskProgram(t, `echo "Malformed entry 'sync.encryption_secret hunter2' in config file." >&2; exit 2`, "")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if row := taskwarriorRow(output); row != "not usable — taskrc has a malformed line" {
		t.Errorf("the taskwarrior row = %q, want the malformed taskrc named", row)
	}

	if strings.Contains(output, "hunter2") {
		t.Errorf("doctor printed the taskrc's secret:\n%s", output)
	}
}

func TestDoctorSaysTaskwarriorWasNeverRunAndWhichToRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// go-task can come first on PATH, so task alone may not run this one.
	program := fakeTaskProgram(t, `echo 'Cannot proceed without rc file.' >&2; exit 2`, "")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if row, want := taskwarriorRow(output), "not usable — installed but never run; run "+program+" once"; row != want {
		t.Errorf("the taskwarrior row = %q, want %q", row, want)
	}
}

func TestDoctorSaysWhyTaskwarriorCannotStart(t *testing.T) {
	t.Parallel()

	// Arrange
	const words = "Could not read include file '/opt/homebrew/Cellar/task/3.4.1/dark-256.theme'."

	fakeTaskProgram(t, `echo "`+words+`" >&2; exit 2`, "")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if row := taskwarriorRow(output); row != "not usable — "+words {
		t.Errorf("the taskwarrior row = %q, want Taskwarrior's own words %q", row, words)
	}
}

func TestDoctorSaysTaskwarriorIsTooOld(t *testing.T) {
	t.Parallel()

	// Arrange
	program := fakeTaskwarrior(t, "2.6.2", "")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	row := taskwarriorRow(output)
	if !strings.HasPrefix(row, "not usable — ") || !strings.Contains(row, "2.6.2 at "+program) ||
		!strings.Contains(row, "3.5.0 or newer") {
		t.Errorf("the taskwarrior row = %q, want the version found, where, and the one needed", row)
	}
}
