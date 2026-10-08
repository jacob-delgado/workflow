// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

// webhook is a local incoming webhook over https that accepts every post, as
// Slack's does, and counts them in posts. The process trusts its certificate
// (see TestMain). The count is atomic because `task test` runs -race and the
// handler runs on its own goroutine.
func webhook(t *testing.T, posts *atomic.Int32) string {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)

		posts.Add(1)

		_, _ = writer.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	return server.URL + "/services/x"
}

// announcingConfig points the forge at its CLI, as forgeCLIConfig does, and
// messaging at a webhook that answers.
func announcingConfig(webhookURL string) string {
	return `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
		`"messaging":{"webhook_url":"` + webhookURL + `"}}`
}

func TestAnnounceYesDeliversTheAnnouncement(t *testing.T) {
	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, announcingConfig(webhook(t, &posts)))

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "announce", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("announce --yes = %v, want the announcement delivered (%+v)", err, printed)
	}

	if posts.Load() != 1 || !strings.Contains(printed.stderr, "Announced to the channel its webhook is bound to.\n") {
		t.Errorf("announce --yes posted %d times and said:\n%s\nwant one post, and that it was announced",
			posts.Load(), printed.stderr)
	}
}

func TestAnnounceRemembersADeliveredAnnouncement(t *testing.T) {
	// Arrange
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, announcingConfig(webhook(t, &posts)))

	where := place{dir: repo, home: t.TempDir()}

	_, err := runStreamsAt(t, where, unusedPrompt(t), "announce", "--yes")
	if err != nil {
		t.Fatalf("the first announce --yes = %v, want the announcement delivered", err)
	}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "announce", "--yes")

	// Assert
	if err != nil || posts.Load() != 1 || !strings.Contains(printed.stderr, alreadyAnnounced) {
		t.Errorf("a second announce --yes = %v after %d posts, saying:\n%s\nwant the first remembered and none posted again",
			err, posts.Load(), printed.stderr)
	}
}

func TestAnnounceRefusedForItsChannelSaysTheFixAndNoKey(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	fakeSlack(t, map[string]slackAnswer{slackPostMessage: {http.StatusOK, `{"ok":false,"error":"not_in_channel"}`}})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, slackLoggedInConfig())

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	// The error is what main prints on stderr: the fix, and no key to press,
	// since a command line has none.
	if err == nil || !strings.Contains(err.Error(), "you are not in #dev; join it") ||
		strings.Contains(err.Error(), "press enter") {
		t.Errorf("announce = %v, want the fix for the channel and no key named", err)
	}
}

func TestAnnounceDeliveredButNotRememberedSucceedsAndSaysSo(t *testing.T) {
	// Arrange
	// A file stands where the store's directory goes, so nothing is kept.
	var posts atomic.Int32

	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, announcingConfig(webhook(t, &posts)))

	where := place{dir: repo, home: t.TempDir(), state: t.TempDir()}
	writeStoreBlocker(t, where.state)

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "announce", "--yes")

	// Assert
	// The line saying where it went, then the sentence every surface says of
	// it, then why.
	saidSo := regexp.MustCompile(`(?m)^Announced to .+\.\n` +
		regexp.QuoteMeta("Posted, but not remembered: it may be offered again.") + ` \S`)
	if err != nil || posts.Load() != 1 || !saidSo.MatchString(printed.stderr) {
		t.Errorf("announce --yes = %v after %d posts, saying:\n%s\nwant the announcement made and the store's "+
			"failure said", err, posts.Load(), printed.stderr)
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
