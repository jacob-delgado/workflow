// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestAnnounceRefusesAPullRequestACheckFailedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.FailingStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	if got := cli.ExitStatus(err); got != 4 {
		t.Fatalf("announce --yes exited %d (%v), want 4: a failed check refuses it", got, err)
	}

	if !strings.Contains(err.Error(), "a check failed on #7 (ci)") || printed.Stdout != "" ||
		strings.Contains(printed.Stderr, "Announced") {
		t.Errorf("announce said %v, printed %+v; want #7 and its failed check named, nothing previewed or announced",
			err, printed)
	}
}

func TestAnnounceNamesOnlyTheChecksThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// build passed and lint failed, so the refusal names lint alone.
	mixed := `{"total_count":2,"statuses":[` +
		`{"state":"success","context":"build","target_url":"https://x"},` +
		`{"state":"failure","context":"lint","target_url":"https://x"}]}`
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: mixed})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "a check failed on #7 (lint)") {
		t.Errorf("announce = %v, want lint, the check that failed, named alone", err)
	}
}

func TestAnnounceDryRunRefusesAFailedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.FailingStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--dry-run")

	// Assert
	if got := cli.ExitStatus(err); got != 4 || strings.Contains(printed.Stderr, "dry run: would announce") {
		t.Errorf("announce --dry-run exited %d (%v), said %q; want 4 and nothing said to be announced",
			got, err, printed.Stderr)
	}
}

func TestAnnounceAnnouncesWhileCIRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.RunningStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--dry-run")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "dry run: would announce") {
		t.Errorf("announce --dry-run = %v, said %q; want it to go ahead while nothing has failed", err, printed.Stderr)
	}
}

func TestAnnounceRefusesWhatItCannotConfirm(t *testing.T) {
	t.Parallel()

	// Arrange
	// An unreadable answer stands for a read of CI that failed.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: "{"})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "#7 was not announced") || printed.Stdout != "" {
		t.Errorf("announce --yes = %v, printed %+v; want #7 not announced, its CI unread, nothing previewed",
			err, printed)
	}
}

func TestAnnounceSaysToReRunTheChecksOfAMergeACheckFailedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	// A merge cannot wait for CI, so it is not pointed at --when-green.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.MergedPull("Add login"), Status: clitest.FailingStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	want := "a check failed on #7 (ci), so its merge was not announced; " +
		"re-run the failed checks on the forge, then announce it once they pass"
	if got := cli.ExitStatus(err); got != 4 || err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("announce --yes exited %d (%v), want 4 saying %q", got, err, want)
	}
}
