// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// answerPatience is how long a test waits for a wait to read CI: a failsafe,
// never the pace of the test.
const answerPatience = 10 * time.Second

// errNeverWaited is a wait that never read CI.
var errNeverWaited = errors.New("the wait never read CI")

// waitingConfig is announcingConfig asking about CI every ten milliseconds,
// so a wait for it passes quickly.
func waitingConfig(webhookURL string) string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
		`"messaging":{"webhook_url":"` + webhookURL + `"},"timing":{"ci_interval":"10ms"}}`
}

func TestAnnounceWhenGreenAnnouncesOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls:          clitest.OpenPull("Add login"),
		StatusSequence: []string{clitest.RunningStatus(), clitest.RunningStatus(), clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Fatalf("announce --when-green --yes = %v, posted %d; want it announced once (%+v)", err, posts.Load(), printed)
	}

	if !strings.Contains(printed.Stderr, "Waiting for CI on #7 to pass") {
		t.Errorf("stderr = %q, want it to say it waits for CI on #7", printed.Stderr)
	}
}

func TestAnnounceWhenGreenWaitsThroughAFailedRun(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{clitest.FailingStatus(), clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stderr, "A check failed on #7 (ci)") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want it to wait on a fix, then announce",
			err, posts.Load(), printed.Stderr)
	}
}

func TestAnnounceWhenGreenRefusesAMergedPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.MergedPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(err); got != 4 || posts.Load() != 0 {
		t.Errorf("announce --when-green on a merge exited %d (%v), posted %d; want 4 and nothing", got, err, posts.Load())
	}
}

func TestAnnounceWhenGreenDryRunWaitsForNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.FailingStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--dry-run")

	// Assert
	want := "dry run: would announce to the channel its webhook is bound to once CI on #7 passes"
	if err != nil || !strings.Contains(printed.Stderr, want) || posts.Load() != 0 {
		t.Errorf("announce --when-green --dry-run = %v, said %q; want %q and nothing posted", err, printed.Stderr, want)
	}
}

func TestAnnounceWhenGreenAsksOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.RunningStatus()})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, new(atomic.Int32))))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "announce", "--when-green")

	// Assert
	if err != nil || len(asked) != 1 || !strings.HasPrefix(asked[0], "Announce to Slack once CI on #7 passes?") {
		t.Errorf("announce --when-green = %v, asked %q; want it to ask to announce once CI passes", err, asked)
	}
}

func TestAnnounceWhenGreenWaitsPastAReadThatFails(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	// An unreadable answer stands for a read of CI that failed.
	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{"{", clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stderr, "Could not read CI on #7") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want the failed read told, then the post",
			err, posts.Load(), printed.Stderr)
	}
}

// recordWhenWaiting records pull request 7 announced as ready in the store
// under home once a wait has read its CI, as another session would announce
// it meanwhile, then lets the CI pass.
func recordWhenWaiting(ctx context.Context, gate, home string) error {
	deadline := time.Now().Add(answerPatience)

	for {
		_, err := os.Stat(filepath.Join(gate, "read"))
		if err == nil {
			break
		}

		if time.Now().After(deadline) {
			return errNeverWaited
		}

		time.Sleep(time.Millisecond)
	}

	err := clitest.RecordAnnounced(ctx, home, messaging.MomentReady)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(gate, "release"), nil, 0o600)
	if err != nil {
		return fmt.Errorf("letting CI pass: %w", err)
	}

	return nil
}

func TestAnnounceWhenGreenPostsNothingAnnouncedWhileItWaited(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	gate := t.TempDir()
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), StatusGate: gate})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))
	home := t.TempDir()

	recorded := make(chan error, 1)

	go func() { recorded <- recordWhenWaiting(t.Context(), gate, home) }()

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes",
	)

	// Assert
	recordErr := <-recorded
	if recordErr != nil {
		t.Fatalf("recording the announcement meanwhile: %v", recordErr)
	}

	if err != nil || posts.Load() != 0 ||
		!strings.Contains(printed.Stderr, "Already announced from another session; not posted again.") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want nothing posted a second time",
			err, posts.Load(), printed.Stderr)
	}
}

func TestAnnounceWhenGreenGivesUpOnAPullRequestThatIsNoLongerTheOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		then string
		want string
	}{
		"closed":       {then: clitest.ClosedPull("Add login"), want: "#7 is no longer this branch's"},
		"merged first": {then: clitest.MergedPull("Add login, merged"), want: "#7 merged before its CI passed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var posts atomic.Int32

			clitest.FakeGh(t, clitest.GhResponses{
				PullsSequence: []string{clitest.OpenPull("Add login"), tt.then}, Status: clitest.RunningStatus(),
			})
			repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
			clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

			// Assert
			if got := cli.ExitStatus(err); got != 4 || !strings.Contains(err.Error(), tt.want) || posts.Load() != 0 {
				t.Errorf("announce --when-green exited %d (%v), posted %d; want 4 saying %q, nothing posted",
					got, err, posts.Load(), tt.want)
			}
		})
	}
}

func TestAnnounceWhenGreenSaysWhatItWaitsOnOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two reads find CI running alike before it passes; only a change is said.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls:          clitest.OpenPull("Add login"),
		StatusSequence: []string{clitest.RunningStatus(), clitest.RunningStatus(), clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("announce --when-green --yes = %v (%+v)", err, printed)
	}

	if said := strings.Count(printed.Stderr, "Waiting for CI on #7 to pass"); said != 1 {
		t.Errorf("stderr said it waits %d times, want once:\n%s", said, printed.Stderr)
	}
}

func TestAnnounceWhenGreenWaitsAgainWhenTheGateCannotReadCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes on the read that lets the post go, the gate's own read cannot
	// be understood, and a later run passes.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls:          clitest.OpenPull("Add login"),
		StatusSequence: []string{clitest.PassingStatus(), "{", clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want it to wait again, then announce once",
			err, posts.Load(), printed.Stderr)
	}
}

func TestAnnounceWhenGreenWaitsAgainWhenTheGateRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes on the read that lets the post go, a check has failed by the
	// gate's own read, and a later run passes.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls:          clitest.OpenPull("Add login"),
		StatusSequence: []string{clitest.PassingStatus(), clitest.FailingStatus(), clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want it to wait again, then announce once",
			err, posts.Load(), printed.Stderr)
	}
}
