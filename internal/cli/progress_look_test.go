// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// unicodeLine is every character the live line draws by default and never in
// ASCII: its mark's frames, and its bar's.
func unicodeLine() []string {
	return append(liveMarks(), "━", "─")
}

// lookCase is a slow command run under a look its live line is drawn in: the
// configuration at home that asks for it, if any, the mark the line opens
// with, and what it never shows.
type lookCase struct {
	slow  progressCase
	home  string
	mark  string
	never []string
}

// plainly is setUp's command with --plain.
func plainly(setUp func(t *testing.T) (string, []string)) func(t *testing.T) (string, []string) {
	return func(t *testing.T) (string, []string) {
		t.Helper()

		dir, args := setUp(t)

		return dir, append(args, "--plain")
	}
}

func lookCases() map[string]lookCase {
	cases := make(map[string]lookCase)

	for name, slow := range progressCases() {
		cases[name+", under ui.ascii"] = lookCase{
			slow: slow, home: `{"ui": {"ascii": true}}`, mark: "*", never: unicodeLine(),
		}
		cases[name+", under ui.motion reduce"] = lookCase{
			slow: slow, home: `{"ui": {"motion": "reduce"}}`, mark: "◐", never: []string{"◓", "◑", "◒"},
		}
	}

	named := progressCases()["status of a named directory"]
	cases["status of a named directory, with --plain"] = lookCase{
		slow: progressCase{setUp: plainly(named.setUp), want: named.want}, mark: "*", never: unicodeLine(),
	}

	return cases
}

// runWatchedAt runs args at where with stderr a stand-in terminal and stdout
// piped, and returns what stderr was sent.
func runWatchedAt(t *testing.T, where clitest.Place, args ...string) (string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	prompt := clitest.UnusedPrompt(t)
	prompt.IsTerminal = func(stream io.Writer) bool { return stream == &stderr }

	err := cli.Execute(args, &stdout, &stderr, prompt, clitest.EnvironmentFor(t, where))

	return stderr.String(), err
}

func TestASlowCommandDrawsItsLiveLineAsItsLookAsks(t *testing.T) {
	t.Parallel()

	for name, tt := range lookCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir, args := tt.slow.setUp(t)

			home := t.TempDir()
			if tt.home != "" {
				clitest.WriteFile(t, home, tt.home)
			}

			// Act
			stderr, _ := runWatchedAt(t, clitest.Place{Dir: dir, Home: home}, args...)

			// Assert
			shown := ansi.Strip(stderr)
			if !strings.Contains(shown, tt.mark+" "+tt.slow.want) {
				t.Errorf("%s: stderr = %q, want the live line drawn with %q", name, shown, tt.mark)
			}

			for _, never := range tt.never {
				if strings.Contains(shown, never) {
					t.Errorf("%s: stderr = %q, want no %q", name, shown, never)
				}
			}
		})
	}
}
