// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// scriptingPage is the page that says what a script can rely on from each
// command; the usage page links to it rather than listing the commands again.
const scriptingPage = "../../docs/content/docs/scripting.md"

// The scripting page's opening paragraph names every command, and the table
// under this header says what each prints on which stream.
const (
	scriptingOpens = "# Scripting"
	streamsHeader  = "| Command | stdout | stderr |"
)

func TestTheScriptingPageOpensByNamingEveryCommand(t *testing.T) {
	t.Parallel()

	// Arrange
	commands := topLevelCommands(cli.NewRootCmd(cli.Prompt{}))

	// Act
	named := firstWords(backtickedIn(openingParagraph(t)))

	// Assert
	for _, command := range commands {
		if !slices.Contains(named, command) {
			t.Errorf("%s's opening paragraph does not name `%s`", scriptingPage, command)
		}
	}
}

func TestTheScriptingPageSaysWhereEachCommandPrints(t *testing.T) {
	t.Parallel()

	// Arrange
	commands := pathsBelowRoot(leafCommands(cli.NewRootCmd(cli.Prompt{})))

	// Act
	rows := streamsTableCommands(t)

	// Assert
	for _, command := range commands {
		if !slices.Contains(rows, command) {
			t.Errorf("%s's table headed %q has no row for `%s`", scriptingPage, streamsHeader, command)
		}
	}
}

// topLevelCommands is the name of each subcommand of root a user runs: any
// hidden command, and cobra's own, left out.
func topLevelCommands(root *cobra.Command) []string {
	names := make([]string, 0, len(root.Commands()))

	for _, command := range root.Commands() {
		if !command.Hidden && !cobraOwn(command.Name()) {
			names = append(names, command.Name())
		}
	}

	return names
}

// pathsBelowRoot is each command path without the root's name, cobra's own
// commands left out: `workflow config show` is `config show`.
func pathsBelowRoot(paths []string) []string {
	below := make([]string, 0, len(paths))

	for _, path := range paths {
		_, rest, _ := strings.Cut(path, " ")
		if name, _, _ := strings.Cut(rest, " "); !cobraOwn(name) {
			below = append(below, rest)
		}
	}

	return below
}

// cobraOwn reports one of the commands cobra adds, help and completion, which
// the scripting page leaves to the command reference.
func cobraOwn(name string) bool {
	return name == "help" || name == "completion"
}

// scriptingContents is the scripting page.
func scriptingContents(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile(scriptingPage)
	if err != nil {
		t.Fatalf("read %s: %v", scriptingPage, err)
	}

	return string(contents)
}

// openingParagraph is the scripting page's first paragraph under its title.
func openingParagraph(t *testing.T) string {
	t.Helper()

	_, body, found := strings.Cut(scriptingContents(t), scriptingOpens+"\n\n")
	if !found {
		t.Fatalf("%s does not open with %q", scriptingPage, scriptingOpens)
	}

	paragraph, _, _ := strings.Cut(body, "\n\n")

	return paragraph
}

// streamsTableCommands is the command each row of the streams table names in
// its first cell.
func streamsTableCommands(t *testing.T) []string {
	t.Helper()

	_, table, found := strings.Cut(scriptingContents(t), streamsHeader+"\n")
	if !found {
		t.Fatalf("%s has no table headed %q", scriptingPage, streamsHeader)
	}

	var commands []string

	for line := range strings.Lines(table) {
		cells := strings.Split(line, "|")
		if !strings.HasPrefix(line, "|") || len(cells) < 3 {
			break
		}

		commands = append(commands, backtickedIn(cells[1])...)
	}

	return commands
}

// backtickedIn is every backticked name in text, its white space folded.
func backtickedIn(text string) []string {
	var names []string

	for index, part := range strings.Split(text, "`") {
		if index%2 == 1 {
			names = append(names, strings.Join(strings.Fields(part), " "))
		}
	}

	return names
}

// firstWords is the first word of each name: `slack login` names slack.
func firstWords(names []string) []string {
	words := make([]string, 0, len(names))

	for _, name := range names {
		first, _, _ := strings.Cut(name, " ")
		words = append(words, first)
	}

	return words
}
