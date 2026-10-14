// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"bytes"
	"io"
	"regexp"
	"strings"
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
		err := notes.Watched(func(wait console.Wait) error {
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
