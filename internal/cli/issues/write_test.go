// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package issues_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// trackerDir is a directory configured for a fresh trackerJira answering
// answers, and that Jira's record of what it was asked.
func trackerDir(t *testing.T, answers trackerAnswers) (string, *trackerAsked) {
	t.Helper()

	baseURL, asked := trackerJira(t, answers)
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	return dir, asked
}

func TestTransitionAppliesAFieldlessMove(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, clitest.StatusInReview, "--yes",
	)
	// Assert
	if err != nil {
		t.Fatalf("transition --yes: %v (%+v)", err, printed)
	}

	if asked.Applied() != 1 || !strings.Contains(asked.SentBody(), `"id":"31"`) {
		t.Errorf("applied %d, last sent %q; want transition 31 once", asked.Applied(), asked.SentBody())
	}

	if !strings.Contains(printed.Stdout, "Move PROJ-7 to In Review (Start Review)") ||
		!strings.Contains(printed.Stderr, "Moved PROJ-7 to In Review.") {
		t.Errorf("transition previewed or reported wrongly:\nstdout:\n%s\nstderr:\n%s", printed.Stdout, printed.Stderr)
	}
}

func TestTransitionMatchesTheStatusWhateverItsCase(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "in REVIEW", "--yes")

	// Assert
	if err != nil || asked.Applied() != 1 || !strings.Contains(asked.SentBody(), `"id":"31"`) {
		t.Errorf("transition = %v, applied %d (%q); want the move to In Review", err, asked.Applied(), asked.SentBody())
	}
}

func TestTransitionTrimsTheStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "  "+clitest.StatusInReview+" ", "--yes",
	)

	// Assert
	if err != nil || asked.Applied() != 1 || !strings.Contains(asked.SentBody(), `"id":"31"`) {
		t.Errorf("transition = %v, applied %d (%q); want the move to In Review", err, asked.Applied(), asked.SentBody())
	}
}

func TestTransitionRefusesABlankStatusBeforeAskingJira(t *testing.T) {
	t.Parallel()

	// Arrange
	reached := new(atomic.Bool)
	server := clitest.JiraServer(t, http.StatusOK, trackerMoves, reached)
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(server.URL, ""))

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "  ", "--yes")

	// Assert
	wantExit(t, err, 2)

	if reached.Load() {
		t.Errorf("transition to a blank status = %v, and Jira was asked; want it refused first", err)
	}
}

func TestTransitionRefusesAMoveThatNeedsFieldsNamingThem(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "Done", "--yes")

	// Assert
	wantExit(t, err, 4)

	if err == nil || !strings.Contains(err.Error(), "Resolution") || !strings.Contains(err.Error(), "Fix Version/s") ||
		asked.Applied() != 0 {
		t.Errorf("transition to Done = %v, applied %d; want the fields named and nothing applied", err, asked.Applied())
	}
}

func TestTransitionNeedingFieldsSaysWhereAFormAsksForThem(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, _ := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "Done", "--yes")

	// Assert
	want := "; change its status with t in the interface or Change status on the web, whose forms ask for them"
	if err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("transition to Done = %v, want it to end %q", err, want)
	}
}

func TestTransitionRefusesAStatusNotOfferedNamingThoseThatAre(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, "Blocked", "--yes")

	// Assert
	wantExit(t, err, 4)

	if err == nil || !strings.Contains(err.Error(), "In Progress, In Review, Done") || asked.Applied() != 0 {
		t.Errorf("transition to Blocked = %v, applied %d; want the statuses offered named", err, asked.Applied())
	}
}

func TestTransitionAsksBeforeMoving(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	var questions []string

	// Act
	printed, err := clitest.RunStreams(
		t, dir, clitest.Answering("n", &questions), "transition", trackedIssue, clitest.StatusInReview,
	)

	// Assert
	if err != nil || len(questions) != 1 || !strings.HasPrefix(questions[0], "Move PROJ-7 to In Review?") ||
		!strings.Contains(printed.Stderr, "Left PROJ-7 as it is.") || asked.Applied() != 0 {
		t.Errorf("transition = %v, asked %q, said %q, applied %d; want one question, declined, nothing moved",
			err, questions, printed.Stderr, asked.Applied())
	}
}

func TestTransitionDryRunMovesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, clitest.StatusInReview, "--dry-run",
	)

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "dry run: would move PROJ-7 to In Review") ||
		asked.Applied() != 0 {
		t.Errorf("transition --dry-run = %v, said %q, applied %d", err, printed.Stderr, asked.Applied())
	}
}

func TestTransitionSaysWhatJiraRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, _ := trackerDir(t, trackerAnswers{writeCode: http.StatusBadRequest})

	// Act
	_, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, clitest.StatusInReview, "--yes",
	)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "moving PROJ-7 to In Review") {
		t.Errorf("transition Jira refused = %v, want the move named", err)
	}
}

func TestTransitionSaysWhyTheMovesCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{movesCode: http.StatusInternalServerError})

	// Act
	_, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "transition", trackedIssue, clitest.StatusInReview, "--yes",
	)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "reading the transitions of PROJ-7") || asked.Applied() != 0 {
		t.Errorf("transition with the moves unread = %v, applied %d", err, asked.Applied())
	}
}

func TestAssignSetsTheAssignee(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "assign", trackedIssue, "bo", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("assign --yes: %v (%+v)", err, printed)
	}

	assigned := asked.Of(writeAssign)
	if len(assigned) != 1 || !strings.Contains(assigned[0].Body, `"name":"bo"`) {
		t.Errorf("assignments = %+v, want PROJ-7 assigned to bo once", assigned)
	}

	if !strings.Contains(printed.Stdout, "Assign PROJ-7 to bo") ||
		!strings.Contains(printed.Stderr, "Assigned PROJ-7 to bo.") {
		t.Errorf("assign previewed or reported wrongly:\nstdout:\n%s\nstderr:\n%s", printed.Stdout, printed.Stderr)
	}
}

func TestAssignRefusesABlankUsername(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "assign", trackedIssue, "  ", "--yes")

	// Assert
	wantExit(t, err, 2)

	if got := asked.Of(writeAssign); len(got) != 0 {
		t.Errorf("a blank username was sent: %+v", got)
	}
}

func TestAssignAsksBeforeAssigning(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	var questions []string

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.Answering("n", &questions), "assign", trackedIssue, "bo")

	// Assert
	if err != nil || len(questions) != 1 || !strings.HasPrefix(questions[0], "Assign PROJ-7 to bo?") ||
		!strings.Contains(printed.Stderr, "Not assigned.") || len(asked.Of(writeAssign)) != 0 {
		t.Errorf("assign = %v, asked %q, said %q; want one question, declined, nothing assigned",
			err, questions, printed.Stderr)
	}
}

func TestAssignSaysWhatJiraRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, _ := trackerDir(t, trackerAnswers{writeCode: http.StatusBadRequest})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "assign", trackedIssue, "bo", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "assigning PROJ-7 to bo") {
		t.Errorf("assign Jira refused = %v, want the assignment named", err)
	}
}

func TestAssignDryRunAssignsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "assign", trackedIssue, "bo", "--dry-run")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "dry run: would assign PROJ-7 to bo") ||
		len(asked.Of(writeAssign)) != 0 {
		t.Errorf("assign --dry-run = %v, said %q, assigned %+v", err, printed.Stderr, asked.Of(writeAssign))
	}
}

func TestWorklogLogsTheDurationWithItsNote(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t),
		"worklog", trackedIssue, "2h", "--comment", "pairing", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("worklog --yes: %v (%+v)", err, printed)
	}

	logged := asked.Of(writeWorklog)
	if len(logged) != 1 || !strings.Contains(logged[0].Body, `"timeSpent":"2h"`) ||
		!strings.Contains(logged[0].Body, `"comment":"pairing"`) {
		t.Errorf("worklogs = %+v, want 2h with its note, once", logged)
	}

	if !strings.Contains(printed.Stdout, "Log 2h on PROJ-7\n  pairing") ||
		!strings.Contains(printed.Stderr, "Logged 2h on PROJ-7.") {
		t.Errorf("worklog previewed or reported wrongly:\nstdout:\n%s\nstderr:\n%s", printed.Stdout, printed.Stderr)
	}
}

func TestWorklogWithoutANotePreviewsTheDurationAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, _ := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "worklog", trackedIssue, "2h", "--yes")

	// Assert
	if err != nil || printed.Stdout != "Log 2h on PROJ-7\n" {
		t.Errorf("worklog = %v, previewed %q, want the duration alone", err, printed.Stdout)
	}
}

func TestWorklogRefusesWhatCannotBeLogged(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		issueKey string
		spent    string
	}{
		"a forge issue":    {issueKey: "42", spent: "1h"},
		"a blank duration": {issueKey: trackedIssue, spent: " "},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir, asked := trackerDir(t, trackerAnswers{})

			// Act
			_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "worklog", tt.issueKey, tt.spent, "--yes")

			// Assert
			wantExit(t, err, 2)

			if got := asked.Of(writeWorklog); len(got) != 0 {
				t.Errorf("work was logged: %+v", got)
			}
		})
	}
}

func TestWorklogWithoutJiraSaysToSetItUp(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{}`)

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "worklog", trackedIssue, "1h", "--yes")

	// Assert
	wantExit(t, err, 3)

	if err == nil || !strings.Contains(err.Error(), "jira.base_url") {
		t.Errorf("worklog with no Jira = %v, want it to say to set jira.base_url", err)
	}
}

func TestWorklogSaysWhatJiraRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, _ := trackerDir(t, trackerAnswers{writeCode: http.StatusBadRequest})

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "worklog", trackedIssue, "2h", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "logging 2h on PROJ-7") {
		t.Errorf("worklog Jira refused = %v, want the log named", err)
	}
}

func TestWorklogDryRunLogsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir, asked := trackerDir(t, trackerAnswers{})

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "worklog", trackedIssue, "2h", "--dry-run")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "dry run: would log 2h on PROJ-7") ||
		len(asked.Of(writeWorklog)) != 0 {
		t.Errorf("worklog --dry-run = %v, said %q, logged %+v", err, printed.Stderr, asked.Of(writeWorklog))
	}
}
