// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The help speaks in one voice: every long help opens in the imperative, each
// summary claims only what its command does, and the static text names both
// forges' nouns, since it is drawn before any remote is read.

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// everyCommand is root and every command under it.
func everyCommand(root *cobra.Command) []*cobra.Command {
	children := root.Commands()
	commands := make([]*cobra.Command, 0, 1+len(children))
	commands = append(commands, root)

	for _, child := range children {
		commands = append(commands, everyCommand(child)...)
	}

	return commands
}

func TestEveryLongHelpOpensInTheImperative(t *testing.T) {
	// Arrange
	root := cli.NewRootCmd(cli.Prompt{})

	// Act
	commands := everyCommand(root)

	// Assert
	for _, cmd := range commands {
		words := strings.Fields(cmd.Long)
		if len(words) > 0 && strings.HasSuffix(words[0], "s") {
			t.Errorf("%s's long help opens %q, want an imperative", cmd.CommandPath(), words[0])
		}
	}
}

func TestConfigInitSummaryClaimsOnlyWhatItAsks(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "config", "--help")

	// Assert
	if err != nil || strings.Contains(output, "each credential") ||
		!strings.Contains(output, "asking for the Jira token and a Slack webhook") {
		t.Errorf("config --help = %v, want init's summary to name the Jira token and the webhook:\n%s", err, output)
	}
}

func TestTheLogFlagNamesItsFile(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "--help")

	// Assert
	if err != nil || !strings.Contains(output, "--log FILE") || strings.Contains(output, "--log string") {
		t.Errorf("--help = %v, want --log's placeholder named FILE:\n%s", err, output)
	}
}

func TestTheRootSummaryNamesBothInterfaces(t *testing.T) {
	// Arrange
	root := cli.NewRootCmd(cli.Prompt{})

	// Act
	opening, _, _ := strings.Cut(root.Long, ".")

	// Assert
	if strings.Contains(root.Short, "from the terminal") || !strings.Contains(root.Short, "a browser") {
		t.Errorf("the root's summary is %q, want it to name the terminal and a browser", root.Short)
	}

	if !strings.Contains(opening, "a browser") {
		t.Errorf("the root's long help opens %q, want it to name the browser too", opening)
	}
}

func TestTheReviewsSummaryNamesBothForgesNouns(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "--help")

	// Assert
	if err != nil || !strings.Contains(output, "List the pull or merge requests that are waiting on your review") {
		t.Errorf("--help = %v, want reviews' summary to say pull or merge requests:\n%s", err, output)
	}
}
