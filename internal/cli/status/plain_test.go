// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

// status --plain draws the line's marks in ASCII for one run, as ui.ascii
// does for every run, whatever the configuration says.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestStatusPlainDrawsTheMarksInASCII(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		inRepository bool
	}{
		"the repository it runs in":        {inRepository: true},
		"a repository it is given by name": {inRepository: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Neither configuration sets ui.ascii, so only --plain asks for it.
			repo := clitest.FeatureRepo(t)
			dir, args := t.TempDir(), []string{statusCommand, "--plain", repo}

			if tt.inRepository {
				dir, args = repo, []string{statusCommand, "--plain"}
			}

			// Act
			printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), args...)

			// Assert
			if err != nil || !strings.Contains(printed.Stdout, "# Branch") || strings.Contains(printed.Stdout, "●") {
				t.Errorf("%v = %v, printed %q; want the marks in ASCII", args, err, printed.Stdout)
			}
		})
	}
}

func TestStatusRefusesPlainBesideJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := clitest.FeatureRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status", "--plain", "--json")

	// Assert
	said := fmt.Sprint(err)
	if !strings.Contains(said, "--plain") || !strings.Contains(said, "--json") || printed.Stdout != "" {
		t.Errorf("status --plain --json = %v, printed %q; want it refused, naming both flags", err, printed.Stdout)
	}

	// A JSON run of named directories still keeps a live line, which ui.ascii
	// alone draws in ASCII.
	if !strings.Contains(said, "ui.ascii") {
		t.Errorf("status --plain --json = %v; want it to name ui.ascii for a JSON run's live line", err)
	}

	wantExit(t, err, 2)
}
