// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// eraseLine is how a progress note is replaced, and cleared at the end.
const eraseLine = "\r\x1b[K"

// atTerminal is unusedPrompt with stderr flagged a terminal, as main flags a
// real one.
func atTerminal(t *testing.T) cli.Prompt {
	t.Helper()

	prompt := unusedPrompt(t)
	prompt.IsTerminal = func(io.Writer) bool { return true }

	return prompt
}

// progressCase is a slow command, and the note it should keep while it works.
type progressCase struct {
	setUp func(t *testing.T) (dir string, args []string)
	want  string
}

func progressCases() map[string]progressCase {
	return map[string]progressCase{
		"doctor, checking online": {
			setUp: func(t *testing.T) (string, []string) {
				t.Helper()

				dir := t.TempDir()
				writeFile(t, dir, `{"jira": {"base_url": "https://jira.invalid", "token": "t"}}`)

				return dir, strings.Fields("doctor --online --log " + filepath.Join(dir, "requests.log"))
			},
			want: "Checking Jira…",
		},
		"summary of a day": {
			setUp: func(t *testing.T) (string, []string) {
				t.Helper()

				fakeGh(t, ghResponses{})
				repo := workedRepository(t, "Add the widget")
				git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")
				writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},"taskwarrior":{"disabled":true}}`)

				return repo, strings.Fields("summary --from " + summaryDay + " --to " + summaryDay)
			},
			want: "Reading GitHub…",
		},
		"status of a named directory": {
			setUp: func(t *testing.T) (string, []string) {
				t.Helper()

				dir := filepath.Join(t.TempDir(), "alpha")
				mustMkdir(t, dir)

				return t.TempDir(), []string{"status", dir}
			},
			want: "Reading alpha…",
		},
	}
}

func TestASlowCommandSaysWhatItIsReadingOnATerminal(t *testing.T) {
	t.Parallel()

	for name, tt := range progressCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir, args := tt.setUp(t)

			// Act
			printed, _ := runStreams(t, dir, atTerminal(t), args...)

			// Assert
			if !strings.Contains(printed.stderr, eraseLine+tt.want) {
				t.Errorf("%s at a terminal: stderr = %q, want the note %q", name, printed.stderr, tt.want)
			}

			last := strings.LastIndex(printed.stderr, eraseLine)
			if last < 0 || strings.Contains(printed.stderr[last+len(eraseLine):], "…") {
				t.Errorf("%s left a note on the line: stderr = %q", name, printed.stderr)
			}

			if strings.Contains(printed.stdout, tt.want) {
				t.Errorf("%s put its note on stdout: %q", name, printed.stdout)
			}
		})
	}
}

func TestASlowCommandSaysNothingOfItsProgressOffATerminal(t *testing.T) {
	t.Parallel()

	for name, tt := range progressCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir, args := tt.setUp(t)

			// Act
			printed, _ := runStreams(t, dir, unusedPrompt(t), args...)

			// Assert
			if strings.Contains(printed.stdout+printed.stderr, tt.want) ||
				strings.Contains(printed.stdout+printed.stderr, "\r") {
				t.Errorf("%s off a terminal printed a progress note:\n%q\n%q", name, printed.stdout, printed.stderr)
			}
		})
	}
}

// mustMkdir makes dir, failing the test when it cannot.
func mustMkdir(t *testing.T, dir string) {
	t.Helper()

	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
}
