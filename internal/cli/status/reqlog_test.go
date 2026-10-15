// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestStatusReportsARequestLogItCannotOpen(t *testing.T) {
	t.Parallel()

	// --request-log names a file whose parent directory does not exist, so
	// opening the log fails before anything is asked of a service.
	cases := [][]string{
		strings.Fields("status"),
		strings.Fields("status ."),
	}

	for _, args := range cases {
		name := strings.Join(append([]string{"workflow"}, args...), " ")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			logPath := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

			// Act
			_, err := clitest.Run(t, t.TempDir(), append([]string{"--request-log", logPath}, args...)...)

			// Assert
			// The error must name the request log: status outside a repository
			// fails anyway, so a plain "an error occurred" check would pass
			// whatever the cause.
			if err == nil || !strings.Contains(err.Error(), "request log") {
				t.Errorf("%s = %v, want it to fail opening the log in a missing directory", name, err)
			}
		})
	}
}

func TestLogReachesStatus(t *testing.T) {
	t.Parallel()

	// Bare status reads the directory it runs in; status DIR... wires each
	// directory it is given on its own, and must log those requests too.
	cases := [][]string{
		strings.Fields("status"),
		strings.Fields("status ."),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
			repo := clitest.JiraFeatureRepo(t, server.URL)
			logPath := filepath.Join(t.TempDir(), "requests.log")

			// Act
			output, err := clitest.Run(t, repo, append([]string{"--request-log", logPath}, args...)...)
			if err != nil {
				t.Fatalf("%s: %v (%s)", name, err, output)
			}

			// Assert
			// status reads the issue from Jira and the pull request through gh;
			// both requests are outlined for a bug report.
			logged := clitest.ReadLog(t, logPath)
			if !strings.Contains(logged, "jira  GET") || !strings.Contains(logged, "forge GET") {
				t.Errorf("the request log does not outline %s's requests:\n%s", name, logged)
			}
		})
	}
}
