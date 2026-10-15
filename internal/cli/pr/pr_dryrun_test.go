// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestPRDryRunSaysWhatItWouldOfferOnceOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	for _, line := range []string{
		"dry run: would open work",
		"dry run: would link it on PROJ-2",
		"dry run: would move PROJ-2 to " + clitest.StatusInReview,
	} {
		if !strings.Contains(printed.Stderr, line) {
			t.Errorf("pr --dry-run does not say %q on stderr:\n%s", line, printed.Stderr)
		}
	}

	if kinds := writes.Kinds(); len(kinds) != 0 {
		t.Errorf("a dry run wrote %q to Jira, want nothing", kinds)
	}
}

func TestPRDryRunNamesNoOfferItWouldNotMake(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge's own issues are the tracker: nothing takes a link, and no
	// review status is configured.
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.Stderr, "would link") || strings.Contains(printed.Stderr, "would move") {
		t.Errorf("pr --dry-run names an offer it would not make:\n%s", printed.Stderr)
	}
}
