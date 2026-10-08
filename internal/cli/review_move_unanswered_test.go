// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestPRStopsAtTheMoveWhenNothingCanAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// The open and the link are answered, then the input ends before the move.
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	_, err := runStreams(t, repo, cli.Prompt{Line: answersThenEnds("y", "y")}, "pr")

	// Assert
	if len(writes.of(writeLink)) != 1 || writes.applied() != 0 {
		t.Errorf("pr made %d links and %d moves, want the link made and no move",
			len(writes.of(writeLink)), writes.applied())
	}

	if err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Errorf("pr = %v, want it to stop at the move, saying how to go ahead without asking", err)
	}

	wantExit(t, err, 2)
}
