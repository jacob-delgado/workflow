// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestPRQuestionNamesThePush(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		published  bool
		pushRemote string
		want       string
	}{
		"a branch not yet pushed": {want: "Push fix/PROJ-2-thing to origin and open the pull request?"},
		"a branch already pushed": {published: true, want: "Open the pull request?"},
		// A fork's clone may push the branch somewhere of its own; the
		// question says where.
		"a branch pushed to a remote of its own": {
			pushRemote: "fork", want: "Push fix/PROJ-2-thing to fork and open the pull request?",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.FakeGh(t, clitest.GhResponses{})
			repo := githubRepo(t, "fix/PROJ-2-thing")

			if tt.published {
				pretendPushed(t, repo)
			}

			if tt.pushRemote != "" {
				git(t, repo, "config", "branch.fix/PROJ-2-thing.pushRemote", tt.pushRemote)
			}

			clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

			var asked []string

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "pr")

			// Assert
			if err != nil || len(asked) != 1 || !strings.HasPrefix(asked[0], tt.want) {
				t.Errorf("pr = %v, asked %q; want the one question %q", err, asked, tt.want)
			}
		})
	}
}

func TestPROpensAMergeRequestInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlab(t)
	repo := prRepo(t, "fix/PROJ-2-thing")
	git(t, repo, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"gitlab","host":"gitlab.com"}}`)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("y", &asked), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.HasPrefix(asked[0], "Open the merge request?") {
		t.Errorf("pr asked %q, want GitLab's noun in its question", asked)
	}

	if !strings.Contains(printed.Stdout, "Opened !7 "+clitest.GitlabMergeRequest) {
		t.Errorf("pr printed %q, want the merge request marked with GitLab's sigil", printed.Stdout)
	}
}

func TestPRDryRunPreviewsWithoutOpening(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	// The template becomes the body, which the preview shows in full.
	writeRepoFile(t, repo, ".github/pull_request_template.md", "## Checklist\n\n- [ ] Tests\n")
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stdout, "fix/PROJ-2-thing → main") {
		t.Errorf("preview does not name the branch and its base:\n%s", printed.Stdout)
	}

	if !strings.Contains(printed.Stdout, "## Checklist") || !strings.Contains(printed.Stdout, "- [ ] Tests") {
		t.Errorf("preview does not show the body composed from the template:\n%s", printed.Stdout)
	}

	if !strings.Contains(printed.Stderr, "dry run: would push fix/PROJ-2-thing to origin and open") {
		t.Errorf("dry run does not describe the push and open it would do:\n%s", printed.Stderr)
	}
}

func TestPRComposesForABranchWithoutAnIssueKey(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch names no issue, so the pull request is composed without one.
	repo := prRepo(t, "chore/cleanup")

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "chore/cleanup → main") {
		t.Errorf("preview does not name the keyless branch:\n%s", output)
	}
}

func TestPRDryRunPreviewsWithATrackerConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	// With Jira reachable the compose reads the issue to link it; the preview is
	// the same, and nothing is opened.
	var reached atomic.Bool

	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-2", "Bug", "thing"), &reached)
	repo := prRepo(t, "fix/PROJ-2-thing")
	clitest.WriteConfigFor(t, repo, server.URL)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	// The tracker is genuinely read — the summary rides in the body, not the
	// previewed title, so reaching the server is what proves the path ran.
	if !reached.Load() {
		t.Error("compose did not read the configured tracker")
	}

	if !strings.Contains(output, "fix/PROJ-2-thing → main") {
		t.Errorf("preview does not name the branch and its base:\n%s", output)
	}
}

func TestPRDryRunOnAPushedBranchOmitsThePush(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is already published, so the dry run would only open, not push.
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "dry run: would open") || strings.Contains(output, "would push") {
		t.Errorf("dry run should open without pushing an already-published branch:\n%s", output)
	}
}

func TestPRTakesItsTitleFromTheConfiguredSource(t *testing.T) {
	t.Parallel()

	// Arrange
	// With the issue as the title source, the previewed title is the issue rather
	// than the branch's oldest commit ("work").
	clitest.FakeGh(t, clitest.GhResponses{})
	baseURL, _ := reviewJira(t)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"pull_request":{"title_source":"issue"}}`)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Open PROJ-2: thing") {
		t.Errorf("preview title does not use the configured issue source:\n%s", output)
	}
}

func TestPRTitlesAForgeIssueNumberFromItsCommitWithoutAskingJira(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch names the forge's issue 42, not a Jira one: Jira would read 42
	// as the id of an unrelated issue and title the pull request after it.
	var reached atomic.Bool

	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("OPS-7", "Bug", "an unrelated issue"), &reached)
	repo := prRepo(t, "fix/42-typo")
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+server.URL+`","token":"t"},`+
		`"pull_request":{"title_source":"issue"}}`)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Open work\n") || reached.Load() {
		t.Errorf("a forge issue number was read on Jira (asked: %t):\n%s", reached.Load(), output)
	}
}

func TestPROpensThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is already published and has no pull request, so one is opened.
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Opened #7") {
		t.Errorf("output does not report the opened pull request:\n%s", output)
	}
}

func TestPRPrintsTheOpenedPullRequestOnStdout(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request's address is the artifact a script captures, so it is
	// the one line that must be on stdout alone.
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	const opened = "Opened #7 https://github.com/owner/repo/pull/7"
	if !strings.Contains(printed.Stdout, opened) || strings.Contains(printed.Stderr, opened) {
		t.Errorf("the opened pull request is not on stdout alone:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}
}

func TestPRPushesThenOpensAnUnpublishedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch has never been pushed, so the flow pushes it — to a local bare
	// remote here — and then opens the pull request.
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	// A repository template becomes the pull request body during compose.
	writeRepoFile(t, repo, ".github/pull_request_template.md", "## Checklist\n\n- [ ] Tests\n")
	bare := localPushRemote(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := clitest.Run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Opened #7") {
		t.Errorf("output does not report the opened pull request:\n%s", output)
	}

	if !strings.Contains(gitOutput(t, bare, "branch", "--list", "fix/PROJ-2-thing"), "fix/PROJ-2-thing") {
		t.Error("the branch was not pushed to the remote before opening")
	}
}

func TestPRDoesNotMoveAForgeIssueNumberOnJira(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch names the forge's issue 42, not a Jira one, so there is no Jira
	// issue to move to the review status: Jira would read 42 as the id of an
	// unrelated issue.
	clitest.FakeGh(t, clitest.GhResponses{})
	baseURL, writes := reviewJira(t)
	repo := githubRepo(t, "fix/42-typo")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t",`+
		`"review_status":"`+clitest.StatusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	if writes.Applied() != 0 || strings.Contains(printed.Stderr, "Moved 42") {
		t.Errorf("a forge issue number was moved on Jira (%d transitions):\n%s", writes.Applied(), printed.Stderr)
	}
}

func TestPRPointsToAnnounceOnlyWhenMessagingIsSetUp(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configuration string
		wantPointer   bool
	}{
		"messaging set up": {
			configuration: `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
				`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`,
			wantPointer: true,
		},
		"no messaging": {
			configuration: `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.FakeGh(t, clitest.GhResponses{})
			repo := githubRepo(t, "fix/PROJ-2-thing")
			pretendPushed(t, repo)
			clitest.WriteFile(t, repo, tt.configuration)

			// Act
			printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
			// Assert
			if err != nil {
				t.Fatalf("pr --yes: %v (%+v)", err, printed)
			}

			if got := strings.Contains(printed.Stderr, "workflow announce"); got != tt.wantPointer {
				t.Errorf("pr --yes with %s: stderr names workflow announce = %v, want %v\n%s",
					name, got, tt.wantPointer, printed.Stderr)
			}

			if strings.Contains(printed.Stdout, "workflow announce") {
				t.Errorf("pr --yes put the pointer on stdout, the artifact's stream:\n%s", printed.Stdout)
			}
		})
	}
}

// localPushRemote gives repo a push remote through remote.pushDefault that
// names the forge repository origin does, so the pull request's head is read as
// on the base, while its pushurl sends a real push in a black-box test to a
// bare repository on disk rather than to GitHub. It returns the bare
// repository's path.
func localPushRemote(t *testing.T, repo string) string {
	t.Helper()

	bare := t.TempDir()
	git(t, bare, "init", "--bare", "--quiet")
	git(t, repo, "remote", "add", "push-target", "https://github.com/owner/repo.git")
	git(t, repo, "config", "remote.push-target.pushurl", bare)
	git(t, repo, "config", "remote.pushDefault", "push-target")

	return bare
}
