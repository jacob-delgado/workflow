// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

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
)

// shownSoFar is a stand-in terminal for a run whose live line is drawn from
// goroutines of its own: what has been written to it so far.
type shownSoFar struct {
	mu      sync.Mutex
	written strings.Builder
}

var _ io.Writer = (*shownSoFar)(nil)

// Write keeps p.
func (s *shownSoFar) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written.Write(p) //nolint:wrapcheck // a builder's write never fails
}

// String is everything written so far.
func (s *shownSoFar) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written.String()
}

// releaseOnceShown lets gate's CI pass once terminal shows text, or once a
// wait has had its patience, so the command never hangs on a test that
// fails.
func releaseOnceShown(t *testing.T, terminal *shownSoFar, gate, text string) {
	t.Helper()

	for deadline := time.Now().Add(answerPatience); time.Now().Before(deadline); {
		if strings.Contains(ansi.Strip(terminal.String()), text) {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

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
	fakeGh(t, ghResponses{pulls: openPull("Add login"), statusGate: gate})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	terminal := new(shownSoFar)
	released := make(chan struct{})

	go func() {
		defer close(released)

		releaseOnceShown(t, terminal, gate, "─ 0 of 1")
	}()

	// Act
	err := cli.Execute(strings.Fields("announce --when-green --yes"), &stdout, terminal, atTerminal(t),
		environmentFor(t, place{dir: repo, home: t.TempDir()}))

	// Assert
	<-released

	shown := terminal.String()
	if err != nil || posts.Load() != 1 || !markedLive(shown) || !strings.Contains(ansi.Strip(shown), "CI on #7") {
		t.Errorf("announce --when-green = %v after %d posts, showing %q; want a live line of CI on #7", err,
			posts.Load(), shown)
	}

	if !strings.Contains(shown, "Waiting for CI on #7 to pass") || !strings.Contains(shown, "Announced to") {
		t.Errorf("showing %q, want the wait's lines kept above the live line", shown)
	}

	if strings.LastIndex(shown, eraseBelow) < strings.LastIndex(shown, "─ 0 of 1") {
		t.Errorf("showing %q, want the live line erased once CI passed", shown)
	}
}
