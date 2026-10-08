// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAnnounceYesDryRunPreviewsAMomentItWouldLeaveAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t),
		"announce", "--yes", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "opened a pull request") {
		t.Errorf("stdout does not preview the announcement:\n%s", printed.stdout)
	}

	if !strings.Contains(printed.stderr, alreadyAnnounced) ||
		!strings.Contains(printed.stderr, "dry run: would not announce it again") ||
		strings.Contains(printed.stderr, "run without --yes") {
		t.Errorf("stderr does not say that --yes would leave the announced moment as it is:\n%s", printed.stderr)
	}
}
