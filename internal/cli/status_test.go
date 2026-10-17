// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// stageDone is the JSON word for a stage that is complete.
const stageDone = "done"

func TestStatusOutsideARepositoryReportsSo(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.Run(t, t.TempDir(), "status")

	// Assert
	if err == nil {
		t.Error("status outside a repository returned no error")
	}

	wantExit(t, err, 4)
}

func TestStatusWithoutGitNamesTheMissingProgram(t *testing.T) {
	t.Parallel()

	// Arrange
	// An empty PATH is the portable way to make git unfindable.
	clitest.SetVariable(t, "PATH", "")

	// Act
	_, err := clitest.Run(t, t.TempDir(), "status")

	// Assert
	// The error is what main prints, so it is what names the program.
	if said := fmt.Sprint(err); !strings.Contains(said, "not found on PATH: git") ||
		strings.Contains(said, "not a git repository") {
		t.Errorf("status without git said %q, want the missing program named", said)
	}

	wantExit(t, err, 1)
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
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "CI none") {
		t.Errorf("expected the current repository's status:\n%s", output)
	}
}

func TestStatusExitsAlikeOutsideARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	_, bareErr := clitest.Run(t, dir, "status")

	// Act
	// "." names the working directory, which is no repository here.
	_, err := clitest.Run(t, dir, "status", ".")

	// Assert
	if got, bare := cli.ExitStatus(err), cli.ExitStatus(bareErr); got != bare || got != 4 {
		t.Errorf("outside a repository status . exits %d and status %d, want both 4", got, bare)
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
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+jiraURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

func TestStatusLineShowsTheIssueStagesAndCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// A feature branch with an open pull request whose CI is still running.
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := clitest.Run(t, repo, "status")
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
	t.Parallel()

	// Arrange
	repo := featureRepo(t)
	clitest.WriteFile(t, repo, `{"messaging":{"kind":"teams","webhook_url":"https://outlook.office.example/webhook/x"}}`)

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "○ Teams") || strings.Contains(output, "Slack") {
		t.Errorf("status line does not name the stage for Teams:\n%s", output)
	}
}

func TestStatusFailedCIReadsTheReviewFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "✗ Review") || !strings.Contains(output, "CI failed") {
		t.Errorf("status line does not show the failure:\n%s", output)
	}
}

func TestStatusJSONReportsTheSameAsData(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	output, err := clitest.Run(t, repo, "status", "--json")
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
		report.Stages[3]["state"] != stageDone {
		t.Errorf("status JSON = %+v, want the issue, passed CI and five stages", report)
	}
}

func TestStatusLineReadsTheAnnouncementAtTheCurrentMoment(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status     string
		remembered messaging.Moment
		mark       string
	}{
		"ready, announced ready":        {status: runningStatus(), remembered: messaging.MomentReady, mark: "● Slack"},
		"ready, announced only merged":  {status: runningStatus(), remembered: messaging.MomentMerged, mark: "○ Slack"},
		"CI red, announced ready":       {status: failingStatus(), remembered: messaging.MomentReady, mark: "● Slack"},
		"CI red, announced only merged": {status: failingStatus(), remembered: messaging.MomentMerged, mark: "○ Slack"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The store remembers pull request 7 announced at one moment, as an
			// earlier announce, or session of the interface, leaves it.
			server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			fakeGh(t, ghResponses{pulls: openPull("Add login"), status: tt.status})
			repo := statusFeatureRepo(t, server.URL)
			home := announcedEarlier(t, tt.remembered)

			// Act
			printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "status")
			if err != nil {
				t.Fatalf("status: %v (%+v)", err, printed)
			}

			// Assert
			if !strings.Contains(printed.Stdout, tt.mark) {
				t.Errorf("status line missing %q:\n%s", tt.mark, printed.Stdout)
			}
		})
	}
}

func TestStatusJSONReadsAnEarlierAnnouncementAsDone(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := statusFeatureRepo(t, server.URL)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "status", "--json",
	)
	if err != nil {
		t.Fatalf("status --json: %v (%+v)", err, printed)
	}

	// Assert
	var report struct {
		Stages []map[string]any `json:"stages"`
	}

	err = json.Unmarshal([]byte(printed.Stdout), &report)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, printed.Stdout)
	}

	if len(report.Stages) != 5 || report.Stages[4]["name"] != "Slack" || report.Stages[4]["state"] != stageDone {
		t.Errorf("status JSON stages = %+v, want the Slack stage done", report.Stages)
	}
}

func TestStatusJSONNamesEachReviewState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status string
		word   string
	}{
		"running": {status: runningStatus(), word: "in_flight"},
		"failed":  {status: failingStatus(), word: "failed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			fakeGh(t, ghResponses{pulls: openPull("Add login"), status: tt.status})
			repo := statusFeatureRepo(t, server.URL)

			// Act
			output, err := clitest.Run(t, repo, "status", "--json")
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
	t.Parallel()

	// Arrange
	// The pull request is open, but its CI cannot be read.
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: "{"})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")
	if err != nil {
		t.Fatalf("status: %v (%+v)", err, printed)
	}

	// Assert
	// The line still reads none, so a prompt keeps its shape, but the forge
	// that would not say is named where a person or a script can see it.
	if !strings.Contains(printed.Stdout, "CI none") || strings.Contains(printed.Stdout, "GitHub") {
		t.Errorf("stdout = %q, want the line with CI none and nothing more", printed.Stdout)
	}

	if !strings.Contains(printed.Stderr, "GitHub could not be read") {
		t.Errorf("stderr = %q, want a note naming GitHub", printed.Stderr)
	}
}

func TestStatusInASCII(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := githubRepo(t, "fix/PROJ-7-login")
	clitest.WriteFile(t, repo, `{"ui":{"ascii":true},"jira":{"base_url":"`+server.URL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "# Branch") {
		t.Errorf("ASCII status should use plain marks:\n%s", output)
	}
}

func TestStatusOnTheBaseBranchIsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository sitting on its base branch, with no issue and no work begun.
	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "init")
	git(t, repo, "branch", "-M", "main")

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "○ Branch") || !strings.Contains(output, "CI none") {
		t.Errorf("status on the base branch did not read as not-started:\n%s", output)
	}
}

func TestStatusDegradesEachServiceOnAFeatureBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// A feature branch for an issue, but neither Jira nor the forge is configured
	// to answer, so each service's stage degrades rather than failing the line.
	repo := prRepo(t, "fix/PROJ-9-x")

	// Act
	output, err := clitest.Run(t, repo, "status")
	if err != nil {
		t.Fatalf("status should degrade, not fail: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-9") || !strings.Contains(output, "○ Review") ||
		!strings.Contains(output, "CI none") {
		t.Errorf("status did not degrade cleanly:\n%s", output)
	}
}
