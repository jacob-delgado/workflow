// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/status"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// stageDone is the JSON word for a stage that is complete.
const stageDone = "done"

// statusCommand is the command these tests run, where it is named apart from
// a whole command line.
const statusCommand = "status"

func TestNewCmdIsStatusWithItsFlags(t *testing.T) {
	t.Parallel()

	// Act
	cmd := status.NewCmd(clitest.UnusedPrompt(t))

	// Assert
	if cmd.Name() != statusCommand {
		t.Errorf("NewCmd names %q, want status", cmd.Name())
	}

	for _, flag := range []string{"json", "plain"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("status declares no --%s", flag)
		}
	}
}

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

func TestStatusHereReportsTheCurrentRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := clitest.FeatureRepo(t)

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

func TestStatusLineShowsTheIssueStagesAndCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// A feature branch with an open pull request whose CI is still running.
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.RunningStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)

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
	repo := clitest.FeatureRepo(t)
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
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.FailingStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)

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
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)

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

	running, failing := clitest.RunningStatus(), clitest.FailingStatus()
	cases := map[string]struct {
		status     string
		remembered messaging.Moment
		mark       string
	}{
		"ready, announced ready":        {status: running, remembered: messaging.MomentReady, mark: "● Slack"},
		"ready, announced only merged":  {status: running, remembered: messaging.MomentMerged, mark: "○ Slack"},
		"CI red, announced ready":       {status: failing, remembered: messaging.MomentReady, mark: "● Slack"},
		"CI red, announced only merged": {status: failing, remembered: messaging.MomentMerged, mark: "○ Slack"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The store remembers pull request 7 announced at one moment, as an
			// earlier announce, or session of the interface, leaves it.
			server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: tt.status})
			repo := clitest.JiraFeatureRepo(t, server.URL)
			home := clitest.AnnouncedEarlier(t, tt.remembered)

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
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.RunningStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

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
		"running": {status: clitest.RunningStatus(), word: "in_flight"},
		"failed":  {status: clitest.FailingStatus(), word: "failed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: tt.status})
			repo := clitest.JiraFeatureRepo(t, server.URL)

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
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: "{"})
	repo := clitest.JiraFeatureRepo(t, server.URL)

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
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-7-login")
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

func TestStatusOnADetachedHeadIsNoFeatureBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// Commits made on a feature branch, then left for the commit itself: no
	// branch is checked out to name an issue or a pull request.
	repo := clitest.FeatureRepo(t)
	clitest.Git(t, repo, "checkout", "--quiet", "--detach")

	// Act
	output, err := clitest.Run(t, repo, statusCommand)
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.HasPrefix(output, "○ Issue  ○ Branch  ○ Commits") || strings.Contains(output, "PROJ-2") {
		t.Errorf("status on a detached HEAD should read as no feature branch begun:\n%s", output)
	}
}

func TestStatusOnTheBaseBranchIsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository sitting on its base branch, with no issue and no work begun.
	repo := t.TempDir()
	clitest.GitInit(t, repo)
	clitest.Commit(t, repo, "init")
	clitest.Git(t, repo, "branch", "-M", "main")

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
	repo := clitest.PRRepo(t, "fix/PROJ-9-x")

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
