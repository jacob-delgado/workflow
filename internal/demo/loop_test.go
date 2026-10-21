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

	// Assert: the branch is made for it
	requireScreen(t, demo.screen(), "fix/PROJ-412-redact-tokens-in-the-request-log")

	// Act: stage everything and commit it
	demo.press("3", "a", "c")
	demo.typed("redact tokens in the request log")
	demo.press("enter")

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
