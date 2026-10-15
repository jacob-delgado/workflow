// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// shownSoFar is a stand-in terminal for a run whose live line is drawn from
// goroutines of its own: what has been written to it so far, and a word each
// time more is.
type shownSoFar struct {
	mu      sync.Mutex
	written strings.Builder
	more    chan struct{}
}

var _ io.Writer = (*shownSoFar)(nil)

// newShownSoFar is a stand-in terminal nothing has been written to yet.
func newShownSoFar() *shownSoFar {
	return &shownSoFar{more: make(chan struct{}, 1)}
}

// Write keeps what was written, and says more was.
func (s *shownSoFar) Write(written []byte) (int, error) {
	s.mu.Lock()
	s.written.Write(written)
	s.mu.Unlock()

	select {
	case s.more <- struct{}{}:
	default:
	}

	return len(written), nil
}

// String is everything written so far.
func (s *shownSoFar) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written.String()
}

// awaits waits for the write that has the terminal show text, or for a wait's
// patience to run out.
func (s *shownSoFar) awaits(text string) {
	patience := time.After(answerPatience)

	for !strings.Contains(ansi.Strip(s.String()), text) {
		select {
		case <-s.more:
		case <-patience:
			return
		}
	}
}

// releaseOnceShown lets gate's CI pass once terminal shows text, or once a
// wait has had its patience, so the command never hangs on a test that
// fails.
func releaseOnceShown(t *testing.T, terminal *shownSoFar, gate, text string) {
	t.Helper()

	terminal.awaits(text)

	err := os.WriteFile(filepath.Join(gate, "release"), nil, 0o600)
	if err != nil {
		t.Errorf("releasing the CI: %v", err)
	}
}

func TestAnnounceWhenGreenKeepsALiveLineOfCIsProgressOnATerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		posts  atomic.Int32
		stdout bytes.Buffer
	)

	gate := t.TempDir()
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), StatusGate: gate})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(clitest.Webhook(t, &posts)))

	terminal := newShownSoFar()
	released := make(chan struct{})

	go func() {
		defer close(released)

		releaseOnceShown(t, terminal, gate, "─ 0 of 1")
	}()

	// Act
	err := cli.Execute(strings.Fields("announce --when-green --yes"), &stdout, terminal, clitest.AtTerminal(t),
		clitest.EnvironmentFor(t, clitest.Place{Dir: repo, Home: t.TempDir()}))

	// Assert
	<-released

	shown := terminal.String()
	if err != nil || posts.Load() != 1 || !clitest.MarkedLive(shown) || !strings.Contains(ansi.Strip(shown), "CI on #7") {
		t.Errorf("announce --when-green = %v after %d posts, showing %q; want a live line of CI on #7", err,
			posts.Load(), shown)
	}

	if !strings.Contains(shown, "Waiting for CI on #7 to pass") || !strings.Contains(shown, "Announced to") {
		t.Errorf("showing %q, want the wait's lines kept above the live line", shown)
	}

	if strings.LastIndex(shown, clitest.EraseBelow) < strings.LastIndex(shown, "─ 0 of 1") {
		t.Errorf("showing %q, want the live line erased once CI passed", shown)
	}
}

func TestAnnounceWhenGreenDrawsItsLiveLineInASCIIUnderUIASCII(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		posts  atomic.Int32
		stdout bytes.Buffer
	)

	gate := t.TempDir()
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), StatusGate: gate})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(clitest.Webhook(t, &posts)))
	home := t.TempDir()
	clitest.WriteFile(t, home, `{"ui": {"ascii": true}}`)

	terminal := newShownSoFar()
	released := make(chan struct{})

	go func() {
		defer close(released)

		releaseOnceShown(t, terminal, gate, "- 0 of 1")
	}()

	prompt := clitest.UnusedPrompt(t)
	prompt.IsTerminal = func(stream io.Writer) bool { return stream == terminal }

	// Act
	err := cli.Execute(strings.Fields("announce --when-green --yes"), &stdout, terminal, prompt,
		clitest.EnvironmentFor(t, clitest.Place{Dir: repo, Home: home}))

	// Assert
	<-released

	shown := ansi.Strip(terminal.String())
	if err != nil || posts.Load() != 1 || !clitest.DrawnWith(shown, clitest.ASCIIMarks(), "CI on #7") {
		t.Errorf("announce --when-green = %v after %d posts, showing %q; want a live line of CI on #7 in ASCII",
			err, posts.Load(), shown)
	}

	for _, never := range clitest.UnicodeLine() {
		if strings.Contains(shown, never) {
			t.Errorf("showing %q, want no %q", shown, never)
		}
	}
}
