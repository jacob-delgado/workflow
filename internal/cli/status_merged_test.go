// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

// mergedStatusRepo is a feature branch for PROJ-7 whose pull request has
// merged, so it has no CI left to read.
func mergedStatusRepo(t *testing.T) string {
	t.Helper()

	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: mergedPull("Add login")})

	return statusFeatureRepo(t, server.URL)
}

func TestStatusLineReadsAMergedPullRequestsReviewAsDone(t *testing.T) {
	// Arrange
	repo := mergedStatusRepo(t)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "● Review") || !strings.Contains(output, "CI none") {
		t.Errorf("status line does not read the merged review as done:\n%s", output)
	}
}

func TestStatusJSONReadsAMergedPullRequestsReviewAsDone(t *testing.T) {
	// Arrange
	repo := mergedStatusRepo(t)

	// Act
	output, err := run(t, repo, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (%s)", err, output)
	}

	// Assert
	var report struct {
		Stages []map[string]any `json:"stages"`
	}

	err = json.Unmarshal([]byte(output), &report)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output)
	}

	if len(report.Stages) != 5 || report.Stages[3]["name"] != "Review" || report.Stages[3]["state"] != stageDone {
		t.Errorf("status JSON stages = %+v, want the Review stage done", report.Stages)
	}
}

func TestStatusLineReadsAMergedAnnouncementAsDone(t *testing.T) {
	// Arrange
	// The store remembers pull request 7 announced as merged, as announce
	// leaves it once the merge is posted.
	repo := mergedStatusRepo(t)
	home := announcedEarlier(t, messaging.MomentMerged)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "status")
	if err != nil {
		t.Fatalf("status: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.stdout, "● Slack") {
		t.Errorf("status line does not read the merged announcement as done:\n%s", printed.stdout)
	}
}

func TestStatusLineReadsAnAnnouncementOfAClosedPullRequestAsNotStarted(t *testing.T) {
	// Arrange
	// The store remembers pull request 7 announced as ready, before it was
	// closed without merging: there is no pull request left to announce.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: closedPull("Add login")})
	repo := statusFeatureRepo(t, server.URL)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "status")
	if err != nil {
		t.Fatalf("status: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.stdout, "○ Review") || !strings.Contains(printed.stdout, "○ Slack") {
		t.Errorf("status line reads a closed pull request's announcement as made:\n%s", printed.stdout)
	}
}
