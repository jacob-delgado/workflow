// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

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

	fakeGh(t, ghResponses{
		pulls: openPull("Add login"), statusSequence: []string{runningStatus(), runningStatus(), passingStatus()},
	})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Fatalf("announce --when-green --yes = %v, posted %d; want it announced once (%+v)", err, posts.Load(), printed)
	}

	if !strings.Contains(printed.stderr, "Waiting for CI on #7 to pass") {
		t.Errorf("stderr = %q, want it to say it waits for CI on #7", printed.stderr)
	}
}

func TestAnnounceWhenGreenWaitsThroughAFailedRun(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login"), statusSequence: []string{failingStatus(), passingStatus()}})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.stderr, "A check failed on #7 (ci)") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want it to wait on a fix, then announce",
			err, posts.Load(), printed.stderr)
	}
}

func TestAnnounceWhenGreenRefusesAMergedPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: mergedPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	_, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(err); got != 4 || posts.Load() != 0 {
		t.Errorf("announce --when-green on a merge exited %d (%v), posted %d; want 4 and nothing", got, err, posts.Load())
	}
}

func TestAnnounceWhenGreenDryRunWaitsForNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: failingStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--dry-run")

	// Assert
	want := "dry run: would announce to the channel its webhook is bound to once CI on #7 passes"
	if err != nil || !strings.Contains(printed.stderr, want) || posts.Load() != 0 {
		t.Errorf("announce --when-green --dry-run = %v, said %q; want %q and nothing posted", err, printed.stderr, want)
	}
}

func TestAnnounceWhenGreenAsksOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	var asked []string

	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: runningStatus()})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, new(atomic.Int32))))

	// Act
	_, err := runStreams(t, repo, answering("n", &asked), "announce", "--when-green")

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
	fakeGh(t, ghResponses{pulls: openPull("Add login"), statusSequence: []string{"{", passingStatus()}})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.stderr, "Could not read CI on #7") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want the failed read told, then the post",
			err, posts.Load(), printed.stderr)
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

	err := recordAnnounced(ctx, home, messaging.MomentReady)
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
	fakeGh(t, ghResponses{pulls: openPull("Add login"), statusGate: gate})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))
	home := t.TempDir()

	recorded := make(chan error, 1)

	go func() { recorded <- recordWhenWaiting(t.Context(), gate, home) }()

	// Act
	printed, err := runStreamsAt(t, place{dir: repo, home: home}, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	recordErr := <-recorded
	if recordErr != nil {
		t.Fatalf("recording the announcement meanwhile: %v", recordErr)
	}

	if err != nil || posts.Load() != 0 ||
		!strings.Contains(printed.stderr, "Already announced from another session; not posted again.") {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want nothing posted a second time",
			err, posts.Load(), printed.stderr)
	}
}

func TestAnnounceWhenGreenGivesUpOnAPullRequestThatIsNoLongerTheOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		then string
		want string
	}{
		"closed":       {then: closedPull("Add login"), want: "#7 is no longer this branch's"},
		"merged first": {then: mergedPull("Add login, merged"), want: "#7 merged before its CI passed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var posts atomic.Int32

			fakeGh(t, ghResponses{pullsSequence: []string{openPull("Add login"), tt.then}, status: runningStatus()})
			repo := githubRepo(t, "fix/PROJ-2-thing")
			writeFile(t, repo, waitingConfig(webhook(t, &posts)))

			// Act
			_, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

			// Assert
			if got := cli.ExitStatus(err); got != 4 || !strings.Contains(err.Error(), tt.want) || posts.Load() != 0 {
				t.Errorf("announce --when-green exited %d (%v), posted %d; want 4 saying %q, nothing posted",
					got, err, posts.Load(), tt.want)
			}
		})
	}
}

func TestAnnounceWhenGreenWaitsAgainWhenTheGateRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes on the read that lets the post go, a check has failed by the
	// gate's own read, and a later run passes.
	var posts atomic.Int32

	fakeGh(t, ghResponses{
		pulls: openPull("Add login"), statusSequence: []string{passingStatus(), failingStatus(), passingStatus()},
	})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want it to wait again, then announce once",
			err, posts.Load(), printed.stderr)
	}
}
