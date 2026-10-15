// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

// Whether a terminal echoes is read from its settings, which Linux names its
// own way, so this test of a secret builds on Linux alone.

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// awaitEchoOff waits until the terminal no longer echoes what is typed at it,
// as a run asking for a secret makes it.
func (a *askingTerminal) awaitEchoOff(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(runPatience)

	for {
		settings, err := unix.IoctlGetTermios(int(a.terminal.Fd()), unix.TCGETS)
		if err != nil {
			t.Fatalf("reading the terminal's settings: %v", err)
		}

		if settings.Lflag&unix.ECHO == 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("the terminal still echoed after %v: Run never asked for a secret", runPatience)
		}

		time.Sleep(time.Millisecond)
	}
}

func TestRunAsksForASecretWithoutShowingIt(t *testing.T) {
	// Arrange
	var value string

	asking := onATerminal(t)
	wait := running(t, quietinput.New(baseTitle, "", &value).Secret())

	// Act
	asking.awaitEchoOff(t)
	asking.typeAt(t, baseBranch)

	err := wait()

	// Assert
	shown := asking.written(t)
	if err != nil || value != baseBranch || !strings.Contains(shown, baseTitle) || strings.Contains(shown, baseBranch) {
		t.Errorf("Run kept %q with %v, asking:\n%s\nwant %q kept under its title and never shown",
			value, err, shown, baseBranch)
	}
}
