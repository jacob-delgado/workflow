// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAnnounceDryRunComposesTheReadyMoment(t *testing.T) {
	// Arrange
	// An open pull request with green CI is ready for review.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "opened a pull request") {
		t.Errorf("preview does not mark the ready-for-review moment:\n%s", output)
	}

	if !strings.Contains(output, "dry run: would post to") {
		t.Errorf("dry run does not say it would post:\n%s", output)
	}
}

func TestAnnounceDryRunComposesTheMergedMoment(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: mergedPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "merged") {
		t.Errorf("preview does not mark the merged moment:\n%s", output)
	}
}

func TestAnnounceDryRunComposesTheCIRedMoment(t *testing.T) {
	// Arrange
	// An open pull request whose CI has failed is announced as CI red.
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "CI is red") {
		t.Errorf("preview does not mark the CI-red moment:\n%s", output)
	}
}

func TestAnnounceDryRunNamesTheConfiguredChannel(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"https://hooks.slack.example/services/x","channel":"#dev"}}`)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "#dev") {
		t.Errorf("preview does not name the configured channel:\n%s", output)
	}
}

func TestAnnounceDryRunNamesTheService(t *testing.T) {
	// Arrange
	// With a Teams webhook, the preview and prompt name Teams, not Slack.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"kind":"teams","webhook_url":"https://outlook.office.example/webhook/x"}}`)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Teams") || strings.Contains(output, "Slack") {
		t.Errorf("preview should name Teams, not Slack:\n%s", output)
	}
}

func TestAnnounceRefusesABranchWithNoPullRequest(t *testing.T) {
	// Arrange
	// The forge is reachable and answers that the branch has no pull request.
	fakeGh(t, ghResponses{pulls: "[]"})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no pull request") {
		t.Errorf("announce = %v, want it to refuse a branch with no pull request", err)
	}
}

func TestAnnounceDryRunComposesForAKeylessBranch(t *testing.T) {
	// Arrange
	// The branch names no issue, so the announcement is composed without one.
	fakeGh(t, ghResponses{pulls: openPull("Cleanup")})
	repo := githubRepo(t, "chore/cleanup")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "opened a pull request") {
		t.Errorf("preview does not compose the announcement for a keyless branch:\n%s", output)
	}
}

func TestAnnounceDryRunWithATrackerConfigured(t *testing.T) {
	// Arrange
	// With Jira reachable the announcement reads the issue and links it.
	var reached atomic.Bool

	jira := jiraServer(t, http.StatusOK, issueFixture("PROJ-2", "Bug", "thing"), &reached)
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"jira":{"base_url":"`+jira.URL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !reached.Load() {
		t.Error("the announcement did not read the configured tracker")
	}

	if !strings.Contains(output, "opened a pull request") {
		t.Errorf("preview does not compose the announcement:\n%s", output)
	}
}

func TestAnnounceDryRunWithoutAKnownAuthor(t *testing.T) {
	// Arrange
	// The forge will not name the author, so the announcement reads without one.
	fakeGh(t, ghResponses{pulls: openPull("Add login"), userError: true})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "A pull request is ready for review") {
		t.Errorf("preview should read without an author:\n%s", output)
	}
}

func TestAnnounceReportsAFailedPost(t *testing.T) {
	// Arrange
	// The pull request is found, but the Slack webhook cannot be reached.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "Slack") {
		t.Errorf("announce = %v, want the failed post reported", err)
	}
}

func TestAnnounceOutsideARepositoryReportsSo(t *testing.T) {
	// Arrange
	// Slack is configured, but there is no repository to read a branch from.
	dir := t.TempDir()
	writeFile(t, dir, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := run(t, dir, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "branch") {
		t.Errorf("announce = %v, want it to report the unreadable branch", err)
	}
}

func TestAnnounceReportsWhenThePullRequestCannotBeRead(t *testing.T) {
	// Arrange
	// Slack is configured, so the command gets as far as looking for the pull
	// request; with no forge there is none it can read.
	repo := prRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"messaging": {"webhook_url": "https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Errorf("announce = %v, want it to report the unreadable pull request", err)
	}
}
