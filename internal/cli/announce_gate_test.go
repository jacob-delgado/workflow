// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestAnnounceRefusesAPullRequestACheckFailedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--yes")

	// Assert
	if got := cli.ExitStatus(err); got != 4 {
		t.Fatalf("announce --yes exited %d (%v), want 4: a failed check refuses it", got, err)
	}

	if !strings.Contains(err.Error(), "a check failed on #7 (ci)") || printed.stdout != "" ||
		strings.Contains(printed.stderr, "Announced") {
		t.Errorf("announce said %v, printed %+v; want #7 and its failed check named, nothing previewed or announced",
			err, printed)
	}
}

func TestAnnounceDryRunRefusesAFailedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--dry-run")

	// Assert
	if got := cli.ExitStatus(err); got != 4 || strings.Contains(printed.stderr, "dry run: would announce") {
		t.Errorf("announce --dry-run exited %d (%v), said %q; want 4 and nothing said to be announced",
			got, err, printed.stderr)
	}
}

func TestAnnounceAnnouncesWhileCIRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--dry-run")

	// Assert
	if err != nil || !strings.Contains(printed.stderr, "dry run: would announce") {
		t.Errorf("announce --dry-run = %v, said %q; want it to go ahead while nothing has failed", err, printed.stderr)
	}
}

func TestAnnounceRefusesWhatItCannotConfirm(t *testing.T) {
	t.Parallel()

	// Arrange
	// An unreadable answer stands for a read of CI that failed.
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: "{"})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "#7 was not announced") || printed.stdout != "" {
		t.Errorf("announce --yes = %v, printed %+v; want #7 not announced, its CI unread, nothing previewed",
			err, printed)
	}
}
