// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"net/http"
	"net/http/httptest"
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

func TestARequestLogThatCannotBeWrittenStillEchoesUnderVerbose(t *testing.T) {
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
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t),
		"--verbose", "doctor", "--online", "--request-log", fullDevice)

	// Assert
	// The file refusing its outlines is no reason to lose the debug lines
	// --verbose asked for: each request is still said on stderr.
	wantExit(t, err, 0)

	if !strings.Contains(printed.Stderr, "DEBUG jira GET /rest/api/2/myself") {
		t.Errorf("stderr with --verbose and a request log that cannot be written:\n%s\nwant each request's "+
			"outline echoed as a debug line all the same", printed.Stderr)
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
	if err == nil || !strings.HasPrefix(err.Error(), "opening the request log: "+clitest.WorkdirUnread) {
		t.Errorf("--request-log requests.log from a removed directory = %v, want it to say the log cannot be "+
			"opened for the directory it cannot name", err)
	}
}

// heldRequest is a request the log wraps that is held until release closes,
// with arrived closed once it is out and answered once it is answered.
type heldRequest struct {
	arrived, release, answered chan struct{}
}

// newHeldRequest is a request not yet out.
func newHeldRequest() heldRequest {
	return heldRequest{arrived: make(chan struct{}), release: make(chan struct{}), answered: make(chan struct{})}
}

// doer answers once the request is released.
func (h heldRequest) doer(*http.Request) (*http.Response, error) {
	close(h.arrived)
	<-h.release

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

// closedUnder is a probe that opens the request log as a command does,
// closes it while held is still in flight, and lets held land after.
func closedUnder(held heldRequest) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		requestLog, closeLog, err := cmdkit.RequestLogFor(cmd)
		if err != nil {
			return err
		}

		//nolint:bodyclose // the request below closes the body of the response it is answered with
		do := requestLog.Wrap("jira", held.doer)

		go func() {
			defer close(held.answered)

			response, _ := do(httptest.NewRequestWithContext(cmd.Context(), http.MethodGet,
				"https://jira.example.com/rest/api/2/myself", nil))
			_ = response.Body.Close()
		}()

		<-held.arrived
		closeLog()
		close(held.release)
		<-held.answered

		return nil
	}
}

func TestARequestLogClosedUnderARequestInFlightSaysSoOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)
	root := probeTree(env, closedUnder(newHeldRequest()))

	// Act
	printed, err := runTree(t, root, "--request-log", "requests.log", probeCommand)

	// Assert
	warned := strings.Count(printed.Stderr, "WARN The request log could not be fully written: "+
		"requests still in flight as the request log closed, so not in it: 1")
	if err != nil || warned != 1 {
		t.Errorf("closing the log under a request = %v, warned %d times:\n%s\nwant it said once", err, warned,
			printed.Stderr)
	}

	written, readErr := os.ReadFile(filepath.Join(dir, "requests.log"))
	if readErr != nil || len(written) != 0 {
		t.Errorf("the log = %v, holding %q; want it kept, with no outline written after it closed", readErr,
			written)
	}
}

func TestARequestThatLandsAfterTheLogClosesIsStillEchoedUnderVerbose(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)
	root := probeTree(env, closedUnder(newHeldRequest()))

	// Act
	printed, err := runTree(t, root, verboseFlag, "--request-log", "requests.log", probeCommand)

	// Assert
	// The file is done with, but stderr is not: the debug lines --verbose
	// asked for still say the request, as they say every other.
	if err != nil || !strings.Contains(printed.Stderr, "DEBUG jira GET /rest/api/2/myself 200") {
		t.Errorf("--verbose with the log closed under a request = %v, stderr:\n%s\nwant the request's outline "+
			"echoed as a debug line all the same", err, printed.Stderr)
	}
}
