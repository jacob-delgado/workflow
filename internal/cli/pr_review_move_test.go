// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestPRReportsAFailedMove(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		link    int
		reports []string
	}{
		"the move fails": {link: http.StatusCreated, reports: []string{"moving PROJ-2 to " + statusInReview}},
		"both fail": {
			link:    http.StatusInternalServerError,
			reports: []string{"linking #7 on PROJ-2", "moving PROJ-2 to " + statusInReview},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo, _ := reviewRepoAnswering(t, reviewMoves,
				jiraAnswers{link: tt.link, transition: http.StatusInternalServerError})

			// Act
			_, err := runStreams(t, repo, unusedPrompt(t), "pr", "--yes")

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
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	// What was opened is the artifact; the move that follows is said about it.
	moved := "Moved PROJ-2 to " + statusInReview
	if !strings.Contains(printed.stdout, "Opened #7") ||
		!strings.Contains(printed.stderr, moved) || strings.Contains(printed.stdout, moved) {
		t.Errorf("pr did not open on stdout and move the issue on stderr:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}

	// The offer is chosen by the destination status name, so the In Review
	// transition (31) is applied — not the first-listed In Progress (11).
	if applied.applied() != 1 || !strings.Contains(applied.sentBody(), `"id":"31"`) {
		t.Errorf("applied = %d, body = %q; want the In Review transition sent once",
			applied.applied(), applied.sentBody())
	}
}

func TestPRLeavesTheIssueWhenTheReviewMoveIsDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, applied := reviewRepo(t, reviewMoves)

	// Act
	// Yes to opening the pull request and to linking it, no to moving the issue.
	printed, err := runStreams(t, repo, scripted([]string{"y", "y", "n"}, nil), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	const left = "Left PROJ-2 as it is."
	if !strings.Contains(printed.stdout, "Opened #7") ||
		!strings.Contains(printed.stderr, left) || strings.Contains(printed.stdout, left) {
		t.Errorf("pr did not open on stdout and leave the issue on stderr:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}

	if applied.applied() != 0 {
		t.Errorf("a declined move still applied a transition (%d)", applied.applied())
	}
}

func TestPRDoesNotMoveToAReviewStatusItCannotComplete(t *testing.T) {
	t.Parallel()

	// A workflow that never reaches In Review, and one that gates it behind a
	// required field the command cannot fill: both leave the issue untouched.
	noReview := `{"transitions":[{"id":"11","name":"Start Progress",` +
		`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}}]}`
	needsField := `{"transitions":[{"id":"31","name":"Start Review",` +
		`"to":{"name":"` + statusInReview + `","statusCategory":{"key":"indeterminate"}},` +
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
			output, err := run(t, repo, "pr", "--yes")
			// Assert
			if err != nil {
				t.Fatalf("pr --yes: %v (%s)", err, output)
			}

			if !strings.Contains(output, "Opened #7") {
				t.Errorf("the pull request should still open:\n%s", output)
			}

			if strings.Contains(output, "Moved PROJ-2") || applied.applied() != 0 {
				t.Errorf("moved the issue to a review status it could not complete:\n%s", output)
			}
		})
	}
}
