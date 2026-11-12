// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestPRReportsAPullRequestOpenedWithoutItsReviewersAsOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	// C19: the pull request opens, and the forge asks the request for its
	// reviewers to wait.
	clitest.FakeGh(t, clitest.GhResponses{ReviewersRateLimited: true})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--reviewer", "ana")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "Opened #7") {
		t.Fatalf("pr = %v, printed %+v; want #7 reported opened, exiting 0", err, printed)
	}

	if !strings.Contains(printed.Stderr, "WARN Opened, but could not add every reviewer, assignee or label") {
		t.Errorf("stderr = %q, want a warning that not everyone was added", printed.Stderr)
	}
}

func TestPRAnnounceAnnouncesAPullRequestWhoseLinkFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// C18: the pull request opens, Jira refuses its link, and --announce
	// still waits for CI and announces it.
	var posts atomic.Int32

	clitest.FakeGh(t, openingThenOpen())
	baseURL, _ := reviewJiraAnswering(t, reviewMoves,
		jiraAnswers{link: http.StatusInternalServerError, transition: http.StatusNoContent})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t",`+
		`"review_status":"`+clitest.StatusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"`+webhook(t, &posts)+`"},"timing":{"ci_interval":"10ms"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stderr, "WARN Could not link #7 on PROJ-2") {
		t.Errorf("pr --announce = %v, posted %d, said %q; want the failed link warned of, then one post, exiting 0",
			err, posts.Load(), printed.Stderr)
	}
}
