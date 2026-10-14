// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/console"
)

// errRefused is a failure a command reports.
var errRefused = errors.New("the Jira token was refused (401); run workflow doctor --online")

// mistyped is a usage error, in cobra's words and with its hint after a
// blank line.
type mistyped struct{}

var _ error = mistyped{}

// Error is the refusal, then the hint.
func (mistyped) Error() string {
	return "unknown command \"launch\" for \"workflow\"\n\nRun 'workflow --help' for usage."
}

// hueParameter matches a select graphic rendition that sets one of the
// sixteen ANSI foreground hues, alone or beside a weight.
var hueParameter = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*(?:3[0-7]|9[0-7])(?:;[0-9]+)*m`)

// forcedColor is an environment that asks for color on any stream.
func forcedColor() map[string]string {
	return map[string]string{"CLICOLOR_FORCE": "1", "TERM": "xterm-256color"}
}

// variables is an environment holding set and nothing else.
func variables(set map[string]string) func(string) string {
	return func(name string) string { return set[name] }
}

// saying is what notes over a pipe, in the environment env, print for what
// says.
func saying(env map[string]string, says func(console.Notes)) string {
	var stderr bytes.Buffer

	says(console.New(&stderr, console.Options{Getenv: variables(env)}))

	return stderr.String()
}

func TestANoteOnAPipeIsTheBareSentence(t *testing.T) {
	t.Parallel()

	// Act
	printed := saying(nil, func(notes console.Notes) { notes.Note("Created feat/PROJ-412-fix-login in a new worktree.") })

	// Assert
	if printed != "Created feat/PROJ-412-fix-login in a new worktree.\n" {
		t.Errorf("printed %q, want the sentence alone on its line", printed)
	}
}

func TestWarningsAndErrorsCarryTheirTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		says func(console.Notes)
		want string
	}{
		{
			name: "a warning",
			says: func(notes console.Notes) { notes.Warn("Not tagging anyone; posting untagged.") },
			want: "WARN Not tagging anyone; posting untagged.\n",
		},
		{
			name: "an error",
			says: func(notes console.Notes) { notes.Fail(errRefused) },
			want: "ERROR the Jira token was refused (401); run workflow doctor --online\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed := saying(nil, tt.says)

			// Assert
			if printed != tt.want {
				t.Errorf("printed %q, want %q", printed, tt.want)
			}
		})
	}
}

func TestAnErrorOfSeveralLinesKeepsThemAfterItsTag(t *testing.T) {
	t.Parallel()

	// Act
	printed := saying(nil, func(notes console.Notes) { notes.Fail(mistyped{}) })

	// Assert
	if printed != "ERROR unknown command \"launch\" for \"workflow\"\n\nRun 'workflow --help' for usage.\n" {
		t.Errorf("printed %q, want the tag before the first line and every line after it as it was", printed)
	}
}

func TestForcedColorDrawsTheTagsInTheirHues(t *testing.T) {
	t.Parallel()

	// Act
	printed := saying(forcedColor(), func(notes console.Notes) {
		notes.Note("Created the branch.")
		notes.Warn("Posting untagged.")
		notes.Fail(errRefused)
	})

	// Assert
	lines := strings.Split(printed, "\n")
	if len(lines) != 4 || lines[0] != "Created the branch." {
		t.Fatalf("printed %q, want an uncolored note, then the two tags", printed)
	}

	if !strings.HasPrefix(lines[1], "\x1b[1;33mWARN\x1b[m ") {
		t.Errorf("warning = %q, want WARN bold in ANSI 3", lines[1])
	}

	if !strings.HasPrefix(lines[2], "\x1b[1;31mERROR\x1b[m ") {
		t.Errorf("error = %q, want ERROR bold in ANSI 1", lines[2])
	}
}

func TestNoColorTurnsTheHuesOff(t *testing.T) {
	t.Parallel()

	// Arrange
	forcedButOff := forcedColor()
	forcedButOff["NO_COLOR"] = "1"

	// Act
	printed := saying(forcedButOff, func(notes console.Notes) {
		notes.Warn("Posting untagged.")
		notes.Fail(errRefused)
	})

	// Assert
	if hueParameter.MatchString(printed) || !strings.Contains(printed, "WARN") || !strings.Contains(printed, "ERROR") {
		t.Errorf("printed %q, want the tags with no hue", printed)
	}
}

func TestEachLineWrittenToTheNotesIsANote(t *testing.T) {
	t.Parallel()

	// Arrange
	preview := "Open PROJ-412: Fix login\n  feat/PROJ-412 → main\n\nThe body.\n"

	// Act
	printed := saying(nil, func(notes console.Notes) { _, _ = notes.Writer().Write([]byte(preview)) })

	// Assert
	if printed != preview {
		t.Errorf("printed %q, want each line as written, blank ones too", printed)
	}
}

func TestNotesSaidToAnotherStreamKeepTheirTagsAndHues(t *testing.T) {
	t.Parallel()

	// Arrange
	var first, other bytes.Buffer

	notes := console.New(&first, console.Options{Getenv: variables(forcedColor())})

	// Act
	notes.To(&other).Warn("Posting untagged.")

	// Assert
	if first.Len() != 0 || !strings.HasPrefix(other.String(), "\x1b[1;33mWARN\x1b[m Posting untagged.") {
		t.Errorf("first stream %q, other %q; want the warning on the other alone, tag and hue kept",
			first.String(), other.String())
	}
}

func TestNotesReadingNoVariablesAreTaggedWithoutHues(t *testing.T) {
	t.Parallel()

	// Arrange
	var stderr bytes.Buffer

	notes := console.New(&stderr, console.Options{})

	// Act
	notes.Warn("Posting untagged.")

	// Assert
	if stderr.String() != "WARN Posting untagged.\n" {
		t.Errorf("printed %q, want the tag with no hue", stderr.String())
	}
}
