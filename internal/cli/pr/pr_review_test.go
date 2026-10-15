// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

// reviewMoves offers In Progress then In Review, both indeterminate, so a test
// proves the offer is chosen by the destination name (In Review) and not by the
// first transition listed.
const reviewMoves = `{"transitions":[` +
	`{"id":"11","name":"Start Progress",` +
	`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}},` +
	`{"id":"31","name":"Start Review",` +
	`"to":{"name":"` + clitest.StatusInReview + `","statusCategory":{"key":"indeterminate"}},"fields":{}}]}`

// reviewJira serves the requests `pr` makes once a pull request is open: the
// issue read during compose, adding the pull request's link, reviewMoves as the
// transitions list, and applying one. It records every link and transition, so
// a test can prove what was sent — or that nothing was.
func reviewJira(t *testing.T) (string, *clitest.JiraWrites) {
	t.Helper()

	return reviewJiraAnswering(t, reviewMoves, jiraAccepts())
}

// jiraAnswers are the statuses a fake Jira answers its two writes with.
type jiraAnswers struct {
	link       int
	transition int
}

// jiraAccepts answers both writes as Jira does when it makes them.
func jiraAccepts() jiraAnswers {
	return jiraAnswers{link: http.StatusCreated, transition: http.StatusNoContent}
}

// reviewJiraAnswering is reviewJira answering its writes with answers, so a test
// can have Jira refuse either one.
func reviewJiraAnswering(t *testing.T, transitions string, answers jiraAnswers) (string, *clitest.JiraWrites) {
	t.Helper()

	writes := &clitest.JiraWrites{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(request.URL.Path, "/remotelink") && request.Method == http.MethodPost:
			writes.Record(clitest.WriteLink, request)
			writer.WriteHeader(answers.link)
		case strings.HasSuffix(request.URL.Path, "/transitions") && request.Method == http.MethodPost:
			writes.Record(clitest.WriteTransition, request)
			writer.WriteHeader(answers.transition)
		case strings.HasSuffix(request.URL.Path, "/transitions"):
			_, _ = writer.Write([]byte(transitions))
		default:
			_, _ = writer.Write([]byte(clitest.IssueFixture("PROJ-2", "Bug", "thing")))
		}
	}))
	t.Cleanup(server.Close)

	return server.URL, writes
}

// reviewRepo is a published GitHub branch with no pull request yet, configured to
// offer clitest.StatusInReview once one is opened, and its fake Jira's record of writes.
func reviewRepo(t *testing.T, transitions string) (string, *clitest.JiraWrites) {
	t.Helper()

	return reviewRepoAnswering(t, transitions, jiraAccepts())
}

// reviewRepoAnswering is reviewRepo whose Jira answers its writes with answers.
func reviewRepoAnswering(t *testing.T, transitions string, answers jiraAnswers) (string, *clitest.JiraWrites) {
	t.Helper()

	clitest.FakeGh(t, clitest.GhResponses{})
	baseURL, writes := reviewJiraAnswering(t, transitions, answers)
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t",`+
		`"review_status":"`+clitest.StatusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo, writes
}

func TestPRLinksThePullOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	// The link goes on the branch's issue, under the pull request's address and
	// title.
	links := writes.Of(clitest.WriteLink)
	if len(links) != 1 || !strings.HasSuffix(links[0].Path, "/issue/PROJ-2/remotelink") ||
		!strings.Contains(links[0].Body, `"url":"https://github.com/owner/repo/pull/7"`) ||
		!strings.Contains(links[0].Body, `"title":"work"`) {
		t.Errorf("links sent = %+v, want the opened pull request linked once on PROJ-2", links)
	}

	// As in the interface, the link is offered before the status move.
	if kinds := writes.Kinds(); !slices.Equal(kinds, []string{clitest.WriteLink, clitest.WriteTransition}) {
		t.Errorf("Jira was written %q, want the link and then the move", kinds)
	}

	if !strings.Contains(printed.Stderr, "Linked #7 on PROJ-2.\n") {
		t.Errorf("pr does not say it linked the pull request:\nstderr:\n%s", printed.Stderr)
	}
}

func TestPRReportsAFailedLinkAndStillMovesTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, writes := reviewRepoAnswering(t, reviewMoves,
		jiraAnswers{link: http.StatusInternalServerError, transition: http.StatusNoContent})

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")

	// Assert
	wantExit(t, err, 1)

	if err == nil || !strings.Contains(err.Error(), "linking #7 on PROJ-2") {
		t.Errorf("pr = %v, want the failed link reported", err)
	}

	// The link failing is said as it happens, and the status move is still made.
	if !strings.Contains(printed.Stderr, "WARN Could not link #7 on PROJ-2") ||
		!strings.Contains(printed.Stderr, "Moved PROJ-2 to "+clitest.StatusInReview+".\n") || writes.Applied() != 1 {
		t.Errorf("pr did not say the link failed and still move the issue (%d moves):\nstderr:\n%s",
			writes.Applied(), printed.Stderr)
	}
}

func TestPRAsksToLinkThenToMove(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, _ := reviewRepo(t, reviewMoves)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("y", &asked), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	// Each offer names the pull request and the issue it would change.
	if len(asked) != 3 || !strings.HasPrefix(asked[1], "Link #7 on PROJ-2?") ||
		!strings.HasPrefix(asked[2], "Move PROJ-2 to "+clitest.StatusInReview+"?") {
		t.Errorf("pr asked %q, want the open, then to link #7 on PROJ-2, then to move PROJ-2", asked)
	}
}

func TestPRLeavesTheIssueUnlinkedWhenDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, writes := reviewRepo(t, reviewMoves)

	// Act
	// Yes to opening the pull request, no to linking it, yes to the move.
	printed, err := clitest.RunStreams(t, repo, clitest.Scripted([]string{"y", "n", "y"}, nil), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	links := writes.Of(clitest.WriteLink)
	if len(links) != 0 || !strings.Contains(printed.Stderr, "Left PROJ-2 unlinked.") {
		t.Errorf("a declined link still sent %+v, or was not said:\nstderr:\n%s", links, printed.Stderr)
	}

	if writes.Applied() != 1 {
		t.Errorf("declining the link skipped the move (%d moves), want it still offered", writes.Applied())
	}
}

func TestPRStopsAtTheLinkWhenNothingCanAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// The open is answered, then the input ends before the link question.
	repo, writes := reviewRepo(t, reviewMoves)
	answers := []string{"y"}

	var asked int

	ending := cmdkit.Prompt{Line: func(string) (string, error) {
		asked++
		if asked > len(answers) {
			return "", io.EOF
		}

		return answers[asked-1], nil
	}}

	// Act
	_, err := clitest.RunStreams(t, repo, ending, "pr")

	// Assert
	wantExit(t, err, 2)

	// Nothing can answer the move either, so it is not asked, and the refusal
	// is said once.
	if asked != 2 || writes.Applied() != 0 || err == nil || strings.Count(err.Error(), "--yes") != 1 {
		t.Errorf("pr asked %d questions, moved %d times and returned %v; want it to stop at the link",
			asked, writes.Applied(), err)
	}
}

func TestPRLinksNothingForABranchWithoutAnIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	baseURL, writes := reviewJira(t)
	repo := clitest.GithubRepo(t, "chore/cleanup")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t",`+
		`"review_status":"`+clitest.StatusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	if kinds := writes.Kinds(); len(kinds) != 0 {
		t.Errorf("a branch that names no issue still wrote to Jira: %q", kinds)
	}
}

func TestPRDoesNotLinkAForgeIssueNumberOnJira(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch names the forge's issue 42, not a Jira one: Jira would refuse
	// the link, or read 42 as the id of an unrelated issue.
	clitest.FakeGh(t, clitest.GhResponses{})
	baseURL, writes := reviewJira(t)
	repo := clitest.GithubRepo(t, "fix/42-typo")
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	if links := writes.Of(clitest.WriteLink); len(links) != 0 {
		t.Errorf("a forge issue number was linked on Jira: %+v", links)
	}
}

func TestPRYesHelpNamesEverythingItAnswers(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "pr", "--help")
	// Assert
	if err != nil {
		t.Fatalf("pr --help: %v", err)
	}

	help := strings.Join(strings.Fields(printed.Stdout), " ")
	for _, answered := range []string{"push the branch", "link it on the issue", "review status"} {
		if !strings.Contains(help, answered) {
			t.Errorf("pr's help does not say --yes answers %q:\n%s", answered, printed.Stdout)
		}
	}
}
