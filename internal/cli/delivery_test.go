// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"net/http"
	"net/http/httptest"
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
