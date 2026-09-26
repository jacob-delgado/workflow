// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

func TestPRDryRunSaysWhatItWouldOfferOnceOpen(t *testing.T) {
	// Arrange
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	for _, line := range []string{
		"dry run: would open work",
		"dry run: would link it on PROJ-2",
		"dry run: would move PROJ-2 to " + statusInReview,
	} {
		if !strings.Contains(printed.stderr, line) {
			t.Errorf("pr --dry-run does not say %q on stderr:\n%s", line, printed.stderr)
		}
	}

	if kinds := writes.kinds(); len(kinds) != 0 {
		t.Errorf("a dry run wrote %q to Jira, want nothing", kinds)
	}
}

func TestPRDryRunNamesNoOfferItWouldNotMake(t *testing.T) {
	// Arrange
	// The forge's own issues are the tracker: nothing takes a link, and no
	// review status is configured.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.stderr, "would link") || strings.Contains(printed.stderr, "would move") {
		t.Errorf("pr --dry-run names an offer it would not make:\n%s", printed.stderr)
	}
}
