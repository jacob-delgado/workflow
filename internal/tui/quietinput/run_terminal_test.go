// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

// Run asks on this process's own standard input and output, as plain prompts
// only where TERM is dumb, and asks for a secret only through a terminal,
// which only a pseudo-terminal can stand in for. So the tests of Run point
// the process's streams at one and set TERM there, and run alone, as
// .golangci.yml's paralleltest exclusion for their files says.

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/ptytest"
	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// runPatience bounds each wait on a run asking on a terminal, for it to reach
// a point the test needs, to return, or for what it wrote to be read, so a run
// that stops answering fails the test rather than hanging it.
const runPatience = 10 * time.Second

// askingTerminal is a pseudo-terminal this process's standard input and
// output are pointed at, as Run asks on them, and what was written to it.
type askingTerminal struct {
	keyboard, terminal *os.File
	shown              bytes.Buffer
	read               chan struct{}
}

// onATerminal points this process's standard input and output at a new
// pseudo-terminal, with TERM dumb, until the test ends, and keeps what is
// written to it.
func onATerminal(t *testing.T) *askingTerminal {
	t.Helper()
	t.Setenv("TERM", "dumb")

	keyboard, terminal := ptytest.Open(t)
	asking := &askingTerminal{keyboard: keyboard, terminal: terminal, read: make(chan struct{})}

	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = terminal, terminal

	t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })

	go func() {
		defer close(asking.read)

		_, _ = io.Copy(&asking.shown, keyboard)
	}()

	return asking
}

// typeAt types answer and enter at the terminal.
func (a *askingTerminal) typeAt(t *testing.T, answer string) {
	t.Helper()

	_, err := a.keyboard.WriteString(answer + "\n")
	if err != nil {
		t.Fatalf("typing %q at the terminal: %v", answer, err)
	}
}

// written closes the terminal and is all that was written to it.
func (a *askingTerminal) written(t *testing.T) string {
	t.Helper()

	_ = a.terminal.Close()

	select {
	case <-a.read:
	case <-time.After(runPatience):
		t.Fatalf("what Run wrote was not read to its end within %v of the terminal closing", runPatience)
	}

	return a.shown.String()
}

// running starts field's Run and is the wait for what it returns.
func running(t *testing.T, field *quietinput.Input) func() error {
	t.Helper()

	ended := make(chan error, 1)

	go func() { ended <- field.Run() }()

	return func() error {
		t.Helper()

		select {
		case err := <-ended:
			return err
		case <-time.After(runPatience):
			t.Fatalf("Run had not returned within %v of its answer", runPatience)

			return nil
		}
	}
}

func TestRunAsksOnTheTerminalAndKeepsTheAnswer(t *testing.T) {
	// Arrange
	var value string

	asking := onATerminal(t)
	field := quietinput.New(baseTitle, "", &value)

	asking.typeAt(t, baseBranch)

	// Act
	err := running(t, field)()

	// Assert
	shown := asking.written(t)
	if err != nil || value != baseBranch || !strings.Contains(shown, baseTitle) {
		t.Errorf("Run kept %q with %v, asking:\n%s\nwant %q kept under its title", value, err, shown, baseBranch)
	}
}
