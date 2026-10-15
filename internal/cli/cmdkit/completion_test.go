// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// completeBranch runs the shell-completion hook for `workflow branch <toComplete>`
// in dir, the way a shell would, and returns everything it offered.
func completeBranch(t *testing.T, dir, toComplete string) string {
	t.Helper()

	output, err := clitest.Run(t, dir, "__complete", "branch", toComplete)
	if err != nil {
		t.Fatalf("__complete branch %q: %v (%s)", toComplete, err, output)
	}

	return output
}

func TestBranchCompletionOffersAssignedIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-7", "PROJ-8"), new(atomic.Bool))
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	if !strings.Contains(output, "PROJ-7") || !strings.Contains(output, "PROJ-8") {
		t.Errorf("completion does not offer the assigned issues:\n%s", output)
	}
}

func TestBranchCompletionFiltersByPrefix(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-7", "TASK-3"), new(atomic.Bool))
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "PROJ")

	// Assert
	if !strings.Contains(output, "PROJ-7") {
		t.Errorf("completion drops a key that matches the typed prefix:\n%s", output)
	}

	if strings.Contains(output, "TASK-3") {
		t.Errorf("completion offers a key that does not match the typed prefix:\n%s", output)
	}
}

func TestBranchCompletionOffersNothingWhenTheTrackerCannotBeReached(t *testing.T) {
	t.Parallel()

	// Arrange
	// The tracker would have keys to offer, but every request fails.
	server := clitest.JiraServer(t, http.StatusInternalServerError, clitest.SearchFixture("PROJ-7"), new(atomic.Bool))
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	// Completion must not spill an error into the shell: the key the tracker
	// would have offered is absent, not surfaced as a failure.
	if strings.Contains(output, "PROJ-7") {
		t.Errorf("completion offered a key from a tracker it could not read:\n%s", output)
	}
}
