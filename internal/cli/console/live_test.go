// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli/console"
)

// errWaitFailed is a wait that ended in failure.
var errWaitFailed = errors.New("the forge did not answer")

// liveMarks are the frames the live line's mark turns through.
func liveMarks() []string {
	return []string{"◐", "◓", "◑", "◒"}
}

// frame is how long the live line's program takes to draw what it was told:
// its renderer draws sixty frames a second.
const frame = time.Second / 60

// standIn is a stand-in terminal: what the live line's program, drawing
// from goroutines of its own, has written to it so far.
type standIn struct {
	mu      sync.Mutex
	written strings.Builder
}

var _ io.Writer = (*standIn)(nil)

// Write keeps p.
func (s *standIn) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written.Write(p) //nolint:wrapcheck // a builder's write never fails
}

// String is everything written so far.
func (s *standIn) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written.String()
}

// shows lets the live line's program draw a frame of what it was told, and
// fails the test unless the terminal then shows text. It runs in a bubble,
// whose fake clock moves only once every goroutine in it waits, so the frame
// is drawn when the test lets it rather than when a real clock does.
func (s *standIn) shows(t *testing.T, text string) {
	t.Helper()

	time.Sleep(frame)
	synctest.Wait()

	if !strings.Contains(ansi.Strip(s.String()), text) {
		t.Errorf("the terminal did not show %q: %q", text, s.String())
	}
}

// outlastTurn has the bubble t runs in outlast the mark's next turn, which is
// still on its timer as the program ends: once t is done, a second on the
// fake clock lets it fire, so the bubble ends with nothing in it left waiting.
func outlastTurn(t *testing.T) {
	t.Helper()

	t.Cleanup(func() { time.Sleep(time.Second) })
}

// watching is what a wait watched on a stand-in terminal printed, and what it
// returned, run in a bubble so the line's frames come on its fake clock.
func watching(t *testing.T, wait func(*standIn, console.Wait) error) (string, error) {
	t.Helper()

	var (
		printed string
		err     error
	)

	synctest.Test(t, func(t *testing.T) {
		outlastTurn(t)

		terminal := new(standIn)

		notes := console.New(terminal, console.Options{})
		err = notes.Watched(func(watched console.Wait) error { return wait(terminal, watched) })
		printed = terminal.String()
	})

	return printed, err
}

// marked reports whether printed shows the live line's mark.
func marked(printed string) bool {
	for _, mark := range liveMarks() {
		if strings.Contains(printed, mark) {
			return true
		}
	}

	return false
}

func TestAWatchedWaitKeepsALiveLineOfWhatItIsDoing(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := watching(t, func(terminal *standIn, wait console.Wait) error {
		wait.Doing("Checking Jira")
		terminal.shows(t, "Checking Jira")
		wait.Counted(1, 3)
		terminal.shows(t, "1 of 3")
		wait.Notes().Note("Jira answered.")

		return nil
	})

	// Assert
	shown := ansi.Strip(printed)
	if err != nil || !marked(shown) || !strings.Contains(shown, "━") {
		t.Errorf("watched = %v, showing %q; want the mark and the bar", err, shown)
	}

	if !strings.Contains(shown, "Jira answered.") {
		t.Errorf("showing %q, want the note said during the wait", shown)
	}
}

func TestAWatchedWaitErasesItsLiveLineAtTheEnd(t *testing.T) {
	t.Parallel()

	// Act
	printed, _ := watching(t, func(terminal *standIn, wait console.Wait) error {
		wait.Doing("Reading GitHub")
		terminal.shows(t, "Reading GitHub")
		wait.Notes().Note("Read GitHub.")

		return nil
	})

	// Assert
	drawn, erased := strings.LastIndex(printed, "Reading GitHub"), strings.LastIndex(printed, ansi.EraseScreenBelow)
	if drawn < 0 || erased < drawn || strings.TrimSpace(ansi.Strip(printed[erased:])) != "" {
		t.Errorf("the terminal was sent %q; want the live line erased last, and nothing after it", printed)
	}
}

func TestAWatchedWaitAnswersWithTheWaitsOutcome(t *testing.T) {
	t.Parallel()

	// Act
	_, err := watching(t, func(*standIn, console.Wait) error { return errWaitFailed })

	// Assert
	if !errors.Is(err, errWaitFailed) {
		t.Errorf("watched = %v, want the wait's own error", err)
	}
}

func TestLinesPrintedAboveTheLiveLineKeepTheirBlankOnes(t *testing.T) {
	t.Parallel()

	// Act
	printed, _ := watching(t, func(terminal *standIn, wait console.Wait) error {
		wait.Doing("Checking Jira")
		terminal.shows(t, "Checking Jira")

		_, _ = wait.Above().Write([]byte("Version: 1.0\n"))
		_, _ = wait.Above().Write([]byte("\n"))
		_, _ = wait.Above().Write([]byte("Repository: workflow\n"))

		return nil
	})

	// Assert
	shown := ansi.Strip(printed)
	version, repository := strings.Index(shown, "Version: 1.0"), strings.Index(shown, "Repository: workflow")

	if version < 0 || repository < version || strings.Count(shown[version:repository], "\n") < 2 {
		t.Errorf("showing %q, want both lines in order with the blank line between", shown)
	}
}

func TestAnUnwatchedWaitSaysWhatItPrintsAndKeepsNoLine(t *testing.T) {
	t.Parallel()

	// Arrange
	var stderr bytes.Buffer

	notes := console.New(&stderr, console.Options{})

	// Act
	err := notes.Unwatched(func(wait console.Wait) error {
		wait.Doing("Checking Jira")
		wait.Counted(1, 3)
		wait.Notes().Note("Jira answered.")
		_, _ = wait.Above().Write([]byte("printed beside\n"))

		return nil
	})

	// Assert
	if err != nil || stderr.String() != "Jira answered.\nprinted beside\n" {
		t.Errorf("unwatched = %v, stderr %q; want the note and the line, and no live line", err, stderr.String())
	}
}
