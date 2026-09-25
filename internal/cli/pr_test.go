// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// commit makes an empty commit in dir with a fixed identity.
func commit(t *testing.T, dir, message string) {
	t.Helper()

	git(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", message)
}

// prRepo is a repository on branch, one commit ahead of main — a branch a pull
// request can be opened for.
func prRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", branch)
	commit(t, repo, "work")

	return repo
}

func TestPRQuestionNamesThePush(t *testing.T) {
	cases := map[string]struct {
		published bool
		want      string
	}{
		"a branch not yet pushed": {want: "Push fix/PROJ-2-thing and open the pull request?"},
		"a branch already pushed": {published: true, want: "Open the pull request?"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			fakeGh(t, ghResponses{})
			repo := githubRepo(t, "fix/PROJ-2-thing")

			if tt.published {
				pretendPushed(t, repo)
			}

			writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

			var asked []string

			// Act
			_, err := runStreams(t, repo, answering("n", &asked), "pr")

			// Assert
			if err != nil || len(asked) != 1 || !strings.HasPrefix(asked[0], tt.want) {
				t.Errorf("pr = %v, asked %q; want the one question %q", err, asked, tt.want)
			}
		})
	}
}

func TestPROpensAMergeRequestInGitLabsWords(t *testing.T) {
	// Arrange
	fakeGlab(t)
	repo := prRepo(t, "fix/PROJ-2-thing")
	git(t, repo, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"gitlab","host":"gitlab.com"}}`)

	var asked []string

	// Act
	printed, err := runStreams(t, repo, answering("y", &asked), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.HasPrefix(asked[0], "Open the merge request?") {
		t.Errorf("pr asked %q, want GitLab's noun in its question", asked)
	}

	if !strings.Contains(printed.stdout, "Opened !7 "+gitlabMergeRequest) {
		t.Errorf("pr printed %q, want the merge request marked with GitLab's sigil", printed.stdout)
	}
}

func TestPRDryRunPreviewsWithoutOpening(t *testing.T) {
	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "fix/PROJ-2-thing → main") {
		t.Errorf("preview does not name the branch and its base:\n%s", output)
	}

	if !strings.Contains(output, "dry run: would push fix/PROJ-2-thing and open") {
		t.Errorf("dry run does not describe the push and open it would do:\n%s", output)
	}
}

func TestPRComposesForABranchWithoutAnIssueKey(t *testing.T) {
	// Arrange
	// The branch names no issue, so the pull request is composed without one.
	repo := prRepo(t, "chore/cleanup")

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "chore/cleanup → main") {
		t.Errorf("preview does not name the keyless branch:\n%s", output)
	}
}

func TestPRDryRunPreviewsWithATrackerConfigured(t *testing.T) {
	// Arrange
	// With Jira reachable the compose reads the issue to link it; the preview is
	// the same, and nothing is opened.
	var reached atomic.Bool

	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-2", "Bug", "thing"), &reached)
	repo := prRepo(t, "fix/PROJ-2-thing")
	writeConfigFor(t, repo, server.URL)

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
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
	// Arrange
	// The branch is already published, so the dry run would only open, not push.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "dry run: would open") || strings.Contains(output, "would push") {
		t.Errorf("dry run should open without pushing an already-published branch:\n%s", output)
	}
}

func TestPRTakesItsTitleFromTheConfiguredSource(t *testing.T) {
	// Arrange
	// With the issue as the title source, the previewed title is the issue rather
	// than the branch's oldest commit ("work").
	fakeGh(t, ghResponses{})
	baseURL, _ := reviewJira(t)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"pull_request":{"title_source":"issue"}}`)

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Open PROJ-2: thing") {
		t.Errorf("preview title does not use the configured issue source:\n%s", output)
	}
}

func TestPROpensThePullRequest(t *testing.T) {
	// Arrange
	// The branch is already published and has no pull request, so one is opened.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Opened #7") {
		t.Errorf("output does not report the opened pull request:\n%s", output)
	}
}

func TestPRPrintsTheOpenedPullRequestOnStdout(t *testing.T) {
	// Arrange
	// The pull request's address is the artifact a script captures, so it is
	// the one line that must be on stdout alone.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	const opened = "Opened #7 https://github.com/owner/repo/pull/7"
	if !strings.Contains(printed.stdout, opened) || strings.Contains(printed.stderr, opened) {
		t.Errorf("the opened pull request is not on stdout alone:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestPRPushesThenOpensAnUnpublishedBranch(t *testing.T) {
	// Arrange
	// The branch has never been pushed, so the flow pushes it — to a local bare
	// remote here — and then opens the pull request.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	// A repository template becomes the pull request body during compose.
	writeRepoFile(t, repo, ".github/pull_request_template.md", "## Checklist\n\n- [ ] Tests\n")
	bare := localPushRemote(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := run(t, repo, "pr", "--yes")
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

func TestPRReportsAFailedOpen(t *testing.T) {
	// Arrange
	// The branch is published, but the forge rejects the open.
	fakeGh(t, ghResponses{createError: true})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Errorf("pr = %v, want the failed open reported", err)
	}
}

func TestPRReportsAFailedPush(t *testing.T) {
	// Arrange
	// The push remote does not exist, so the branch cannot be published and no
	// pull request is opened.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	git(t, repo, "remote", "add", "push-target", filepath.Join(t.TempDir(), "nonexistent.git"))
	git(t, repo, "config", "remote.pushDefault", "push-target")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pushed") {
		t.Errorf("pr = %v, want a failed push reported", err)
	}

	wantExit(t, err, 1)
}

func TestPRRefusesASecondPullRequest(t *testing.T) {
	// Arrange
	// The branch already has an open pull request, which the forge reports.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("pr = %v, want it to refuse a second pull request", err)
	}

	wantExit(t, err, 4)
}

func TestPRRefusesASecondPullRequestInItsOwnWords(t *testing.T) {
	// Arrange
	// The refusal is the command line's own sentence, not the shared layer's.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "an open pull request already exists for this branch") {
		t.Errorf("pr = %v, want the command line's refusal of a second pull request", err)
	}
}

func TestPullAlreadyOpenCarriesItsURL(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "https://github.com/owner/repo/pull/7") {
		t.Errorf("pr = %v, want the refusal to carry the open pull request's address", err)
	}
}

func TestPRRefusesABranchWithNoCommits(t *testing.T) {
	// Arrange
	// A branch level with main has nothing to propose.
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "commits") {
		t.Errorf("pr = %v, want a refusal that there are no commits to open", err)
	}

	wantExit(t, err, 4)
}

func TestPRDoesNotMoveAForgeIssueNumberOnJira(t *testing.T) {
	// Arrange
	// The branch names the forge's issue 42, not a Jira one, so there is no Jira
	// issue to move to the review status: Jira would read 42 as the id of an
	// unrelated issue.
	fakeGh(t, ghResponses{})
	baseURL, writes := reviewJira(t)
	repo := githubRepo(t, "fix/42-typo")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t","review_status":"`+statusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%+v)", err, printed)
	}

	if writes.applied() != 0 || strings.Contains(printed.stderr, "Moved 42") {
		t.Errorf("a forge issue number was moved on Jira (%d transitions):\n%s", writes.applied(), printed.stderr)
	}
}
