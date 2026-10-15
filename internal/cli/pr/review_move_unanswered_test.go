// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

func TestPRStopsAtTheMoveWhenNothingCanAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// The open and the link are answered, then the input ends before the move.
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	_, err := clitest.RunStreams(t, repo, cmdkit.Prompt{Line: clitest.AnswersThenEnds("y", "y")}, "pr")

	// Assert
	if len(writes.Of(clitest.WriteLink)) != 1 || writes.Applied() != 0 {
		t.Errorf("pr made %d links and %d moves, want the link made and no move",
			len(writes.Of(clitest.WriteLink)), writes.Applied())
	}

	if err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Errorf("pr = %v, want it to stop at the move, saying how to go ahead without asking", err)
	}

	wantExit(t, err, 2)
}
