// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStatusSaysNothingOnStderrWhenEveryServiceAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status")

	// Assert
	if err != nil || printed.stderr != "" {
		t.Errorf("status = %v, stderr %q, want no note when every service answered", err, printed.stderr)
	}
}

func TestStatusNamesTheForgeWhenItsPullRequestCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: "{"})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status", "--json")

	// Assert
	// The JSON is the data a script reads, unchanged; the note is beside it.
	if err != nil || !strings.Contains(printed.stdout, `"ci": "none"`) {
		t.Errorf("status --json = %v, stdout %q, want the status as data", err, printed.stdout)
	}

	if strings.Count(printed.stderr, "\n") != 1 || !strings.HasPrefix(printed.stderr, "GitHub could not be read: ") {
		t.Errorf("stderr = %q, want one line naming GitHub", printed.stderr)
	}
}

func TestStatusNamesJiraWhenTheIssueCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jiraServer(t, http.StatusInternalServerError, `{}`, new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.Contains(printed.stdout, "PROJ-7  ") {
		t.Errorf("status = %v, stdout %q, want the line without a summary", err, printed.stdout)
	}

	if !strings.HasPrefix(printed.stderr, "Jira could not be read: ") || strings.Contains(printed.stderr, "GitHub") {
		t.Errorf("stderr = %q, want a note naming Jira alone", printed.stderr)
	}
}

func TestStatusNamesTheWorkingTreeWhenItsChangesCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")

	err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.HasPrefix(printed.stderr, "The working tree could not be read: ") {
		t.Errorf("status = %v, stderr %q, want a note naming the working tree", err, printed.stderr)
	}
}

func TestStatusSaysTheForgeIsNotSetUpWhenOriginNamesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	// No origin, and no Jira: the forge is not set up, and the issue, read
	// from the forge's issues, is no number it could hold.
	repo := prRepo(t, "fix/PROJ-9-x")

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status")

	// Assert
	want := "The forge is not set up: origin does not name a repository on a forge; point it at the repository\n"
	if err != nil || printed.stderr != want {
		t.Errorf("status = %v, stderr %q, want only %q", err, printed.stderr, want)
	}
}

func TestStatusSaysTheForgeIsNotSetUpWhenItHasNoToken(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{signedOut: true})
	repo := statusFeatureRepo(t, server.URL)
	// Not through gh's own transport, so the forge needs a token, and none is
	// set or signed in.
	writeFile(t, repo, `{"jira":{"base_url":"`+server.URL+`","token":"t"},"forge":{"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.Contains(printed.stdout, "CI none") {
		t.Errorf("status = %v, stdout %q, want the line, CI none", err, printed.stdout)
	}

	if strings.Count(printed.stderr, "\n") != 1 || !strings.HasPrefix(printed.stderr, "GitHub is not set up: ") ||
		!strings.Contains(printed.stderr, "$GITHUB_TOKEN") {
		t.Errorf("stderr = %q, want one note saying GitHub is not set up and how to set it up", printed.stderr)
	}
}

func TestStatusAcrossLabelsEachNoteWithItsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	fakeGh(t, ghResponses{pulls: "{"})
	repo := statusFeatureRepo(t, server.URL)

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", repo)

	// Assert
	want := filepath.Base(repo) + ": GitHub could not be read: "
	if err != nil || !strings.HasPrefix(printed.stderr, want) {
		t.Errorf("status DIR = %v, stderr %q, want a note beginning %q", err, printed.stderr, want)
	}
}
