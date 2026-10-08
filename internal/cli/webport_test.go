// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// webFlag is the root's flag that serves the web interface.
const webFlag = "--web"

// rootServing is one run of the root command over the real web server: whether
// the server answered where the test looked, what the run said on stderr, and
// how it ended once stopped.
type rootServing struct {
	answered bool
	notes    string
	err      error
}

// serveRoot runs the root command with args in an empty directory and home of
// its own, over the real web server, until the server answers its health read
// at base or the run ends by itself, and then stops it. Like run, it hands the
// run an Environment of its own.
func serveRoot(t *testing.T, base string, args ...string) rootServing {
	t.Helper()

	env := environmentFor(t, place{dir: t.TempDir(), home: t.TempDir()})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	notes := &sharedNotes{}
	root := cli.NewRootCmdOver(unusedPrompt(t), tui.Run, cli.WebServerAt, env)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(notes)

	done := make(chan error, 1)

	go func() { done <- root.ExecuteContext(ctx) }()

	answered, ended, err := awaitAnswerOrEnd(t, base, done)
	if !ended {
		cancel()

		err = <-done
	}

	return rootServing{answered: answered, notes: notes.String(), err: err}
}

// awaitAnswerOrEnd waits until the server at base answers its health read, the
// run that serves it ends, or five seconds pass. It reports whether the server
// answered and whether the run ended, with how.
func awaitAnswerOrEnd(t *testing.T, base string, done <-chan error) (bool, bool, error) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			return false, true, err
		default:
		}

		response, err := getURL(t, base+"/api/health")
		if err == nil {
			_ = response.Body.Close()

			return true, false, nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return false, false, nil
}

func TestTheWebFlagServesOnThePortItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	addr := freeLoopbackAddr(t)

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("reading the port of %s: %v", addr, err)
	}

	// Act
	served := serveRoot(t, "http://"+addr, webFlag, "--port", port)

	// Assert
	if !served.answered || served.err != nil {
		t.Fatalf("workflow --web --port %s answered at %s: %v, and ended with %v; want it serving there until stopped",
			port, addr, served.answered, served.err)
	}

	if !strings.Contains(served.notes, "serving http://"+addr+"/#session=") {
		t.Errorf("workflow --web --port %s said %q, want where it serves", port, served.notes)
	}
}

func TestThePortFlagWithoutTheWebFlagIsAMistakeInTheCall(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir(), "--port", "7001")

	// Assert
	if got := cli.ExitStatus(ran.err); got != 2 || !strings.Contains(fmt.Sprint(ran.err), webFlag) {
		t.Errorf("workflow --port 7001 = %v, exit %d; want exit 2 and --web named", ran.err, got)
	}

	if ran.interfaces != 0 || ran.servers != 0 {
		t.Errorf("workflow --port 7001 opened %d interfaces and %d servers, want neither", ran.interfaces, ran.servers)
	}
}

func TestAPortNoListenerCanTakeIsAMistakeInTheCall(t *testing.T) {
	t.Parallel()

	for name, port := range map[string]string{"zero": "0", "past the last": "65536", "negative": "-1"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			ran := runRoot(t, t.TempDir(), webFlag, "--port="+port)

			// Assert
			if got := cli.ExitStatus(ran.err); got != 2 || !strings.Contains(fmt.Sprint(ran.err), `"`+port+`" for "--port"`) {
				t.Errorf("workflow --web --port=%s = %v, exit %d; want exit 2 and the port refused by name", port, ran.err, got)
			}

			if ran.servers != 0 {
				t.Errorf("workflow --web --port=%s served %d times, want never", port, ran.servers)
			}
		})
	}
}

func TestTheWebFlagServesTheLoopbackInterface(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		want string
	}{
		"on its default port": {args: []string{webFlag}, want: "127.0.0.1:13579"},
		"on the port named":   {args: []string{webFlag, "--port", "7001"}, want: "127.0.0.1:7001"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			ran := runRoot(t, t.TempDir(), tt.args...)

			// Assert
			if ran.err != nil || ran.servers != 1 || ran.addr != tt.want {
				t.Errorf("workflow %v = %v, served %d times at %q; want once, at %s",
					tt.args, ran.err, ran.servers, ran.addr, tt.want)
			}
		})
	}
}
