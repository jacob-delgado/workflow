// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// mergedStatusRepo is a feature branch for PROJ-7 whose pull request has
// merged, so it has no CI left to read.
func mergedStatusRepo(t *testing.T) string {
	t.Helper()

	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.MergedPull("Add login")})

	return clitest.JiraFeatureRepo(t, server.URL)
}

func TestStatusLineReadsAMergedPullRequestsReviewAsDone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedStatusRepo(t)

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "● Review") || !strings.Contains(output, "CI none") {
		t.Errorf("status line does not read the merged review as done:\n%s", output)
	}
}

func TestStatusJSONReadsAMergedPullRequestsReviewAsDone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mergedStatusRepo(t)

	// Act
	output, err := clitest.Run(t, repo, "status", "--json")
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
	t.Parallel()

	// Arrange
	// The store remembers pull request 7 announced as merged, as announce
	// leaves it once the merge is posted.
	repo := mergedStatusRepo(t)
	home := clitest.AnnouncedEarlier(t, messaging.MomentMerged)

	// Act
	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "status")
	if err != nil {
		t.Fatalf("status: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.Stdout, "● Slack") {
		t.Errorf("status line does not read the merged announcement as done:\n%s", printed.Stdout)
	}
}

func TestStatusLineReadsAnAnnouncementOfAClosedPullRequestAsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// The store remembers pull request 7 announced as ready, before it was
	// closed without merging: there is no pull request left to announce.
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.ClosedPull("Add login")})
	repo := clitest.JiraFeatureRepo(t, server.URL)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "status")
	if err != nil {
		t.Fatalf("status: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.Stdout, "○ Review") || !strings.Contains(printed.Stdout, "○ Slack") {
		t.Errorf("status line reads a closed pull request's announcement as made:\n%s", printed.Stdout)
	}
}
