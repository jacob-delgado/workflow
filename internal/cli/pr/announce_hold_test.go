// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// armedBranch is the branch the tests below arm an announcement on.
const armedBranch = "fix/PROJ-2-thing"

// switchWhenWaiting checks out main in repo once a wait has read CI on the
// pull request gate answers for, as a person switching away meanwhile would,
// then lets the CI pass.
func switchWhenWaiting(gate, repo string) error {
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

	err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o600)
	if err != nil {
		return fmt.Errorf("switching to main: %w", err)
	}

	err = os.WriteFile(filepath.Join(gate, "release"), nil, 0o600)
	if err != nil {
		return fmt.Errorf("letting CI pass: %w", err)
	}

	return nil
}

func TestAnnounceWhenGreenWaitsOnTheBranchItWasArmedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	// C37: the checkout moves to main, which has no pull request, while the
	// wait reads CI on #7; the wait still reads the branch it was armed on.
	var posts atomic.Int32

	gate := t.TempDir()
	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), PullsBranch: armedBranch, StatusGate: gate,
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	switched := make(chan error, 1)

	go func() { switched <- switchWhenWaiting(gate, repo) }()

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	switchErr := <-switched
	if switchErr != nil {
		t.Fatalf("switching the checkout meanwhile: %v", switchErr)
	}

	if err != nil || posts.Load() != 1 {
		t.Errorf("announce --when-green = %v, posted %d, said %q; want #7 announced once, the switch not read",
			err, posts.Load(), printed.Stderr)
	}
}

func TestPRAnnounceGivesUpOnAPullRequestClosedAfterTheGateRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// C39: CI passes, a check has failed by the gate's own read as it is
	// posted, and the pull request is then closed, which the forge's find
	// passes over: it is no longer the one to announce.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		PullsSequence:  []string{"[]", clitest.OpenPull("work"), clitest.ClosedPull("work")},
		StatusSequence: []string{clitest.PassingStatus(), clitest.FailingStatus()},
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if got := cli.ExitStatus(err); got != 4 || !strings.Contains(fmt.Sprint(err), "#7 is no longer this branch's") ||
		posts.Load() != 0 {
		t.Errorf("pr --announce exited %d (%v), posted %d, said %q; want 4, #7 no longer the branch's, nothing posted",
			got, err, posts.Load(), printed.Stderr)
	}
}

// signalsWatchedOutsideBubbles starts the runtime's watch over the process's
// signals from outside any synctest bubble, as a command run in one asks for
// its interrupt: a watch first started inside a bubble belongs to it, and
// the runtime refuses its use from any other.
func signalsWatchedOutsideBubbles() {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	signal.Stop(interrupts)
}

// clocked is how a run on a fake clock went: what it printed, its error, and
// how long it took by that clock.
type clocked struct {
	printed clitest.Streams
	err     error
	took    time.Duration
}

// clockedWorld is what a run on a fake clock meets: the forge's answers, the
// messaging service's address, and the home it runs in, a fresh one when
// empty.
type clockedWorld struct {
	responses clitest.GhResponses
	hook      string
	home      string
}

// onAFakeClock runs args on a fake clock, in a synctest bubble, in a pushed
// repository on armedBranch in world, which reads CI every minute. The
// messaging service is the test's, outside the bubble, whose clock only moves
// once everything in it waits on the clock.
func onAFakeClock(t *testing.T, world clockedWorld, args ...string) clocked {
	t.Helper()

	signalsWatchedOutsideBubbles()

	var run clocked

	synctest.Test(t, func(t *testing.T) {
		clitest.FakeGh(t, world.responses)
		repo := clitest.GithubRepo(t, armedBranch)
		clitest.PretendPushed(t, repo)
		clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+
			`"messaging":{"webhook_url":"`+world.hook+`"},"timing":{"ci_interval":"1m"}}`)

		home := cmp.Or(world.home, t.TempDir())
		started := time.Now()
		run.printed, run.err = clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.UnusedPrompt(t),
			args...)
		run.took = time.Since(started)
	})

	return run
}

func TestAnnounceWhenGreenGivesUpOnCIItCannotReadForTenMinutes(t *testing.T) {
	t.Parallel()

	// Arrange
	// C38: the forge asks every read of CI to wait, as a spent rate limit
	// does; on a fake clock, the wait gives up once ten minutes have passed,
	// and exits as a forge it could not reach.
	var posts atomic.Int32

	world := clockedWorld{
		responses: clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), StatusRateLimited: true},
		hook:      webhook(t, &posts),
	}

	// Act
	run := onAFakeClock(t, world, "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(run.err); got != 5 ||
		!strings.Contains(fmt.Sprint(run.err), "could not read CI on #7 for 10m") {
		t.Errorf("announce --when-green exited %d (%v); want 5, saying CI on #7 could not be read for 10m", got, run.err)
	}

	if posts.Load() != 0 || run.took < 10*time.Minute || run.took > 12*time.Minute {
		t.Errorf("announce --when-green posted %d after %v; want nothing posted, after ten minutes",
			posts.Load(), run.took)
	}
}

func TestAnnounceWhenGreenAskedAgainPostsAgainOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	// An earlier session announced #7 ready; asked, it is announced again
	// once CI passes, re-claimed from the claim that session took.
	var (
		posts atomic.Int32
		asked []string
	)

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{clitest.RunningStatus(), clitest.PassingStatus()},
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.Answering("y", &asked),
		"announce", "--when-green")

	// Assert
	if err != nil || posts.Load() != 1 || len(asked) != 1 || !strings.Contains(asked[0], "again") {
		t.Errorf("announce --when-green = %v, posted %d, asked %q, said %q; want it asked again and posted once",
			err, posts.Load(), asked, printed.Stderr)
	}
}

func TestPRAnnounceGivesUpOnAPullRequestTheForgeNeverLists(t *testing.T) {
	t.Parallel()

	// Arrange
	// C37: on a fake clock, the forge never lists the pull request just
	// opened; the wait gives up once ten minutes have passed.
	var posts atomic.Int32

	world := clockedWorld{
		responses: clitest.GhResponses{Pulls: "[]", Status: clitest.PassingStatus()}, hook: webhook(t, &posts),
	}

	// Act
	run := onAFakeClock(t, world, "pr", "--yes", "--announce")

	// Assert
	said := fmt.Sprint(run.err)
	if got := cli.ExitStatus(run.err); got != 4 || !strings.Contains(said, "the forge did not list #7 within 10m") ||
		posts.Load() != 0 {
		t.Errorf("pr --announce exited %d (%v), posted %d; want 4, #7 never listed, nothing posted",
			got, run.err, posts.Load())
	}

	if waits := strings.Count(run.printed.Stderr, "Waiting for the forge to list #7"); waits != 1 {
		t.Errorf("stderr said it waits for the forge %d times, want once:\n%s", waits, run.printed.Stderr)
	}
}

func askingToWait(t *testing.T, posts *atomic.Int32) string {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)

		posts.Add(1)

		writer.Header().Set("Connection", "close")
		writer.Header().Set("Retry-After", "30")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	return server.URL + "/services/x"
}

func TestAnnounceWhenGreenGivesUpOnAServiceThatAsksEveryPostToWait(t *testing.T) {
	t.Parallel()

	// Arrange
	// C16's command line twin: on a fake clock, CI passes and every post is
	// asked to wait; each is tried again until ten minutes have passed.
	var asked atomic.Int32

	world := clockedWorld{
		responses: clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()},
		hook:      askingToWait(t, &asked),
	}

	// Act
	run := onAFakeClock(t, world, "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(run.err); got != 5 ||
		!strings.Contains(fmt.Sprint(run.err), "could not post to Slack for 10m") {
		t.Errorf("announce --when-green exited %d (%v); want 5, saying Slack could not be posted to for 10m",
			got, run.err)
	}

	if asked.Load() < 2 || !strings.Contains(run.printed.Stderr, "Could not post to the channel its webhook is bound to") {
		t.Errorf("posted %d times, said %q; want each post tried again, and that said", asked.Load(), run.printed.Stderr)
	}
}

func TestPRAnnounceSaysAReadThatFailedAsTheForgeListsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The find after the open cannot be read; the one after it lists #7.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		PullsSequence: []string{"[]", "{", clitest.OpenPull("work")}, Status: clitest.PassingStatus(),
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.PretendPushed(t, repo)
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes", "--announce")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stderr, "Could not read CI on #7 (") {
		t.Errorf("pr --announce = %v, posted %d, said %q; want the failed read told, then one post",
			err, posts.Load(), printed.Stderr)
	}
}

func TestAnnounceWhenGreenGivesUpOnAPullRequestThatReportsNoCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	// On a fake clock, every read hears no check reported.
	var posts atomic.Int32

	world := clockedWorld{responses: clitest.GhResponses{Pulls: clitest.OpenPull("Add login")}, hook: webhook(t, &posts)}

	// Act
	run := onAFakeClock(t, world, "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(run.err); got != 4 ||
		!strings.Contains(fmt.Sprint(run.err), "no checks were reported on #7 within 10m") || posts.Load() != 0 {
		t.Errorf("announce --when-green exited %d (%v), posted %d; want 4, no checks reported, nothing posted",
			got, run.err, posts.Load())
	}
}

// storeInTheWay is a home where a file stands where its store goes, so the
// store can neither be read nor take a claim.
func storeInTheWay(t *testing.T) string {
	t.Helper()

	home := t.TempDir()

	storeDir, err := clitest.StoreDir(home)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(storeDir), 0o700)
	}

	if err == nil {
		err = os.WriteFile(storeDir, nil, 0o600)
	}

	if err != nil {
		t.Fatalf("putting a file where the store goes: %v", err)
	}

	return home
}

func TestAnnounceWhenGreenGivesUpOnAnAnnouncementTheStoreCannotClaim(t *testing.T) {
	t.Parallel()

	// Arrange
	// On a fake clock, CI passes and the store can never take the claim a
	// post needs first: nothing is posted, each post is tried again, and the
	// wait gives up once ten minutes have passed.
	var posts atomic.Int32

	world := clockedWorld{
		responses: clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()},
		hook:      webhook(t, &posts), home: storeInTheWay(t),
	}

	// Act
	run := onAFakeClock(t, world, "announce", "--when-green", "--yes")

	// Assert
	if got := cli.ExitStatus(run.err); got != 4 ||
		!strings.Contains(fmt.Sprint(run.err), "could not post to Slack for 10m") || posts.Load() != 0 {
		t.Errorf("announce --when-green exited %d (%v), posted %d; want 4, never posted, given up on",
			got, run.err, posts.Load())
	}

	if !strings.Contains(run.printed.Stderr, "Could not post to the channel its webhook is bound to (") {
		t.Errorf("stderr = %q, want each post the store could not claim told", run.printed.Stderr)
	}
}
