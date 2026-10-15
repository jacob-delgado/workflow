// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// announcingConfig points the forge at its CLI, as clitest.ForgeCLIConfig does, and
// messaging at a webhook that answers.
func announcingConfig(webhookURL string) string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
		`"messaging":{"webhook_url":"` + webhookURL + `"}}`
}

func TestAnnounceYesDeliversTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, announcingConfig(clitest.Webhook(t, &posts)))

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "announce", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes = %v, want the announcement delivered (%+v)", err, printed)
	}

	if posts.Load() != 1 || !strings.Contains(printed.Stderr, "Announced to the channel its webhook is bound to.\n") {
		t.Errorf("announce --yes posted %d times and said:\n%s\nwant one post, and that it was announced",
			posts.Load(), printed.Stderr)
	}
}

func TestAnnounceRemembersADeliveredAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, announcingConfig(clitest.Webhook(t, &posts)))

	where := clitest.Place{Dir: repo, Home: t.TempDir()}

	_, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "announce", "--yes")
	if err != nil {
		t.Fatalf("the first announce --yes = %v, want the announcement delivered", err)
	}

	// Act
	printed, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.Stderr, alreadyAnnounced) {
		t.Errorf("a second announce --yes = %v after %d posts, saying:\n%s\nwant the first remembered and none posted again",
			err, posts.Load(), printed.Stderr)
	}
}

func TestAnnounceRefusedForItsChannelSaysTheFixAndNoKey(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	clitest.FakeSlack(t, map[string]clitest.SlackAnswer{
		clitest.SlackPostMessage: {Status: http.StatusOK, Body: `{"ok":false,"error":"not_in_channel"}`},
	})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.SlackLoggedInConfig())

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	// The error is what main prints on stderr: the fix, and no key to press,
	// since a command line has none.
	if err == nil || !strings.Contains(err.Error(), "you are not in #dev; join it") ||
		strings.Contains(err.Error(), "press enter") {
		t.Errorf("announce = %v, want the fix for the channel and no key named", err)
	}
}

func TestAnnounceDeliveredButNotRememberedSucceedsAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file stands where the store's directory goes, so nothing is kept.
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, announcingConfig(clitest.Webhook(t, &posts)))

	where := clitest.Place{Dir: repo, Home: t.TempDir(), State: t.TempDir()}
	writeStoreBlocker(t, where.State)

	// Act
	printed, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "announce", "--yes")

	// Assert
	// The line saying where it went, then the sentence every surface says of
	// it, then why, straight after it: the store's directory could not be
	// made.
	saidSo := regexp.MustCompile(`(?m)^Announced to .+\.\n` +
		regexp.QuoteMeta("WARN Posted, but not remembered: it may be offered again. creating the store directory: "))
	if err != nil || posts.Load() != 1 || !saidSo.MatchString(printed.Stderr) {
		t.Errorf("announce --yes = %v after %d posts, saying:\n%s\nwant the announcement made and the store's "+
			"failure said", err, posts.Load(), printed.Stderr)
	}
}

// writeStoreBlocker puts a file where the store's directory goes under
// state, $XDG_STATE_HOME, so the store can keep nothing.
func writeStoreBlocker(t *testing.T, state string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(state, "workflow"), nil, 0o600)
	if err != nil {
		t.Fatalf("putting a file where the store goes: %v", err)
	}
}
