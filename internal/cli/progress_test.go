// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// markDrawn is where the live line's mark was first and last drawn in shown,
// or -1 for both when it never was.
func markDrawn(shown string) (int, int) {
	first, last := -1, -1

	for _, mark := range clitest.LiveMarks() {
		at := strings.Index(shown, mark)
		if at >= 0 && (first < 0 || at < first) {
			first = at
		}

		last = max(last, strings.LastIndex(shown, mark))
	}

	return first, last
}

// runWatched runs args in dir with stderr a stand-in terminal, and stdout one
// too when both is set, and returns what each stream was sent.
func runWatched(t *testing.T, dir string, both bool, args ...string) (clitest.Streams, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	prompt := clitest.UnusedPrompt(t)
	prompt.IsTerminal = func(stream io.Writer) bool {
		return stream == &stderr || both && stream == &stdout
	}

	err := cli.Execute(
		args, &stdout, &stderr, prompt, clitest.EnvironmentFor(t, clitest.Place{Dir: dir, Home: t.TempDir()}),
	)

	return clitest.Streams{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

// progressCase is a slow command, and the verb its live line opens with while
// it works: the subject after it is drawn only as fast as the terminal is
// redrawn, which a quick read can outrun.
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
				clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.invalid", "token": "t"}}`)

				return dir, strings.Fields("doctor --online --request-log " + filepath.Join(dir, "requests.log"))
			},
			want: "Checking",
		},
		"summary of a day": {
			setUp: func(t *testing.T) (string, []string) {
				t.Helper()

				clitest.FakeGh(t, clitest.GhResponses{})
				repo := workedRepository(t, "Add the widget")
				clitest.Git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")
				clitest.WriteFile(
					t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},"taskwarrior":{"disabled":true}}`,
				)

				return repo, strings.Fields("summary --from " + summaryDay + " --to " + summaryDay)
			},
			want: "Reading",
		},
		"status of a named directory": {
			setUp: func(t *testing.T) (string, []string) {
				t.Helper()

				dir := filepath.Join(t.TempDir(), "alpha")
				mustMkdir(t, dir)

				return t.TempDir(), []string{"status", dir}
			},
			want: "Reading",
		},
	}
}

func TestASlowCommandKeepsALiveLineOfWhatItIsReadingOnATerminal(t *testing.T) {
	t.Parallel()

	for name, tt := range progressCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir, args := tt.setUp(t)

			// Act
			printed, _ := clitest.RunStreams(t, dir, clitest.AtTerminal(t), args...)

			// Assert
			first, last := markDrawn(printed.Stderr)
			if first < 0 || !strings.Contains(printed.Stderr[first:], tt.want) {
				t.Errorf("%s at a terminal: stderr = %q, want the live line %q", name, printed.Stderr, tt.want)
			}

			if erased := strings.LastIndex(printed.Stderr, clitest.EraseBelow); erased < last {
				t.Errorf("%s left its live line on the terminal: stderr = %q", name, printed.Stderr)
			}

			if strings.Contains(printed.Stdout, tt.want) || clitest.MarkedLive(printed.Stdout) {
				t.Errorf("%s put its live line on stdout: %q", name, printed.Stdout)
			}
		})
	}
}

func TestAWaitOnATerminalStillWritesAPipedStdoutDirectly(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, args := progressCases()["doctor, checking online"].setUp(t)

	// Act
	printed, _ := runWatched(t, dir, false, args...)

	// Assert
	if !strings.Contains(printed.Stdout, "Version:") || strings.Contains(printed.Stderr, "Version:") {
		t.Errorf("stdout %q, stderr %q; want the report on stdout alone", printed.Stdout, printed.Stderr)
	}
}

func TestAWaitWithBothStreamsOnTheTerminalPrintsItsArtifactAboveTheLine(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, args := progressCases()["doctor, checking online"].setUp(t)

	// Act
	printed, _ := runWatched(t, dir, true, args...)

	// Assert
	if printed.Stdout != "" || !strings.Contains(printed.Stderr, "Version:") || !clitest.MarkedLive(printed.Stderr) {
		t.Errorf("stdout %q, stderr %q; want the report above the live line, on stderr", printed.Stdout, printed.Stderr)
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
			printed, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), args...)

			// Assert
			if strings.Contains(printed.Stdout+printed.Stderr, tt.want) || clitest.MarkedLive(printed.Stderr) ||
				strings.Contains(printed.Stdout+printed.Stderr, "\x1b[") {
				t.Errorf("%s off a terminal printed a progress note:\n%q\n%q", name, printed.Stdout, printed.Stderr)
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
