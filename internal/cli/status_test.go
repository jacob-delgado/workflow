// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestStatusOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "status")

	// Assert
	if err == nil {
		t.Error("status outside a repository returned no error")
	}

	wantExit(t, err, 4)
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

func TestStatusExitsAlikeOutsideARepository(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	_, bareErr := run(t, dir, "status")

	// Act
	// "." names the working directory, which is no repository here.
	_, err := run(t, dir, "status", ".")

	// Assert
	if got, bare := cli.ExitStatus(err), cli.ExitStatus(bareErr); got != bare || got != 4 {
		t.Errorf("outside a repository status . exits %d and status %d, want both 4", got, bare)
	}
}

func TestStatusAcrossLabelsTheCurrentDirectory(t *testing.T) {
	// Act
	// "." names the working directory, which is no repository here.
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", ".")

	// Assert
	if !strings.HasPrefix(printed.stdout, ".  ") || !strings.Contains(printed.stdout, "not a git repository") {
		t.Errorf("expected a labeled line for the current directory:\n%s", printed.stdout)
	}

	wantExit(t, err, 4)
}

func TestStatusAcrossReportsEachRepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", notRepo, repo)

	// Assert
	// Every directory gets its line, even after one that cannot be read, and
	// the one that is no repository still fails the command, as bare status
	// fails outside one.
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected the repository's issue and a note for the non-repository:\n%s", output)
	}

	wantExit(t, err, 4)
}

func TestStatusAcrossPassesWhenEveryDirectoryIsARepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if err != nil || !strings.Contains(output, "PROJ-2") {
		t.Errorf("status across a repository = %v, want its line and success:\n%s", err, output)
	}
}

func TestStatusAcrossSaysWhyARepositoryCannotBeRead(t *testing.T) {
	// Arrange
	// A repository whose branch name holds a character with no width: it is a
	// repository, but its branch cannot be shown as it is, so it is not read.
	repo := featureRepo(t)
	git(t, repo, "checkout", "-b", "fix/PROJ\u200b-3")

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if strings.Contains(output, "not a git repository") || !strings.Contains(output, "cannot be shown") {
		t.Errorf("status called a repository it could not read no repository:\n%s", output)
	}

	wantExit(t, err, 1)
}

func TestStatusAcrossRefusesADirectoryWhoseConfigurationCannotBeRead(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	writeFile(t, repo, "{not json")

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if strings.Contains(output, "PROJ-2") || !strings.Contains(output, "invalid") {
		t.Errorf("status read a repository whose configuration it could not:\n%s", output)
	}

	wantExit(t, err, 3)
}

func TestStatusAcrossAsJSONIsAnArray(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", "--json", notRepo, repo)

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Issue      string `json:"issue"`
		Error      string `json:"error"`
	}

	decodeErr := json.Unmarshal([]byte(printed.stdout), &reports)
	if decodeErr != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", decodeErr, printed.stdout)
	}

	if len(reports) != 2 || reports[0].Error != "not a git repository" || reports[1].Issue != "PROJ-2" {
		t.Errorf("status JSON = %+v, want the repository and the non-repository's error", reports)
	}

	wantExit(t, err, 4)
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

func TestStatusNamesTheLastStageForTheMessagingServiceInUse(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	writeFile(t, repo, `{"messaging":{"kind":"teams","webhook_url":"https://outlook.office.example/webhook/x"}}`)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "○ Teams") || strings.Contains(output, "Slack") {
		t.Errorf("status line does not name the stage for Teams:\n%s", output)
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
