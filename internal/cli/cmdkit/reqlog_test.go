// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

func TestRequestLogWarnsOnceWhenItCouldNotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every write to /dev/full fails as a full disk would; where there is none,
	// no file can be made to refuse writes from a test.
	const fullDevice = "/dev/full"

	_, statErr := os.Stat(fullDevice)
	if statErr != nil {
		t.Skip("no " + fullDevice + " here to refuse the request log's writes")
	}

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--online", "--request-log", fullDevice)

	// Assert
	// The check itself succeeded, so its status stands; only the bug report
	// it was asked to keep is incomplete, and one line on stderr says so.
	wantExit(t, err, 0)

	if got := strings.Count(printed.Stderr, "WARN The request log could not be fully written"); got != 1 {
		t.Errorf("stderr warned %d times that the request log was not written, want once:\n%s", got, printed.Stderr)
	}
}

// requestLogRoot is a tree whose root and probe each open the request log as
// a command does, keeping whether one was opened and closing it before they
// end.
func requestLogRoot(env cmdkit.Environment, opened *bool) *cobra.Command {
	return probeTree(env, func(cmd *cobra.Command, _ []string) error {
		requestLog, closeLog, err := cmdkit.RequestLogFor(cmd)
		*opened = requestLog != nil

		closeLog()

		return err
	})
}

func TestARequestLogIsKeptOnlyWhereOneIsAskedFor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args   []string
		opened bool
	}{
		"a command":                      {args: []string{probeCommand}},
		"a command, --verbose":           {args: []string{verboseFlag, probeCommand}, opened: true},
		"bare workflow, --verbose":       {args: []string{verboseFlag}},
		"bare workflow --web, --verbose": {args: []string{verboseFlag, webFlag}, opened: true},
		"a command, --quiet":             {args: []string{"--quiet", probeCommand}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, _ := inATempDir(t)

			var opened bool

			root := requestLogRoot(env, &opened)

			// Act
			_, err := runTree(t, root, tt.args...)

			// Assert
			if err != nil || opened != tt.opened {
				t.Errorf("%s = %v, a log kept: %v; want one: %v", name, err, opened, tt.opened)
			}
		})
	}
}

func TestARequestLogNamedRelativelyIsKeptInTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)

	var opened bool

	root := requestLogRoot(env, &opened)

	// Act
	_, err := runTree(t, root, "--request-log", "requests.log", probeCommand)

	// Assert
	info, statErr := os.Stat(filepath.Join(dir, "requests.log"))
	if err != nil || !opened || statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("--request-log requests.log = %v, opened %v, file %v; want it made in the working directory, "+
			"readable by its owner alone", err, opened, statErr)
	}
}

func TestARequestLogNamedRelativelyNeedsTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	var opened bool

	root := requestLogRoot(fromARemovedDirectory(t), &opened)

	// Act
	_, err := runTree(t, root, "--request-log", "requests.log", probeCommand)

	// Assert
	if err == nil || !strings.HasPrefix(err.Error(), "opening the request log: "+workdirUnread) {
		t.Errorf("--request-log requests.log from a removed directory = %v, want it to say the log cannot be "+
			"opened for the directory it cannot name", err)
	}
}
