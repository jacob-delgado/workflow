// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
)

// announcedEarlier is a home whose store remembers the fake forge's pull request
// 7 announced at moment, as an earlier session — of the interface or of
// announce — leaves it: keyed by the repository's host and path.
func announcedEarlier(t *testing.T, moment messaging.Moment) string {
	t.Helper()

	home := t.TempDir()
	env := isolatedEnvironment(home)

	dir, err := store.Dir(runtime.GOOS, home, func(name string) (string, bool) {
		value, ok := env[name]

		return value, ok
	})
	if err != nil {
		t.Fatalf("finding the store under %s: %v", home, err)
	}

	err = store.New(dir, false).RecordAnnounce(t.Context(), "github.com/owner/repo",
		store.Announce{Pull: 7, Moment: int(moment)}, time.Now())
	if err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	return home
}

// alreadyAnnounced is what announce says of a pull request an earlier session
// announced at the moment it is at.
const alreadyAnnounced = "#7 was already announced at this moment in an earlier session"

func TestAnnounceDryRunComposesTheReadyMoment(t *testing.T) {
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

	if !strings.Contains(printed.stderr, "dry run: would post to") || strings.Contains(printed.stdout, "dry run:") {
		t.Errorf("the dry-run line is not on stderr alone:\nstdout:\n%s\nstderr:\n%s", printed.stdout, printed.stderr)
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

func TestAnnounceSaysWhenAlreadyPosted(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stderr, alreadyAnnounced) || strings.Contains(printed.stdout, alreadyAnnounced) {
		t.Errorf("announce does not say, on stderr, that it already announced this moment:"+
			"\nstdout:\n%s\nstderr:\n%s", printed.stdout, printed.stderr)
	}
}

func TestAnnounceYesSkipsWhatWasAlreadyAnnounced(t *testing.T) {
	// Arrange
	// A post would fail — the webhook is on a reserved domain — so succeeding is
	// what proves nothing was posted.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes = %v, want the repeat skipped and nothing posted (%+v)", err, printed)
	}

	if !strings.Contains(printed.stderr, alreadyAnnounced) || !strings.Contains(printed.stderr, "without --yes") ||
		printed.stdout != "" {
		t.Errorf("announce --yes does not skip the repeat, saying so on stderr alone:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestAnnounceAsksBeforeAnnouncingAgain(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)

	var asked []string

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, answering("n", &asked), "announce")
	// Assert
	if err != nil {
		t.Fatalf("announce: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.Contains(asked[0], " again?") {
		t.Errorf("announce asked %q, want it to ask whether to announce again", asked)
	}
}

func TestAnnounceAgainWithNoTerminalDoesNotPointAtYes(t *testing.T) {
	// Arrange
	// --yes leaves a moment already announced as it is, so the refusal must not
	// send a script there: it names the one way to announce again.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentReady)
	noTerminal := cli.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := runStreamsAt(t, place{dir: repo, home: home}, noTerminal, "announce")

	// Assert
	wantExit(t, err, 2)

	if err == nil || strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "at a terminal") {
		t.Errorf("announce = %v, want it to say to run it at a terminal, not to pass --yes", err)
	}
}

func TestAnnounceOffersAMomentNotYetAnnounced(t *testing.T) {
	// Arrange
	// The earlier session announced the pull request's merge; it is open now,
	// which is another moment.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)
	home := announcedEarlier(t, messaging.MomentMerged)

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.stderr, "already announced") || !strings.Contains(printed.stderr, "dry run:") {
		t.Errorf("announce treated another moment as already announced:\n%s", printed.stderr)
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

	wantExit(t, err, 4)
}

func TestAnnounceRefusesABranchWithNoPullRequestInItsOwnWords(t *testing.T) {
	// Arrange
	// The refusal is the command line's own sentence, not the shared layer's.
	fakeGh(t, ghResponses{pulls: "[]"})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "there is no pull request on this branch to announce") {
		t.Errorf("announce = %v, want the command line's refusal of a branch with no pull request", err)
	}
}

func TestNoPullRequestPointsAtPR(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: "[]"})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, forgeCLIConfig)

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "workflow pr") {
		t.Errorf("announce = %v, want the refusal to name the command that opens a pull request", err)
	}
}

func TestMessagingNotConfiguredNamesTheKeysAndDoctor(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "announce")

	// Assert
	for _, next := range []string{"messaging.kind", "messaging.webhook_url", "messaging.token", "workflow doctor"} {
		if err == nil || !strings.Contains(err.Error(), next) {
			t.Errorf("announce = %v, want the refusal to name %s", err, next)
		}
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

	wantExit(t, err, 5)
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
