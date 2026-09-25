// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// standupSearch is the Jira search body for one issue you touched in the window.
func standupSearch(key, summary, status string) string {
	return `{"total":1,"issues":[{"key":"` + key + `","fields":{"summary":"` + summary +
		`","issuetype":{"name":"Bug"},"status":{"name":"` + status +
		`","statusCategory":{"key":"indeterminate"}}}}]}`
}

// repoWithCommit is a repository with one in-window commit, so the draft has
// something to gather.
func repoWithCommit(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "work")

	return repo
}

// staleCommit makes a commit dated well before any standup window, so it is
// gathered by nothing and the sections read as empty.
func staleCommit(t *testing.T, dir, message string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "-C", dir,
		"-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", message)

	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00", "GIT_COMMITTER_DATE=2020-01-01T00:00:00")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v (%s)", err, out)
	}
}

func TestStandupDraftsFromTheRepository(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "config", "user.email", "me@example.com")
	git(t, dir, "config", "user.name", "Me")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "Add the widget")

	// Act
	out, err := run(t, dir, "standup", "--no-edit")
	// Assert
	if err != nil {
		t.Fatalf("standup in a repository failed: %v\n%s", err, out)
	}

	if !strings.Contains(out, "# Standup") || !strings.Contains(out, "Add the widget") {
		t.Errorf("standup did not draft the commit:\n%s", out)
	}
}

func TestStandupOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "standup", "--no-edit")

	// Assert
	if err == nil {
		t.Error("standup outside a repository returned no error")
	}
}

func TestStandupDraftsCommitsIssuesAndPulls(t *testing.T) {
	// Arrange
	// A day with a commit, an issue you touched, and an open pull request.
	server := jiraServer(t, http.StatusOK, standupSearch("PROJ-7", "Fix the login", "In Progress"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-7-login")
	commit(t, repo, "Fix the login")
	writeFile(t, repo, `{"jira":{"base_url":"`+server.URL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	out, err := run(t, repo, "standup", "--no-edit")
	if err != nil {
		t.Fatalf("standup: %v (%s)", err, out)
	}

	// Assert
	for _, want := range []string{"# Standup", "Fix the login", "PROJ-7", "In Progress", "#7", "Add login"} {
		if !strings.Contains(out, want) {
			t.Errorf("standup draft missing %q:\n%s", want, out)
		}
	}
}

func TestStandupOmitsAMergedPullRequest(t *testing.T) {
	// Arrange
	// The branch's pull request has merged, so it is no longer open work to list.
	fakeGh(t, ghResponses{pulls: mergedPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-7-login")
	commit(t, repo, "Fix the login")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	out, err := run(t, repo, "standup", "--no-edit")
	if err != nil {
		t.Fatalf("standup: %v (%s)", err, out)
	}

	// Assert
	if strings.Contains(out, "#7") || !strings.Contains(out, "## Pull requests\n- none") {
		t.Errorf("standup listed a merged pull request as open:\n%s", out)
	}
}

func TestStandupWithNoWorkSaysEachSectionIsEmpty(t *testing.T) {
	// Arrange
	// A repository whose only commit falls outside the window, no forge and no
	// tracker: every section is empty.
	repo := t.TempDir()
	gitInit(t, repo)
	staleCommit(t, repo, "long ago")

	// Act
	out, err := run(t, repo, "standup", "--no-edit")
	if err != nil {
		t.Fatalf("standup: %v (%s)", err, out)
	}

	// Assert
	if got := strings.Count(out, "- none"); got != 3 {
		t.Errorf("empty standup listed %d empty sections, want 3:\n%s", got, out)
	}
}

func TestStandupOpensTheDraftInTheEditor(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, standupSearch("PROJ-7", "Fix the login", "In Progress"), new(atomic.Bool))
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"jira":{"base_url":"`+server.URL+`","token":"t"}}`)

	var seen, help string

	prompt := cli.Prompt{
		Compose: func(draft, below string) (string, error) {
			seen, help = draft, below

			return "# Standup\n\nwhat I actually did", nil
		},
	}

	// Act
	out, err := runGuided(t, repo, prompt, "standup")
	if err != nil {
		t.Fatalf("standup: %v (%s)", err, out)
	}

	// Assert
	if !strings.Contains(seen, "PROJ-7") || !strings.Contains(out, "what I actually did") {
		t.Errorf("the editor was not handed the draft, or its edit was dropped:\nseen=%q\nout=%s", seen, out)
	}

	if !strings.Contains(help, "Edit your standup above this line") {
		t.Errorf("the editor was handed help %q, want the standup's own", help)
	}
}

func TestStandupWithNothingLeftSaysSo(t *testing.T) {
	// Arrange
	// The editor empties the draft, so there is nothing left to post.
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	prompt := cli.Prompt{Compose: func(string, string) (string, error) { return "   \n\t", nil }}

	// Act
	printed, err := runStreams(t, repo, prompt, "standup")
	if err != nil {
		t.Fatalf("standup: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.stderr, "Nothing to share.") || printed.stdout != "" {
		t.Errorf("an emptied draft was not reported on stderr alone:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestStandupIsNotPostedWhenDeclined(t *testing.T) {
	// Arrange
	// Slack is configured, but the confirmation is declined, so nothing is posted.
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	printed, err := runStreams(t, repo, scripted([]string{"n"}, nil), "standup", "--no-edit")
	if err != nil {
		t.Fatalf("standup: %v (%+v)", err, printed)
	}

	// Assert
	// The draft is the artifact; the decline is said about it.
	if !strings.Contains(printed.stdout, "# Standup") || !strings.Contains(printed.stderr, "Not posted.") ||
		strings.Contains(printed.stdout, "Not posted.") {
		t.Errorf("a declined standup put its draft or its notice on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestStandupReportsAFailedPost(t *testing.T) {
	// Arrange
	// A Teams service is configured and the post is confirmed, but the webhook is
	// unreachable, so the post fails — and the error names the service in use, not
	// a hardcoded "Slack".
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"messaging":{"kind":"teams","webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := runGuided(t, repo, scripted([]string{"y"}, nil), "standup", "--no-edit")

	// Assert
	// The failure names the service in use — "posting to Teams" — rather than a
	// hardcoded "Slack" (the underlying transport error still mentions the Slack
	// API, which is a separate, deeper surface).
	if err == nil || !strings.Contains(err.Error(), "posting to Teams") {
		t.Errorf("standup returned %v, want a Teams post failure", err)
	}
}

func TestStandupDryRunPostsNothing(t *testing.T) {
	// Arrange
	// The webhook cannot be reached, so a post that was attempted would fail.
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "standup", "--no-edit", "--dry-run", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("standup --dry-run --yes = %v, want nothing posted and success (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "# Standup") ||
		!strings.Contains(printed.stderr, "dry run: would post to Slack") {
		t.Errorf("a dry run did not preview the draft and say what it would post:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestStandupYesPostsWithoutAsking(t *testing.T) {
	// Arrange
	// Nothing answers the prompt, and the webhook cannot be reached: a post that
	// went ahead without asking fails on the network, not on the prompt.
	repo := repoWithCommit(t)
	writeFile(t, repo, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := run(t, repo, "standup", "--no-edit", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "posting to Slack") {
		t.Errorf("standup --yes = %v, want the post attempted without a confirmation", err)
	}
}

func TestStandupRejectsNegativeDays(t *testing.T) {
	cases := map[string]string{
		"negative": "-1",
		"zero":     "0",
	}

	for name, days := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			repo := repoWithCommit(t)

			// Act
			_, err := run(t, repo, "standup", "--no-edit", "--days", days)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "--days") {
				t.Errorf("standup --days %s = %v, want the value refused by name", days, err)
			}

			wantExit(t, err, 2)
		})
	}
}
