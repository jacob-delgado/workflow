// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// openingThenOpen is a forge with no pull request for the branch until one is
// opened, which every later find answers, its CI running and then passed.
func openingThenOpen() ghResponses {
	return ghResponses{
		pullsSequence:  []string{"[]", openPull("work")},
		statusSequence: []string{runningStatus(), passingStatus()},
	}
}

func TestPRAnnounceOpensThenAnnouncesOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	fakeGh(t, openingThenOpen())
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stdout, "Opened #7") {
		t.Errorf("pr --announce = %v, posted %d, printed %+v; want it opened, then announced once",
			err, posts.Load(), printed)
	}

	if strings.Contains(printed.Stdout, "opened a pull request") {
		t.Errorf("pr --announce previewed the announcement on stdout, the pull request's stream:\n%s", printed.Stdout)
	}
}

func TestPRAnnounceWithoutMessagingOpensNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	sent := filepath.Join(t.TempDir(), "sent.log")
	responses := openingThenOpen()
	responses.sentLog = sent
	fakeGh(t, responses)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if got := cli.ExitStatus(err); got != 3 {
		t.Errorf("pr --announce without messaging exited %d (%v), want 3", got, err)
	}

	if log, _ := os.ReadFile(sent); strings.Contains(string(log), "/pulls") {
		t.Errorf("pr --announce without messaging opened a pull request:\n%s", log)
	}
}

func TestPRRefusesAnnounceBesideDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	sent := filepath.Join(t.TempDir(), "sent.log")
	responses := openingThenOpen()
	responses.sentLog = sent
	fakeGh(t, responses)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, new(atomic.Int32))))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--draft", "--announce")

	// Assert
	if said := fmt.Sprint(err); !strings.Contains(said, "--announce") || !strings.Contains(said, "--draft") {
		t.Errorf("pr --draft --announce = %v, want it refused, naming both flags", err)
	}

	if log, _ := os.ReadFile(sent); strings.Contains(string(log), "/pulls") {
		t.Errorf("pr --draft --announce opened a pull request:\n%s", log)
	}

	wantExit(t, err, 2)
}

func TestPRAnnounceDryRunSaysItWouldAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, openingThenOpen())
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, new(atomic.Int32))))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--announce")

	// Assert
	want := "dry run: would announce it to the channel its webhook is bound to once its CI passes"
	if err != nil || !strings.Contains(printed.Stderr, want) {
		t.Errorf("pr --dry-run --announce = %v, said %q; want %q", err, printed.Stderr, want)
	}
}

func TestPRAnnounceJSONPrintsTheReportBeforeWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	fakeGh(t, openingThenOpen())
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--json", "--announce")

	// Assert
	var report struct {
		Pull struct {
			Number int `json:"number"`
		} `json:"pull"`
	}

	decodeErr := json.Unmarshal([]byte(printed.Stdout), &report)
	if err != nil || decodeErr != nil || report.Pull.Number != 7 || posts.Load() != 1 {
		t.Errorf("pr --json --announce = %v, stdout %q (%v), posted %d; want the report, then one post",
			err, printed.Stdout, decodeErr, posts.Load())
	}
}

func TestPRWithoutAnnounceNamesWhenGreen(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig("https://hooks.slack.example/services/x"))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")

	// Assert
	want := "Announce it with workflow announce, or once CI passes with workflow announce --when-green."
	if err != nil || !strings.Contains(printed.Stderr, want) {
		t.Errorf("pr --yes = %v, said %q; want %q", err, printed.Stderr, want)
	}
}

func TestPRAnnounceWaitsForAPullRequestTheForgeHasNotListedYet(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge lists the pull request it opened only from the second read
	// after.
	var posts atomic.Int32

	responses := openingThenOpen()
	responses.pullsSequence = []string{"[]", "[]", openPull("work")}
	fakeGh(t, responses)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Errorf("pr --announce = %v, posted %d, said %q; want it announced once the forge lists it",
			err, posts.Load(), printed.Stderr)
	}
}
