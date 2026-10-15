// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errKeyboardGone is a read of an answer that failed for a reason of its own.
var errKeyboardGone = errors.New("the keyboard went away")

func TestConfirmTakesOnlyAYesAsYes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer  string
		readErr error
		yes     bool
		wantErr error
	}{
		"y":                    {answer: "y", yes: true},
		"yes, spaced and loud": {answer: "  YES \n", yes: true},
		"n":                    {answer: "n"},
		"a bare enter":         {answer: ""},
		"nothing left to read": {readErr: io.EOF, wantErr: cmdkit.ErrNoTerminal},
		"a read that failed":   {readErr: errKeyboardGone, wantErr: errKeyboardGone},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var asked string

			prompt := cmdkit.Prompt{Line: func(question string) (string, error) {
				asked = question

				return tt.answer, tt.readErr
			}}

			// Act
			yes, err := cmdkit.Confirm(prompt, "Open it?")

			// Assert
			if yes != tt.yes || !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Errorf("%s = %v, %v; want %v, %v", name, yes, err, tt.yes, tt.wantErr)
			}

			if asked != "Open it? [y/N]: " {
				t.Errorf("asked %q, want the question with no as its default", asked)
			}
		})
	}
}

// waitedRoot is a tree whose probe waits as a slow command does, under
// prompt, printing artifact as its artifact while it waits.
func waitedRoot(t *testing.T, prompt cmdkit.Prompt, artifact string) *cobra.Command {
	t.Helper()

	env, _ := inATempDir(t)

	return probeTree(env, func(cmd *cobra.Command, _ []string) error {
		look := cmdkit.LiveLook(config.UI{ASCII: true})

		return cmdkit.Waiting(cmd, prompt, look, func(out cmdkit.Output, _ console.Wait) error {
			fmt.Fprintln(out.Artifact, artifact)

			return nil
		})
	})
}

// terminalsAre reports a stream a terminal when it is one of streams.
func terminalsAre(streams ...io.Writer) func(io.Writer) bool {
	return func(stream io.Writer) bool { return slices.Contains(streams, stream) }
}

func TestAWaitPrintsItsArtifactWhereThePersonWatchingSeesIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		terminals func(stdout, stderr io.Writer) func(io.Writer) bool
		onStdout  bool
	}{
		"no terminal at all": {
			terminals: func(io.Writer, io.Writer) func(io.Writer) bool { return nil },
			onStdout:  true,
		},
		"no stream a terminal": {
			terminals: func(io.Writer, io.Writer) func(io.Writer) bool { return terminalsAre() },
			onStdout:  true,
		},
		"stderr a terminal, stdout piped": {
			terminals: func(_, stderr io.Writer) func(io.Writer) bool { return terminalsAre(stderr) },
			onStdout:  true,
		},
		"both streams the terminal": {
			terminals: func(stdout, stderr io.Writer) func(io.Writer) bool { return terminalsAre(stdout, stderr) },
			onStdout:  false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var stdout, stderr bytes.Buffer

			root := waitedRoot(t, cmdkit.Prompt{IsTerminal: tt.terminals(&stdout, &stderr)}, "the report")
			root.SetArgs([]string{probeCommand})
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			// Act
			err := root.ExecuteContext(t.Context())

			// Assert
			onStdout := strings.Contains(stdout.String(), "the report")
			onStderr := strings.Contains(stderr.String(), "the report")

			if err != nil || onStdout != tt.onStdout || onStderr == tt.onStdout {
				t.Errorf("%s = %v; stdout %q, stderr %q; want the artifact on stdout: %v", name, err,
					stdout.String(), stderr.String(), tt.onStdout)
			}
		})
	}
}
