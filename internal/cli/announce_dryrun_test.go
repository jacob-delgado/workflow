// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAnnounceYesDryRunPreviewsAMomentItWouldLeaveAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t),
		"announce", "--yes", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "opened a pull request") {
		t.Errorf("stdout does not preview the announcement:\n%s", printed.stdout)
	}

	if !strings.Contains(printed.stderr, alreadyAnnounced) ||
		!strings.Contains(printed.stderr, "dry run: would not announce it again") ||
		strings.Contains(printed.stderr, "run without --yes") {
		t.Errorf("stderr does not say that --yes would leave the announced moment as it is:\n%s", printed.stderr)
	}
}

func TestAnnounceDryRunComposesTheReadyMoment(t *testing.T) {
	t.Parallel()

	// Arrange
	// An open pull request with green CI is ready for review.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	// The preview is the artifact; what the dry run would do is said about it.
	if !strings.Contains(printed.stdout, "opened a pull request") {
		t.Errorf("stdout does not preview the ready-for-review moment:\n%s", printed.stdout)
	}

	if !strings.Contains(printed.stderr, "dry run: would announce to the channel its webhook is bound to") ||
		strings.Contains(printed.stdout, "dry run:") {
		t.Errorf("the dry-run line is not on stderr alone:\nstdout:\n%s\nstderr:\n%s", printed.stdout, printed.stderr)
	}
}

func TestAnnounceDryRunComposesTheMergedMoment(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"kind":"slack","client_id":"`+slackClientID+`","channel":"#dev"}}`)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "to #dev\n") || !strings.Contains(output, "dry run: would announce to #dev") {
		t.Errorf("preview does not name the configured channel:\n%s", output)
	}
}

func TestAnnounceDryRunNamesNoChannelWhereNoneIsSet(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"kind":"slack","client_id":"`+slackClientID+`"}}`)

	// Act
	output, err := run(t, repo, "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "to (no channel set)") || strings.Contains(output, "the configured Slack channel") {
		t.Errorf("preview claims a channel when none is set:\n%s", output)
	}
}

func TestAnnounceDryRunSaysAWebhookKeepsItsOwnChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	// A webhook posts where it is bound, whatever channel the file names.
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

	if !strings.Contains(output, "to the channel its webhook is bound to") || strings.Contains(output, "to #dev") {
		t.Errorf("preview names a channel the webhook does not post to:\n%s", output)
	}
}

func TestAnnounceDryRunComposesForAKeylessBranch(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
