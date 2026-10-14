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
)

// eraseBelow is what the live line's program sends as it ends, erasing the
// line it kept.
const eraseBelow = "\x1b[J"

// liveMarks are the frames the live line's mark turns through.
func liveMarks() []string {
	return []string{"◐", "◓", "◑", "◒"}
}

// markedLive reports whether shown holds the live line's mark.
func markedLive(shown string) bool {
	for _, mark := range liveMarks() {
		if strings.Contains(shown, mark) {
			return true
		}
	}

	return false
}

// markDrawn is where the live line's mark was first and last drawn in shown,
// or -1 for both when it never was.
func markDrawn(shown string) (int, int) {
	first, last := -1, -1

	for _, mark := range liveMarks() {
		at := strings.Index(shown, mark)
		if at >= 0 && (first < 0 || at < first) {
			first = at
		}

		last = max(last, strings.LastIndex(shown, mark))
	}

	return first, last
}

// atTerminal is unusedPrompt with every stream flagged a terminal, as main
// flags a real one.
func atTerminal(t *testing.T) cli.Prompt {
	t.Helper()

	prompt := unusedPrompt(t)
	prompt.IsTerminal = func(io.Writer) bool { return true }

	return prompt
}

// runWatched runs args in dir with stderr a stand-in terminal, and stdout one
// too when both is set, and returns what each stream was sent.
func runWatched(t *testing.T, dir string, both bool, args ...string) (streams, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	prompt := unusedPrompt(t)
	prompt.IsTerminal = func(stream io.Writer) bool {
		return stream == &stderr || both && stream == &stdout
	}

	err := cli.Execute(args, &stdout, &stderr, prompt, environmentFor(t, place{dir: dir, home: t.TempDir()}))

	return streams{stdout: stdout.String(), stderr: stderr.String()}, err
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
				writeFile(t, dir, `{"jira": {"base_url": "https://jira.invalid", "token": "t"}}`)

				return dir, strings.Fields("doctor --online --request-log " + filepath.Join(dir, "requests.log"))
			},
			want: "Checking",
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
			printed, _ := runStreams(t, dir, atTerminal(t), args...)

			// Assert
			first, last := markDrawn(printed.stderr)
			if first < 0 || !strings.Contains(printed.stderr[first:], tt.want) {
				t.Errorf("%s at a terminal: stderr = %q, want the live line %q", name, printed.stderr, tt.want)
			}

			if erased := strings.LastIndex(printed.stderr, eraseBelow); erased < last {
				t.Errorf("%s left its live line on the terminal: stderr = %q", name, printed.stderr)
			}

			if strings.Contains(printed.stdout, tt.want) || markedLive(printed.stdout) {
				t.Errorf("%s put its live line on stdout: %q", name, printed.stdout)
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
	if !strings.Contains(printed.stdout, "Version:") || strings.Contains(printed.stderr, "Version:") {
		t.Errorf("stdout %q, stderr %q; want the report on stdout alone", printed.stdout, printed.stderr)
	}
}

func TestAWaitWithBothStreamsOnTheTerminalPrintsItsArtifactAboveTheLine(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, args := progressCases()["doctor, checking online"].setUp(t)

	// Act
	printed, _ := runWatched(t, dir, true, args...)

	// Assert
	if printed.stdout != "" || !strings.Contains(printed.stderr, "Version:") || !markedLive(printed.stderr) {
		t.Errorf("stdout %q, stderr %q; want the report above the live line, on stderr", printed.stdout, printed.stderr)
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
			if strings.Contains(printed.stdout+printed.stderr, tt.want) || markedLive(printed.stderr) ||
				strings.Contains(printed.stdout+printed.stderr, "\x1b[") {
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
