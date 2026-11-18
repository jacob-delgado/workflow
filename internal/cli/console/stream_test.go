// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli/console"
)

func TestAStreamWritesStraightThroughWhileNoLineIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	var stderr bytes.Buffer

	stream := console.NewStream(&stderr)

	// Act
	_, err := stream.Write([]byte("serving http://127.0.0.1:13579\n"))

	// Assert
	if err != nil || stderr.String() != "serving http://127.0.0.1:13579\n" {
		t.Errorf("wrote %v, stderr %q; want the line as written", err, stderr.String())
	}
}

func TestWhatIsWrittenToTheStreamDuringAWaitPrintsAboveItsLine(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		outlastTurn(t)

		terminal := new(standIn)
		stream := console.NewStream(terminal)
		notes := console.New(stream, console.Options{})

		// Act
		err := notes.Watched(console.Look{}, func(wait console.Wait) error {
			wait.Doing("Checking Jira")
			terminal.shows(t, "Checking Jira")

			_, _ = io.WriteString(stream, "jira GET /rest/api/2/myself 200 184ms\n")

			terminal.shows(t, "184ms")

			return nil
		})

		// Assert
		shown := terminal.String()
		if err != nil || !strings.Contains(shown, ansi.InsertLine(1)) || !strings.Contains(shown, "184ms") {
			t.Errorf("watched = %v, the terminal was sent %q; want the line inserted above the live line", err, shown)
		}
	})
}

func TestUnderlyingIsTheStreamAStreamWritesThrough(t *testing.T) {
	t.Parallel()

	// Arrange
	var stderr bytes.Buffer

	// Act
	beneath, plain := console.Underlying(console.NewStream(&stderr)), console.Underlying(&stderr)

	// Assert
	if beneath != &stderr || plain != &stderr {
		t.Errorf("Underlying = %v and %v, want stderr both times", beneath, plain)
	}
}

// lateRuns is how many times a line is written as the live line ends.
const lateRuns = 200

func TestALineWrittenAsTheLiveLineEndsStillPrints(t *testing.T) {
	t.Parallel()

	// The late write and the end of the wait race for the stream's mutex, and
	// Go's mutex lets the program's goroutine barge back in for the writes that
	// restore the terminal and for the stream's return to writing straight
	// through, so no hook a black-box test has can order the late write first:
	// the one Act runs lateRuns times, a fresh stream and terminal each, for
	// the orderings it meets.

	// Act
	dropped := runsDroppingALateLine(t, lateRuns)

	// Assert
	if dropped > 0 {
		t.Errorf("output lacks the line written as the live line ended in %d of %d runs", dropped, lateRuns)
	}
}

// runsDroppingALateLine is how many of runs waits, each watched on a fresh
// stream and terminal, lost a line written to the stream once the wait was
// over, as the live line was erased.
func runsDroppingALateLine(t *testing.T, runs int) int {
	t.Helper()

	dropped := 0

	for range runs {
		if !strings.Contains(ansi.Strip(writtenAsTheLineEnds(t)), "written late") {
			dropped++
		}
	}

	return dropped
}

// writtenAsTheLineEnds is what a terminal shows of a watched wait over which,
// once the wait is over, a line is written to the stream from a goroutine of
// its own as the program erases the live line.
func writtenAsTheLineEnds(t *testing.T) string {
	t.Helper()

	var printed string

	synctest.Test(t, func(t *testing.T) {
		outlastTurn(t)

		terminal := &erasedLate{written: make(chan struct{})}
		stream := console.NewStream(terminal)
		terminal.stream = stream

		_ = console.New(stream, console.Options{}).Watched(console.Look{}, func(wait console.Wait) error {
			wait.Doing("Waiting on CI")
			terminal.shows(t, "Waiting on CI")
			terminal.over.Store(true)

			return nil
		})

		<-terminal.written

		printed = terminal.String()
	})

	return printed
}

// erasedLate is a stand-in terminal that, once over is set, writes a line to
// stream from a goroutine of its own when the live line is erased, and
// closes written once that write returns.
type erasedLate struct {
	standIn

	stream  io.Writer
	over    atomic.Bool
	once    sync.Once
	written chan struct{}
}

var _ io.Writer = (*erasedLate)(nil)

// Write keeps output, and has the line written once the wait is over and
// output erases the live line.
func (e *erasedLate) Write(output []byte) (int, error) {
	if e.over.Load() && bytes.Contains(output, []byte(ansi.EraseScreenBelow)) {
		e.once.Do(func() {
			go func() {
				defer close(e.written)

				_, _ = io.WriteString(e.stream, "written late\n")
			}()
		})
	}

	return e.standIn.Write(output)
}

// How many times a wait yields to the line racing its end before it ends,
// and how long that line is: long enough that writing it takes the time the
// wait's end needs to overtake it, which a short line almost never does.
const (
	racingYields = 10
	racingLength = 1 << 15
)

func TestALineOnItsWayAboveTheLiveLineAsTheWaitEndsStillPrints(t *testing.T) {
	t.Parallel()

	// A write that saw the line drawn is on its way above it, the stream
	// unlocked, as the wait ends, and nothing a black-box test can hook sits
	// between the two: the one Act runs lateRuns times, a fresh stream and
	// terminal each, for the orderings it meets.

	// Act
	dropped := runsDroppingARacingLine(t, lateRuns)

	// Assert
	if dropped > 0 {
		t.Errorf("output lacks the line on its way above the live line as the wait ended in %d of %d runs",
			dropped, lateRuns)
	}
}

// runsDroppingARacingLine is how many of runs waits, each watched on a fresh
// stream and terminal, lost a line written to the stream as the wait ended.
func runsDroppingARacingLine(t *testing.T, runs int) int {
	t.Helper()

	dropped := 0

	for range runs {
		if !strings.Contains(writtenAsTheWaitEnds(t), "racing") {
			dropped++
		}
	}

	return dropped
}

// writtenAsTheWaitEnds is what a terminal shows of a watched wait that ends
// as a goroutine of its own writes a long line to the stream, once that write
// has returned.
func writtenAsTheWaitEnds(t *testing.T) string {
	t.Helper()

	var printed string

	synctest.Test(t, func(t *testing.T) {
		outlastTurn(t)

		terminal := new(standIn)
		stream := console.NewStream(terminal)
		racing := "racing" + strings.Repeat(".", racingLength) + "\n"

		var writer sync.WaitGroup

		_ = console.New(stream, console.Options{}).Watched(console.Look{}, func(wait console.Wait) error {
			wait.Doing("Waiting on CI")
			terminal.shows(t, "Waiting on CI")
			writer.Go(func() { _, _ = io.WriteString(stream, racing) })

			for range racingYields {
				runtime.Gosched()
			}

			return nil
		})

		writer.Wait()

		printed = terminal.String()
	})

	return printed
}

// errTerminalGone is a terminal that hung up.
var errTerminalGone = errors.New("input/output error")

func TestAWatchedWaitSucceedsThoughStderrFailsAsItsLineEnds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		outlastTurn(t)

		terminal := new(hungUp)
		notes := console.New(console.NewStream(terminal), console.Options{})
		posted := false

		// Act
		err := notes.Watched(console.Look{}, func(wait console.Wait) error {
			wait.Doing("Posting the announcement")
			terminal.shows(t, "Posting the announcement")
			terminal.gone.Store(true)

			posted = true

			return nil
		})

		// Assert
		if err != nil || !posted {
			t.Errorf("watched = %v once stderr failed as the line ended, the wait run: %v; want the wait's "+
				"own outcome, nil", err, posted)
		}
	})
}

// hungUp is a stand-in terminal whose writes fail once gone is set.
type hungUp struct {
	standIn

	gone atomic.Bool
}

var _ io.Writer = (*hungUp)(nil)

// Write keeps output, or, once the terminal is gone, fails.
func (h *hungUp) Write(output []byte) (int, error) {
	if h.gone.Load() {
		return 0, errTerminalGone
	}

	return h.standIn.Write(output)
}

// farMove matches a cursor movement of four rows or more, up or down.
var farMove = regexp.MustCompile(`\x1b\[([4-9]|[1-9][0-9]+)[AB]`)

func TestWhatIsSaidBeforeTheLineIsDrawnMovesTheCursorNoFurtherThanItPrints(t *testing.T) {
	t.Parallel()

	// Act
	printed, _ := watching(t, func(terminal *standIn, wait console.Wait) error {
		wait.Notes().Note("Version: 1.0")
		wait.Doing("Checking Jira")
		terminal.shows(t, "Checking Jira")
		wait.Notes().Note("Jira answered.")

		return nil
	})

	// Assert
	shown := ansi.Strip(printed)
	if farMove.MatchString(printed) || !strings.Contains(shown, "Version: 1.0") ||
		!strings.Contains(shown, "Jira answered.") {
		t.Errorf("the terminal was sent %q; want each note printed where it belongs, the cursor kept close", printed)
	}
}
