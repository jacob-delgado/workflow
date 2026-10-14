// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// A run interrupted mid-subprocess exits as interrupted. The interrupt is a
// real one, raised on this process, which every run beside it would hear too,
// so this file's tests run alone, as .golangci.yml's paralleltest exclusion
// for it says.

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAnInterruptedGitCommandExitsAsInterrupted(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	// A git that raises Ctrl+C on the process running it and then hangs until it
	// is stopped: the interrupt lands mid-subprocess, where what comes back is
	// git's own exit status rather than the interrupt.
	writeExecutable(t, filepath.Join(programsOf(t), "git"), "#!/bin/sh\nkill -INT $PPID\nexec sleep 5\n", 0o755)

	// Act
	_, err := run(t, repo, "status")

	// Assert
	wantExit(t, err, 130)
}

func TestAWaitForCIStoppedByCtrlCSaysNothingWasAnnounced(t *testing.T) {
	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login"), interruptsStatus: true})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, atTerminal(t), "announce", "--when-green", "--yes")

	// Assert
	wantExit(t, err, 130)

	if !strings.Contains(printed.stderr, "Stopped waiting; nothing was announced.") || posts.Load() != 0 {
		t.Errorf("announce --when-green stopped, posting %d, said %q; want nothing announced, and that said",
			posts.Load(), printed.stderr)
	}
}
