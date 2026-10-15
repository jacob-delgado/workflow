// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// status --plain draws the line's marks in ASCII for one run, as ui.ascii
// does for every run, whatever the configuration says.

import (
	"fmt"
	"strings"
	"testing"
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
			repo := featureRepo(t)
			dir, args := t.TempDir(), []string{statusCommand, "--plain", repo}

			if tt.inRepository {
				dir, args = repo, []string{statusCommand, "--plain"}
			}

			// Act
			printed, err := runStreams(t, dir, unusedPrompt(t), args...)

			// Assert
			if err != nil || !strings.Contains(printed.stdout, "# Branch") || strings.Contains(printed.stdout, "●") {
				t.Errorf("%v = %v, printed %q; want the marks in ASCII", args, err, printed.stdout)
			}
		})
	}
}

func TestStatusRefusesPlainBesideJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status", "--plain", "--json")

	// Assert
	const refusal = "--plain and --json cannot be used together: " +
		"drop --plain for JSON, or --json for the status line in ASCII"
	if !strings.HasSuffix(fmt.Sprint(err), ": "+refusal) || printed.stdout != "" {
		t.Errorf("status --plain --json = %v, printed %q; want it refused with %q", err, printed.stdout, refusal)
	}

	wantExit(t, err, 2)
}
