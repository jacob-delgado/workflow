// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// runStatus reads from a repository, a forge and Jira, none of which a
// black-box test can point at a fake through the wiring the command builds
// (the forge base is derived from the git remote, not injectable). So these
// drive runStatus directly with fake seams, which is what the seams are for.

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// errServiceDown is a forge or Jira that will not answer.
var errServiceDown = errors.New("service down")

// statusBase is the base ref the status fixtures branch off.
const statusBase = "origin/main"

// featureBranch is a branch for an issue, with a commit, and its pull request.
func featureBranch() statusSeams {
	return statusSeams{
		Branch: func() (gitrepo.Branch, error) {
			return gitrepo.Branch{
				Name: "fix/PROJ-7-login", Base: statusBase, Head: "abc123",
				Commits: make([]gitrepo.Commit, 1),
			}, nil
		},
		Changes:     func() ([]gitrepo.Change, error) { return nil, nil },
		FindPull:    func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{Number: 3}, true, nil },
		CheckStatus: func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{State: forge.CIRunning}, nil },
		Issue: func(string) (jira.IssueDetail, error) {
			return jira.IssueDetail{Issue: jira.Issue{Key: sampleKey, Summary: "Fix login"}}, nil
		},
	}
}

func TestStatusLineShowsTheIssueStagesAndCI(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runStatus(&out, featureBranch(), false, false)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	// Assert
	line := out.String()
	for _, want := range []string{"PROJ-7 Fix login", "● Branch", "● Commits", "◐ Review", "CI running"} {
		if !strings.Contains(line, want) {
			t.Errorf("status line missing %q:\n%s", want, line)
		}
	}
}

func TestStatusFailedCIReadsTheReviewFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := featureBranch()
	seams.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIFailed}, nil
	}

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, false)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	// Assert
	if line := out.String(); !strings.Contains(line, "✗ Review") || !strings.Contains(line, "CI failed") {
		t.Errorf("status line does not show the failure:\n%s", line)
	}
}

func TestStatusJSONReportsTheSameAsData(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := featureBranch()
	seams.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CIPassed}, nil
	}

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, true)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	// Assert
	var report struct {
		Issue   string           `json:"issue"`
		Summary string           `json:"summary"`
		CI      string           `json:"ci"`
		Stages  []map[string]any `json:"stages"`
	}

	err = json.Unmarshal(out.Bytes(), &report)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}

	if report.Issue != sampleKey || report.CI != "passed" || len(report.Stages) != 5 ||
		report.Stages[3]["state"] != "done" {
		t.Errorf("status JSON = %+v, want the issue, passed CI and five stages", report)
	}
}

func TestStatusDegradesWhenServicesDoNotAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// On the base branch, and neither the forge nor Jira will answer.
	seams := statusSeams{
		Branch:   func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: "main", Base: statusBase}, nil },
		Changes:  func() ([]gitrepo.Change, error) { return nil, nil },
		FindPull: func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errServiceDown },
		Issue:    func(string) (jira.IssueDetail, error) { return jira.IssueDetail{}, errServiceDown },
	}

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, false)
	// Assert
	if err != nil {
		t.Fatalf("runStatus should degrade, not fail: %v", err)
	}

	if line := out.String(); !strings.Contains(line, "○ Branch") || !strings.Contains(line, "CI none") {
		t.Errorf("status did not degrade to not-started:\n%s", line)
	}
}

func TestStatusDegradesEachServiceOnAFeatureBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// A feature branch for an issue, but every service refuses.
	seams := statusSeams{
		Branch:   func() (gitrepo.Branch, error) { return gitrepo.Branch{Name: "fix/PROJ-9-x", Base: statusBase}, nil },
		Changes:  func() ([]gitrepo.Change, error) { return nil, errServiceDown },
		FindPull: func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errServiceDown },
		Issue:    func(string) (jira.IssueDetail, error) { return jira.IssueDetail{}, errServiceDown },
	}

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, false)
	// Assert
	if err != nil {
		t.Fatalf("status should degrade, not fail: %v", err)
	}

	line := out.String()
	if !strings.Contains(line, "PROJ-9") || !strings.Contains(line, "○ Review") || !strings.Contains(line, "CI none") {
		t.Errorf("status did not degrade cleanly:\n%s", line)
	}
}

func TestStatusWhenCICannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := featureBranch()
	seams.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{}, errServiceDown }

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, false)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	// Assert
	if line := out.String(); !strings.Contains(line, "CI none") {
		t.Errorf("an unreadable CI should read none:\n%s", line)
	}
}

func TestStatusInASCII(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runStatus(&out, featureBranch(), true, false)
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	// Assert
	if line := out.String(); !strings.Contains(line, "# Branch") {
		t.Errorf("ASCII status should use plain marks:\n%s", line)
	}
}

func TestStatusJSONNamesEachState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ci   forge.CIState
		word string
	}{
		"running": {ci: forge.CIRunning, word: "in_flight"},
		"failed":  {ci: forge.CIFailed, word: "failed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := featureBranch()
			seams.CheckStatus = func(forge.PullRequest, string) (forge.CI, error) { return forge.CI{State: tt.ci}, nil }

			var out bytes.Buffer

			// Act
			err := runStatus(&out, seams, false, true)
			if err != nil {
				t.Fatalf("runStatus: %v", err)
			}

			// Assert
			if !strings.Contains(out.String(), `"state": "`+tt.word+`"`) {
				t.Errorf("JSON does not name the review state %q:\n%s", tt.word, out.String())
			}
		})
	}
}

func TestStatusReportsAnUnreadableBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := statusSeams{Branch: func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errServiceDown }}

	var out bytes.Buffer

	// Act
	err := runStatus(&out, seams, false, false)

	// Assert
	if !errors.Is(err, errServiceDown) {
		t.Errorf("runStatus returned %v, want the branch read failure", err)
	}
}
