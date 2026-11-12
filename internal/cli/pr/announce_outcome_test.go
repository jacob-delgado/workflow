// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAnnounceNamesTheChecksThatFailedAsItWasPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	// C82: CI passes on the read before the preview, and a check has failed
	// by the gate's own read as it is posted.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{clitest.PassingStatus(), clitest.FailingStatus()},
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, announcingConfig(webhook(t, &posts)))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	said := fmt.Sprint(err)
	if got := cli.ExitStatus(err); got != 4 || !strings.Contains(said, "a check failed on #7 (ci)") ||
		!strings.Contains(said, "workflow announce --when-green") || posts.Load() != 0 {
		t.Errorf("announce exited %d (%v), posted %d; want 4 naming ci and the way on, nothing posted",
			got, err, posts.Load())
	}
}

func TestAnnounceSaysCIItCouldNotReadAsItWasPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes on the read before the preview, and the gate's own read as
	// it is posted cannot be understood.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{
		Pulls: clitest.OpenPull("Add login"), StatusSequence: []string{clitest.PassingStatus(), "{"},
	})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, announcingConfig(webhook(t, &posts)))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	if !strings.Contains(fmt.Sprint(err), "announcing to Slack") || posts.Load() != 0 {
		t.Errorf("announce = %v, posted %d; want the unread CI reported, nothing posted", err, posts.Load())
	}
}

func TestAnnounceAskedAgainPostsAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// An earlier session announced #7 ready; asked, it is announced again,
	// re-claimed from the claim that session took.
	var (
		posts atomic.Int32
		asked []string
	)

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, announcingConfig(webhook(t, &posts)))
	home := clitest.AnnouncedEarlier(t, messaging.MomentReady)

	// Act
	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, clitest.Answering("y", &asked),
		"announce")

	// Assert
	if err != nil || posts.Load() != 1 || len(asked) != 1 || !strings.Contains(asked[0], "again") {
		t.Errorf("announce = %v, posted %d, asked %q, said %q; want it asked again and posted once",
			err, posts.Load(), asked, printed.Stderr)
	}
}

// refusing is a webhook that refuses every post as Slack refuses a payload
// it cannot take, counting each.
func refusing(t *testing.T, posts *atomic.Int32) string {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)

		posts.Add(1)

		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("invalid_payload"))
	}))
	t.Cleanup(server.Close)

	return server.URL + "/services/x"
}

func TestAnnounceWhenGreenGivesUpOnAPostTheServiceRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passes, and Slack refuses the post: it surely did not go, and is
	// not tried again.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, waitingConfig(refusing(t, &posts)))

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--when-green", "--yes")

	// Assert
	if !strings.Contains(fmt.Sprint(err), "Slack refused the post (") || posts.Load() != 1 {
		t.Errorf("announce --when-green = %v, posted %d; want the refusal said, the post tried once",
			err, posts.Load())
	}
}

// repairingPrompt answers yes to every question, first removing what stands
// where the store goes at path, so the store a command could not read before
// it asked can be written once it is answered.
func repairingPrompt(t *testing.T, path string) cmdkit.Prompt {
	t.Helper()

	answer := func(string) (string, error) {
		err := os.Remove(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("clearing the way for the store: %v", err)
		}

		return "y", nil
	}

	return cmdkit.Prompt{Line: answer, Secret: answer}
}

func TestAnnounceStillPostsOnceWhatWasAnnouncedCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// P020: a file stands where the store goes as the command reads what was
	// announced, and is gone by the time it posts.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.GithubRepo(t, armedBranch)
	clitest.WriteFile(t, repo, announcingConfig(webhook(t, &posts)))
	home := storeInTheWay(t)

	storeDir, err := clitest.StoreDir(home)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: repo, Home: home}, repairingPrompt(t, storeDir),
		"announce")

	// Assert
	if err != nil || posts.Load() != 1 {
		t.Fatalf("announce = %v, posted %d (%+v); want it announced once", err, posts.Load(), printed)
	}

	want := "Could not read what was announced"
	if !strings.Contains(printed.Stderr, want) || !strings.Contains(printed.Stderr, "keeps it from posting twice") {
		t.Errorf("stderr = %q, want %q, and that the claim still keeps it from posting twice", printed.Stderr, want)
	}
}
