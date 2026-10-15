// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestLogReachesDoctorOnline(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online", "--request-log", logPath)
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	// Assert
	// doctor --online is what a bug report asks for first, so its questions to
	// each service are what the log most needs to hold.
	if logged := clitest.ReadLog(t, logPath); !strings.Contains(logged, "jira  GET  /rest/api/2/myself") {
		t.Errorf("the request log does not outline doctor's check of Jira:\n%s", logged)
	}
}

func TestDoctorStopsAtARequestLogItCannotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// The log's directory does not exist, so opening it fails before doctor
	// reports anything or asks any service.
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))
	logPath := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online", "--request-log", logPath)

	// Assert
	if err == nil || !strings.HasPrefix(err.Error(), "opening the request log") {
		t.Errorf("doctor = %v, want it to stop at the request log it cannot open", err)
	}

	if strings.Contains(output, "Version:") {
		t.Errorf("doctor reported with no request log to outline its checks in:\n%s", output)
	}
}
