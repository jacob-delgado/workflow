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
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

func TestEveryCommandSpellsTheSharedFlagsShortAlike(t *testing.T) {
	t.Parallel()

	// Arrange
	shorthands := map[string]string{"dry-run": "n", "yes": "y", "json": "j"}
	root := cli.NewRootCmd(cmdkit.Prompt{})

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
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "-n")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "dry run: would push fix/PROJ-2-thing to origin and open") {
		t.Errorf("pr -n = %v, printed %+v; want the push and open held back", err, printed)
	}
}

func TestDashYGoesAheadWithoutAsking(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "-y")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "Opened #7") {
		t.Errorf("pr -y = %v, printed %+v; want the pull request opened unasked", err, printed)
	}
}

func TestDashJPrintsTheData(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status", "-j")

	// Assert
	var report struct {
		Stages []map[string]any `json:"stages"`
	}

	decodeErr := json.Unmarshal([]byte(printed.Stdout), &report)
	if err != nil || decodeErr != nil || len(report.Stages) == 0 {
		t.Errorf("status -j = %v, printed %q (%v); want the status as JSON", err, printed.Stdout, decodeErr)
	}
}
