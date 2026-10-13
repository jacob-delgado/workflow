// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The flags the scriptable commands share are spelled short alike on every
// command that takes them, so a script learns -n, -y and -j once.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestEveryCommandSpellsTheSharedFlagsShortAlike(t *testing.T) {
	t.Parallel()

	// Arrange
	shorthands := map[string]string{"dry-run": "n", "yes": "y", "json": "j"}
	root := cli.NewRootCmd(cli.Prompt{})

	// Act
	commands := everyCommand(root)

	// Assert
	taking := make(map[string]int, len(shorthands))

	for _, cmd := range commands {
		// Asking for the inherited flags merges the root's into the command's
		// own, which panics on two flags spelled with one letter.
		cmd.InheritedFlags()

		for name, short := range shorthands {
			flag := cmd.Flag(name)
			if flag == nil {
				continue
			}

			taking[name]++

			if flag.Shorthand != short {
				t.Errorf("%s spells --%s short as %q, want -%s", cmd.CommandPath(), name, flag.Shorthand, short)
			}
		}
	}

	for name := range shorthands {
		if taking[name] == 0 {
			t.Errorf("no command takes --%s", name)
		}
	}
}

func TestDashNHoldsBackAWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "-n")

	// Assert
	if err != nil || !strings.Contains(printed.stderr, "dry run: would push fix/PROJ-2-thing and open") {
		t.Errorf("pr -n = %v, printed %+v; want the push and open held back", err, printed)
	}
}

func TestDashYGoesAheadWithoutAsking(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "-y")

	// Assert
	if err != nil || !strings.Contains(printed.stdout, "Opened #7") {
		t.Errorf("pr -y = %v, printed %+v; want the pull request opened unasked", err, printed)
	}
}

func TestDashJPrintsTheData(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status", "-j")

	// Assert
	var report struct {
		Stages []map[string]any `json:"stages"`
	}

	decodeErr := json.Unmarshal([]byte(printed.stdout), &report)
	if err != nil || decodeErr != nil || len(report.Stages) == 0 {
		t.Errorf("status -j = %v, printed %q (%v); want the status as JSON", err, printed.stdout, decodeErr)
	}
}
