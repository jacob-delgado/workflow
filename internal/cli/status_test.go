// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStatusOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "status")

	// Assert
	if err == nil {
		t.Error("status outside a repository returned no error")
	}
}

// featureRepo makes a repository on a branch for PROJ-2 with a commit, and
// returns its path.
func featureRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "init")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "work")

	return repo
}

func TestStatusHereReportsTheCurrentRepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "CI none") {
		t.Errorf("expected the current repository's status:\n%s", output)
	}
}

func TestStatusAcrossLabelsTheCurrentDirectory(t *testing.T) {
	// Act
	// "." names the working directory, which is no repository here.
	output, err := run(t, t.TempDir(), "status", ".")
	if err != nil {
		t.Fatalf("status .: %v (%s)", err, output)
	}

	// Assert
	if !strings.HasPrefix(output, ".  ") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected a labeled line for the current directory:\n%s", output)
	}
}

func TestStatusAcrossReportsEachRepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", repo, notRepo)
	if err != nil {
		t.Fatalf("status across directories: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected the repository's issue and a note for the non-repository:\n%s", output)
	}
}

func TestStatusAcrossAsJSONIsAnArray(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", "--json", repo, notRepo)
	if err != nil {
		t.Fatalf("status --json across directories: %v (%s)", err, output)
	}

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Issue      string `json:"issue"`
		Error      string `json:"error"`
	}

	err = json.Unmarshal([]byte(output), &reports)
	if err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, output)
	}

	if len(reports) != 2 || reports[0].Issue != "PROJ-2" || reports[1].Error == "" {
		t.Errorf("status JSON = %+v, want the repository and the non-repository's error", reports)
	}
}

// runningStatus is a commit status still to finish.
func runningStatus() string {
	return `{"total_count":1,"statuses":[{"state":"pending","context":"ci","target_url":"https://x"}]}`
}

// passingStatus is a commit status that has passed.
func passingStatus() string {
	return `{"total_count":1,"statuses":[{"state":"success","context":"ci","target_url":"https://x"}]}`
}

// statusFeatureRepo is a repository on a feature branch for PROJ-7, one commit
// ahead of main, with a GitHub remote so the forge resolves through the fake gh,
// and a configuration naming the Jira at jiraURL.
func statusFeatureRepo(t *testing.T, jiraURL string) string {
	t.Helper()

	repo := githubRepo(t, "fix/PROJ-7-login")
	writeFile(t, repo, `{"jira":{"base_url":"`+jiraURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

func TestStatusLineShowsTheIssueStagesAndCI(t *testing.T) {
	// Arrange
	// A feature branch with an open pull request whose CI is still running.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	for _, want := range []string{"PROJ-7 Fix login", "● Branch", "● Commits", "◐ Review", "CI running"} {
		if !strings.Contains(output, want) {
			t.Errorf("status line missing %q:\n%s", want, output)
		}
	}
}

func TestStatusFailedCIReadsTheReviewFailed(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "✗ Review") || !strings.Contains(output, "CI failed") {
		t.Errorf("status line does not show the failure:\n%s", output)
	}
}

func TestStatusJSONReportsTheSameAsData(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := run(t, repo, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (%s)", err, output)
	}

	// Assert
	var report struct {
		Issue  string           `json:"issue"`
		CI     string           `json:"ci"`
		Stages []map[string]any `json:"stages"`
	}

	err = json.Unmarshal([]byte(output), &report)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output)
	}

	if report.Issue != "PROJ-7" || report.CI != "passed" || len(report.Stages) != 5 ||
		report.Stages[3]["state"] != "done" {
		t.Errorf("status JSON = %+v, want the issue, passed CI and five stages", report)
	}
}

func TestStatusJSONNamesEachReviewState(t *testing.T) {
	cases := map[string]struct {
		status string
		word   string
	}{
		"running": {status: runningStatus(), word: "in_flight"},
		"failed":  {status: failingStatus(), word: "failed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			fakeGh(t, ghResponses{pulls: openPull("Add login"), status: tt.status})
			repo := statusFeatureRepo(t, server.URL)

			// Act
			output, err := run(t, repo, "status", "--json")
			if err != nil {
				t.Fatalf("status --json: %v (%s)", err, output)
			}

			// Assert
			if !strings.Contains(output, `"state": "`+tt.word+`"`) {
				t.Errorf("JSON does not name the review state %q:\n%s", tt.word, output)
			}
		})
	}
}

func TestStatusWhenCICannotBeRead(t *testing.T) {
	// Arrange
	// The pull request is open, but its CI cannot be read.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: "{"})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "CI none") {
		t.Errorf("an unreadable CI should read none:\n%s", output)
	}
}

func TestStatusInASCII(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := githubRepo(t, "fix/PROJ-7-login")
	writeFile(t, repo, `{"ui":{"ascii":true},"jira":{"base_url":"`+server.URL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "# Branch") {
		t.Errorf("ASCII status should use plain marks:\n%s", output)
	}
}

func TestStatusOnTheBaseBranchIsNotStarted(t *testing.T) {
	// Arrange
	// A repository sitting on its base branch, with no issue and no work begun.
	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "init")
	git(t, repo, "branch", "-M", "main")

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "○ Branch") || !strings.Contains(output, "CI none") {
		t.Errorf("status on the base branch did not read as not-started:\n%s", output)
	}
}

func TestStatusDegradesEachServiceOnAFeatureBranch(t *testing.T) {
	// Arrange
	// A feature branch for an issue, but neither Jira nor the forge is configured
	// to answer, so each service's stage degrades rather than failing the line.
	repo := prRepo(t, "fix/PROJ-9-x")

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status should degrade, not fail: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-9") || !strings.Contains(output, "○ Review") ||
		!strings.Contains(output, "CI none") {
		t.Errorf("status did not degrade cleanly:\n%s", output)
	}
}
