// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// alreadyAnnounced is what announce says of a pull request an earlier session
// announced at the moment it is at.
const alreadyAnnounced = "#7 was already announced at this moment in an earlier session"

func TestAnnounceAsksByTheServiceName(t *testing.T) {
	t.Parallel()

	// Arrange
	// With a Teams webhook, the question names Teams, not Slack.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
		`"messaging":{"kind":"teams","webhook_url":"https://outlook.office.example/webhook/x"}}`)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "announce")
	// Assert
	if err != nil {
		t.Fatalf("announce: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.HasPrefix(asked[0], "Announce to Teams?") ||
		strings.Contains(printed.Stdout+printed.Stderr, "Slack") {
		t.Errorf("announce asked %q and said %+v; want Teams named, not Slack", asked, printed)
	}
}

func TestAnnounceAsksToAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "announce")
	// Assert
	if err != nil {
		t.Fatalf("announce: %v (%+v)", err, printed)
	}

	// The command is announce, and it says so: its question and its decline use
	// the verb the interface and the web use, not "post".
	if len(asked) != 1 || !strings.HasPrefix(asked[0], "Announce to Slack?") ||
		!strings.Contains(printed.Stderr, "Not announced.") {
		t.Errorf("announce asked %q and said %q; want it to ask to announce, and say it did not", asked, printed.Stderr)
	}
}

func TestAnnounceSaysWhenAlreadyPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stderr, alreadyAnnounced) || strings.Contains(printed.Stdout, alreadyAnnounced) {
		t.Errorf("announce does not say, on stderr, that it already announced this moment:"+
			"\nstdout:\n%s\nstderr:\n%s", printed.Stdout, printed.Stderr)
	}
}

func TestAnnounceYesSkipsWhatWasAlreadyAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post would fail — the webhook is on a reserved domain — so succeeding is
	// what proves nothing was posted.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--yes",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --yes = %v, want the repeat skipped and nothing posted (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stderr, alreadyAnnounced) ||
		!strings.Contains(printed.Stderr, "Not announced again; run without --yes to be asked.") ||
		printed.Stdout != "" {
		t.Errorf("announce --yes does not skip the repeat, saying so on stderr alone:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}
}

func TestAnnounceAsksBeforeAnnouncingAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	var asked []string

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.Answering("n", &asked), "announce",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce: %v (%+v)", err, printed)
	}

	if len(asked) != 1 || !strings.HasPrefix(asked[0], "Announce to Slack again?") {
		t.Errorf("announce asked %q, want it to ask whether to announce again", asked)
	}
}

func TestAnnounceAgainWithNoTerminalDoesNotPointAtYes(t *testing.T) {
	t.Parallel()

	// Arrange
	// --yes leaves a moment already announced as it is, so the refusal must not
	// send a script there: it names the one way to announce again.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)
	noTerminal := cmdkit.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, noTerminal, "announce")

	// Assert
	wantExit(t, err, 2)

	if err == nil || strings.Contains(err.Error(), "--yes") || !strings.Contains(err.Error(), "at a terminal") {
		t.Errorf("announce = %v, want it to say to run it at a terminal, not to pass --yes", err)
	}
}

func TestAnnounceWithNoTerminalPointsAtYes(t *testing.T) {
	t.Parallel()

	// Arrange
	// Nothing was announced before, so --yes is the way on, not a terminal.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	noTerminal := cmdkit.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := clitest.RunGuided(t, repo, noTerminal, "announce")

	// Assert
	wantExit(t, err, 2)

	if err == nil || !strings.Contains(err.Error(), "--yes") || strings.Contains(err.Error(), "again") {
		t.Errorf("announce = %v, want it to say to pass --yes", err)
	}
}

func TestAnnounceOffersAMomentNotYetAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	// The earlier session announced the pull request's merge; it is open now,
	// which is another moment.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)
	home := clitest.AnnouncedEarlier(t, messaging.MomentMerged)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--dry-run",
	)
	// Assert
	if err != nil {
		t.Fatalf("announce --dry-run: %v (%+v)", err, printed)
	}

	if strings.Contains(printed.Stderr, "already announced") || !strings.Contains(printed.Stderr, "dry run:") {
		t.Errorf("announce treated another moment as already announced:\n%s", printed.Stderr)
	}
}

func TestAnnounceRefusesABranchWithNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge is reachable and answers that the branch has no pull request.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "[]"})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no pull request") {
		t.Errorf("announce = %v, want it to refuse a branch with no pull request", err)
	}

	wantExit(t, err, 4)
}

func TestAnnounceRefusesABranchWithNoPullRequestInItsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refusal is the command line's own sentence, not the shared layer's.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "[]"})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "there is no pull request on this branch to announce") {
		t.Errorf("announce = %v, want the command line's refusal of a branch with no pull request", err)
	}
}

func TestNoPullRequestPointsAtPR(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "[]"})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.HasSuffix(err.Error(), "; open one with `workflow pr`") {
		t.Errorf("announce = %v, want the refusal to name the command that opens a pull request", err)
	}
}

func TestMessagingNotConfiguredNamesTheKeysAndDoctor(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.Run(t, t.TempDir(), "announce")

	// Assert
	for _, next := range []string{"messaging.kind", "messaging.webhook_url", "workflow slack login", "workflow doctor"} {
		if err == nil || !strings.Contains(err.Error(), next) {
			t.Errorf("announce = %v, want the refusal to name %s", err, next)
		}
	}
}

func TestAnnounceReportsAFailedPost(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request is found, but the Slack webhook cannot be reached.
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "announcing to Slack") {
		t.Errorf("announce = %v, want the failed announcement reported", err)
	}

	wantExit(t, err, 5)
}

func TestAnnounceOutsideARepositoryReportsSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// Slack is configured, but there is no repository to read a branch from.
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := clitest.Run(t, dir, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "branch") {
		t.Errorf("announce = %v, want it to report the unreadable branch", err)
	}
}

func TestAnnounceReportsWhenThePullRequestCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// Slack is configured, so the command gets as far as looking for the pull
	// request; with no forge there is none it can read.
	repo := clitest.PRRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, `{"messaging": {"webhook_url": "https://hooks.slack.example/services/x"}}`)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Errorf("announce = %v, want it to report the unreadable pull request", err)
	}
}
