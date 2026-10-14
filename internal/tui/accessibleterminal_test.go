// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/ptytest"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// promptPatience bounds each wait on a run asking on a terminal — for what it
// writes, or for the move it sends — so a run that stops answering fails the
// test rather than hanging it.
const promptPatience = 10 * time.Second

// transcript is what a run writes, kept for a test to read while the run is
// still writing, which says each time it is written to.
type transcript struct {
	mu      sync.Mutex
	written strings.Builder
	more    chan struct{}
}

// newTranscript is an empty transcript.
func newTranscript() *transcript {
	return &transcript{more: make(chan struct{}, 1)}
}

// Write keeps what was written and says more was.
func (s *transcript) Write(written []byte) (int, error) {
	s.mu.Lock()
	s.written.Write(written)
	s.mu.Unlock()

	select {
	case s.more <- struct{}{}:
	default:
	}

	return len(written), nil
}

// text is what was written so far, its escapes stripped.
func (s *transcript) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ansi.Strip(s.written.String())
}

// await waits until the transcript says want somewhere past from, and is
// where that ends, failing the test if it never does.
func (s *transcript) await(t *testing.T, from int, want string) int {
	t.Helper()

	deadline := time.After(promptPatience)

	for {
		if text := s.text(); len(text) >= from {
			if at := strings.Index(text[from:], want); at >= 0 {
				return from + at + len(want)
			}
		}

		select {
		case <-s.more:
		case <-deadline:
			t.Fatalf("the run never said %q:\n%s", want, s.text())
		}
	}
}

// typeAt types text at the terminal, failing the test if it cannot.
func typeAt(t *testing.T, keyboard *os.File, text string) {
	t.Helper()

	_, err := keyboard.WriteString(text)
	if err != nil {
		t.Fatalf("typing %q at the terminal: %v", text, err)
	}
}

// awaitMove waits for the world to be sent a transition, failing the test if
// none is sent in time or the prompts refuse an answer first.
func awaitMove(t *testing.T, moved *world, said *transcript) []string {
	t.Helper()

	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()

	deadline := time.After(promptPatience)

	for {
		if calls := moved.asked("transition PROJ"); len(calls) > 0 {
			return calls
		}

		if strings.Contains(said.text(), "needs a value") {
			t.Fatalf("the prompts refused an answer typed in full:\n%s", said.text())
		}

		select {
		case <-poll.C:
		case <-deadline:
			t.Fatalf("no move was sent once the prompts were answered:\n%s", said.text())
		}
	}
}

// The prompts run on the terminal the interface runs on, handed over through
// Bubble Tea as the editor is: the interface draws until enter, the prompts
// ask on the terminal as it was before the interface took it, and the
// interface takes it back to send the move.
func TestAnAccessibleFormAsksOnTheTerminalTheInterfaceRunsOn(t *testing.T) {
	t.Parallel()

	// Arrange
	keyboard, terminal := ptytest.Open(t)
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	model := sized(t, tui.New(resolving.cfg, nil, resolving.deps()).WithAccessibleForms(), 120, 40)
	picking := typing(t, drain(t, model, model.Init()), "t")
	said := newTranscript()

	// What the run writes to the terminal is read off it as it is written,
	// as the screen a user looks at would be: the terminal holds only so much
	// unread before the run's writes wait for room.
	go func() { _, _ = io.Copy(said, keyboard) }()

	ctx, cancel := context.WithTimeout(t.Context(), 4*promptPatience)
	defer cancel()

	ended := make(chan error, 1)

	go func() {
		_, err := tui.Run(ctx, picking, terminal, terminal)
		ended <- err
	}()

	// Act: choose the move once the interface draws on the terminal, and
	// answer each prompt as it is asked
	said.await(t, 0, "Change status")
	typeAt(t, keyboard, "\r")
	asked := said.await(t, 0, "Enter a number between 1 and 2")
	typeAt(t, keyboard, "2\n")
	said.await(t, asked, "Root cause (2 of 2)")
	typeAt(t, keyboard, "nil token\n")
	calls := awaitMove(t, resolving, said)

	// Assert: the move carries what was typed at the terminal
	if len(calls) != 1 || calls[0] != resolvedNilToken {
		t.Errorf("transition calls = %q, want %q", calls, resolvedNilToken)
	}

	// Act: quit, the interface holding the terminal again
	typeAt(t, keyboard, "\x03")

	// Assert: the run ends cleanly
	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("the run ended with %v, want it to quit cleanly", err)
		}
	case <-time.After(promptPatience):
		t.Fatal("the run did not end once ctrl+c was typed")
	}
}
