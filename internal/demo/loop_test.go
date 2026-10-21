// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"
	"time"
)

// ciPasses is long enough on the demo's clock for every check to pass.
const ciPasses = 5 * time.Second

func TestTheRecordedLoopRunsFromAnIssueToAnAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := opened(t)

	// Act: start work on the selected issue
	demo.press("1", "b", "enter")

	// Assert: the branch is made for it, and the move into progress offered
	requireScreen(t, demo.screen(), "fix/PROJ-412-redact-tokens-in-the-request-log", "Start Progress → In Progress")

	// Act: move it into progress
	demo.press("enter")

	// Assert: Jira has it in progress
	requireScreen(t, demo.screen(), "PROJ-412 is now In Progress")

	// Act: stage everything and commit it
	demo.commitTheRedaction()

	// Assert: the commit is on the branch, and nothing is left to stage
	requireScreen(t, demo.screen(), "4f2c9e1 fix(config): redact tokens in the request log", "Nothing has changed.")

	// Act: push the branch
	demo.press("2", "P", "enter")

	// Assert: origin has it
	requireScreen(t, demo.screen(), "upstream  pushed")

	// Act: open its pull request on the base proposed
	demo.press("4", "n", "enter")

	// Assert: its body starts from the repository's template, and its
	// reviewers are the code owners of its changes
	requireScreen(t, demo.screen(), "template pull_request_template (1 of 1)", "## How it was tested",
		"lee.chen, sam.ortiz, acme/platform")

	// Act: open it with the title proposed
	demo.press("enter")

	// Assert: CI runs, and the announcement waits for it
	requireScreen(t, demo.screen(), "#128 fix(config): redact tokens in the request log", "running (1 of 4 finished)",
		"announces to #dev-workflow once CI passes")

	// Act: let CI finish
	demo.wait(ciPasses)

	// Assert: CI passed
	requireScreen(t, demo.screen(), "passed (4 of 4 finished)")

	// Act: look at Slack
	demo.press("5")

	// Assert: the pull request was announced, and the store remembers it
	requireScreen(t, demo.screen(), "state  ● announced")

	announced := demo.world.Deps().Store.Announced(t.Context())
	if len(announced) != 1 || announced[0].Pull != 128 {
		t.Errorf("the store holds %+v announced, want #128", announced)
	}
}

func TestStartingWorkOffersToMoveTheIssueIntoProgress(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := opened(t)

	// Act: start work on the issue, which no one has started
	demo.press("1", "b", "enter")

	// Assert: the move into progress is offered, and not yet made
	requireScreen(t, demo.screen(), "Start Progress → In Progress")

	if detail, _ := demo.world.Deps().Jira.Issue(t.Context(), worked); detail.Issue.Status != toDo {
		t.Fatalf("PROJ-412 is %s before the move is confirmed, want To Do", detail.Issue.Status)
	}

	// Act: confirm the move
	demo.press("enter")

	// Assert: PROJ-412 is in progress
	if detail, _ := demo.world.Deps().Jira.Issue(t.Context(), worked); detail.Issue.Status != inProgress {
		t.Errorf("PROJ-412 is %s once the move is confirmed, want In Progress", detail.Issue.Status)
	}
}
