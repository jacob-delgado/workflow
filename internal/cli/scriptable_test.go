// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// The scriptable write commands are wired end to end here; without a tracker,
// a repository or Slack they fail with a reason rather than a panic.

func TestBranchCommandNeedsATracker(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "branch", "PROJ-1")

	// Assert
	if err == nil {
		t.Error("branch for an issue with no tracker configured returned no error")
	}
}

func TestPRCommandReadsTheBranch(t *testing.T) {
	// Act
	// Outside a repository there is no branch to open a pull request for.
	_, err := run(t, t.TempDir(), "pr")

	// Assert
	if err == nil {
		t.Error("pr outside a repository returned no error")
	}
}

func TestAnnounceCommandNeedsMessaging(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "announce")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "messaging") {
		t.Errorf("announce with no messaging returned %v, want a not-configured error", err)
	}
}

// searchFixture is the Jira body for a search of the issues assigned to you.
func searchFixture(keys ...string) string {
	issues := make([]string, 0, len(keys))
	for _, key := range keys {
		issues = append(issues, `{"key":"`+key+`","fields":{"summary":"x",`+
			`"issuetype":{"name":"Bug"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	}

	return `{"total":` + strconv.Itoa(len(keys)) + `,"issues":[` + strings.Join(issues, ",") + `]}`
}

// completeBranch runs the shell-completion hook for `workflow branch <toComplete>`
// in dir, the way a shell would, and returns everything it offered.
func completeBranch(t *testing.T, dir, toComplete string) string {
	t.Helper()

	output, err := run(t, dir, "__complete", "branch", toComplete)
	if err != nil {
		t.Fatalf("__complete branch %q: %v (%s)", toComplete, err, output)
	}

	return output
}

func TestBranchCompletionOffersAssignedIssues(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, searchFixture("PROJ-7", "PROJ-8"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	if !strings.Contains(output, "PROJ-7") || !strings.Contains(output, "PROJ-8") {
		t.Errorf("completion does not offer the assigned issues:\n%s", output)
	}
}

func TestBranchCompletionFiltersByPrefix(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, searchFixture("PROJ-7", "TASK-3"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

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
	// Arrange
	// The tracker would have keys to offer, but every request fails.
	server := jiraServer(t, http.StatusInternalServerError, searchFixture("PROJ-7"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	// Completion must not spill an error into the shell: the key the tracker
	// would have offered is absent, not surfaced as a failure.
	if strings.Contains(output, "PROJ-7") {
		t.Errorf("completion offered a key from a tracker it could not read:\n%s", output)
	}
}
