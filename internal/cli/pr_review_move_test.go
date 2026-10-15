// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestPRReportsAFailedMove(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		link    int
		reports []string
	}{
		"the move fails": {link: http.StatusCreated, reports: []string{"moving PROJ-2 to " + clitest.StatusInReview}},
		"both fail": {
			link:    http.StatusInternalServerError,
			reports: []string{"linking #7 on PROJ-2", "moving PROJ-2 to " + clitest.StatusInReview},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo, _ := reviewRepoAnswering(t, reviewMoves,
				jiraAnswers{link: tt.link, transition: http.StatusInternalServerError})

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")

			// Assert
			wantExit(t, err, 1)

			for _, report := range tt.reports {
				if err == nil || !strings.Contains(err.Error(), report) {
					t.Errorf("pr = %v, want it to report %q", err, report)
				}
			}
		})
	}
}

func TestPRMovesTheIssueToTheReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, applied := reviewRepo(t, reviewMoves)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	// What was opened is the artifact; the move that follows is said about it.
	moved := "Moved PROJ-2 to " + clitest.StatusInReview
	if !strings.Contains(printed.Stdout, "Opened #7") ||
		!strings.Contains(printed.Stderr, moved) || strings.Contains(printed.Stdout, moved) {
		t.Errorf("pr did not open on stdout and move the issue on stderr:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}

	// The offer is chosen by the destination status name, so the In Review
	// transition (31) is applied — not the first-listed In Progress (11).
	if applied.Applied() != 1 || !strings.Contains(applied.SentBody(), `"id":"31"`) {
		t.Errorf("applied = %d, body = %q; want the In Review transition sent once",
			applied.Applied(), applied.SentBody())
	}
}

func TestPRLeavesTheIssueWhenTheReviewMoveIsDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, applied := reviewRepo(t, reviewMoves)

	// Act
	// Yes to opening the pull request and to linking it, no to moving the issue.
	printed, err := clitest.RunStreams(t, repo, clitest.Scripted([]string{"y", "y", "n"}, nil), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	const left = "Left PROJ-2 as it is."
	if !strings.Contains(printed.Stdout, "Opened #7") ||
		!strings.Contains(printed.Stderr, left) || strings.Contains(printed.Stdout, left) {
		t.Errorf("pr did not open on stdout and leave the issue on stderr:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}

	if applied.Applied() != 0 {
		t.Errorf("a declined move still applied a transition (%d)", applied.Applied())
	}
}

func TestPRDoesNotMoveToAReviewStatusItCannotComplete(t *testing.T) {
	t.Parallel()

	// A workflow that never reaches In Review, and one that gates it behind a
	// required field the command cannot fill: both leave the issue untouched.
	noReview := `{"transitions":[{"id":"11","name":"Start Progress",` +
		`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}}]}`
	needsField := `{"transitions":[{"id":"31","name":"Start Review",` +
		`"to":{"name":"` + clitest.StatusInReview + `","statusCategory":{"key":"indeterminate"}},` +
		`"fields":{"resolution":{"required":true,"hasDefaultValue":false,"name":"Resolution",` +
		`"schema":{"type":"string"}}}}]}`

	cases := map[string]string{
		"the status is not offered":    noReview,
		"the transition needs a field": needsField,
	}

	for name, transitions := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo, applied := reviewRepo(t, transitions)

			// Act
			output, err := clitest.Run(t, repo, "pr", "--yes")
			// Assert
			if err != nil {
				t.Fatalf("pr --yes: %v (%s)", err, output)
			}

			if !strings.Contains(output, "Opened #7") {
				t.Errorf("the pull request should still open:\n%s", output)
			}

			if strings.Contains(output, "Moved PROJ-2") || applied.Applied() != 0 {
				t.Errorf("moved the issue to a review status it could not complete:\n%s", output)
			}
		})
	}
}
