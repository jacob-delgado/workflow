// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// A wait for CI interrupted mid-subprocess says nothing was announced. The
// interrupt is a real one, raised on this process, which every run beside it
// would hear too, so this file's tests run alone, as .golangci.yml's
// paralleltest exclusion for it says.

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestAWaitForCIStoppedByCtrlCSaysNothingWasAnnounced(t *testing.T) {
	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), InterruptsStatus: true})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(clitest.Webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.AtTerminal(t), "announce", "--when-green", "--yes")

	// Assert
	wantExit(t, err, 130)

	if !strings.Contains(printed.Stderr, "Stopped waiting; nothing was announced.") || posts.Load() != 0 {
		t.Errorf("announce --when-green stopped, posting %d, said %q; want nothing announced, and that said",
			posts.Load(), printed.Stderr)
	}
}
