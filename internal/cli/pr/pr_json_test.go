// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// prReport is what `workflow pr --json` prints.
type prReport struct {
	Pull struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
		Title  string `json:"title"`
		Draft  bool   `json:"draft"`
		Head   string `json:"head"`
		Base   string `json:"base"`
		Body   string `json:"body"`
	} `json:"pull"`
	Warning   string           `json:"warning"`
	FollowUps []followUpReport `json:"follow_ups"`
}

// followUpReport is one offer `pr` made once the pull request was open.
type followUpReport struct {
	Action   string `json:"action"`
	IssueKey string `json:"issue_key"`
	Status   string `json:"status"`
	Done     bool   `json:"done"`
}

// decodePR parses stdout as `pr --json`'s report, failing the test when it is
// anything else.
func decodePR(t *testing.T, stdout string) prReport {
	t.Helper()

	var report prReport

	err := json.Unmarshal([]byte(stdout), &report)
	if err != nil {
		t.Fatalf("pr --json printed something other than its report: %v\n%s", err, stdout)
	}

	return report
}

func TestPRAsJSONPrintsWhatItOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, _ := reviewRepo(t, reviewMoves)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--json")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes --json: %v (%+v)", err, printed)
	}

	report := decodePR(t, printed.Stdout)
	if report.Pull.Number != 7 || report.Pull.URL != "https://github.com/owner/repo/pull/7" ||
		report.Pull.Head != "fix/PROJ-2-thing" || report.Pull.Base != "main" {
		t.Errorf("pull = %+v, want #7 from fix/PROJ-2-thing into main", report.Pull)
	}

	const issueKey = "PROJ-2"

	want := []followUpReport{
		{Action: "link", IssueKey: issueKey, Done: true},
		{Action: "transition", IssueKey: issueKey, Status: clitest.StatusInReview, Done: true},
	}
	if !reflect.DeepEqual(report.FollowUps, want) {
		t.Errorf("follow_ups = %+v, want the link and the move, both made", report.FollowUps)
	}

	// What is said about the open stays on stderr, the preview among it.
	if !strings.Contains(printed.Stderr, "Linked #7 on PROJ-2.") || !strings.Contains(printed.Stderr, "Open work") {
		t.Errorf("pr --json did not keep its notes on stderr:\n%s", printed.Stderr)
	}
}

func TestPRAsJSONSaysAnOfferDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, _ := reviewRepo(t, reviewMoves)

	// Act
	// The open and the link are answered yes, the move no.
	printed, err := clitest.RunStreams(t, repo, clitest.Scripted([]string{"y", "y", "n"}, nil), "pr", "--json")
	// Assert
	if err != nil {
		t.Fatalf("pr --json: %v (%+v)", err, printed)
	}

	report := decodePR(t, printed.Stdout)
	if len(report.FollowUps) != 2 || !report.FollowUps[0].Done || report.FollowUps[1].Done {
		t.Errorf("follow_ups = %+v, want the link made and the move left", report.FollowUps)
	}
}

func TestPRAsJSONDryRunPrintsTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteRepoFile(t, repo, ".github/pull_request_template.md", "## Checklist\n")
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--json")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run --json: %v (%+v)", err, printed)
	}

	report := decodePR(t, printed.Stdout)
	if report.Pull.Number != 0 || report.Pull.Title == "" || report.Pull.Head != "fix/PROJ-2-thing" ||
		report.Pull.Base != "main" {
		t.Errorf("pull = %+v, want the unopened draft from fix/PROJ-2-thing into main", report.Pull)
	}

	if !strings.Contains(report.Pull.Body, "## Checklist") {
		t.Errorf("pull body = %q, want the body composed from the template", report.Pull.Body)
	}

	if !strings.Contains(printed.Stderr, "dry run: would push fix/PROJ-2-thing to origin and open") {
		t.Errorf("the dry-run line is not on stderr:\n%s", printed.Stderr)
	}
}

func TestPRAsJSONDeclinedPrintsNoReport(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, _ := reviewRepo(t, reviewMoves)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Scripted([]string{"n"}, nil), "pr", "--json")
	// Assert
	if err != nil {
		t.Fatalf("pr --json: %v (%+v)", err, printed)
	}

	if printed.Stdout != "" {
		t.Errorf("pr --json printed %q, want no report for a pull request never opened", printed.Stdout)
	}
}
