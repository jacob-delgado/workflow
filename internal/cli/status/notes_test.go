// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestStatusSaysNothingOnStderrWhenEveryServiceAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")

	// Assert
	if err != nil || printed.Stderr != "" {
		t.Errorf("status = %v, stderr %q, want no note when every service answered", err, printed.Stderr)
	}
}

func TestStatusNamesTheForgeWhenItsPullRequestCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "{"})
	repo := clitest.JiraFeatureRepo(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status", "--json")

	// Assert
	// The JSON is the data a script reads, unchanged; the note is beside it.
	if err != nil || !strings.Contains(printed.Stdout, `"ci": "none"`) {
		t.Errorf("status --json = %v, stdout %q, want the status as data", err, printed.Stdout)
	}

	if strings.Count(printed.Stderr, "\n") != 1 || !strings.HasPrefix(printed.Stderr, "GitHub could not be read: ") {
		t.Errorf("stderr = %q, want one line naming GitHub", printed.Stderr)
	}
}

func TestStatusNamesJiraWhenTheIssueCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusInternalServerError, `{}`, new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
	repo := clitest.JiraFeatureRepo(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "PROJ-7  ") {
		t.Errorf("status = %v, stdout %q, want the line without a summary", err, printed.Stdout)
	}

	if !strings.HasPrefix(printed.Stderr, "Jira could not be read: ") || strings.Contains(printed.Stderr, "GitHub") {
		t.Errorf("stderr = %q, want a note naming Jira alone", printed.Stderr)
	}
}

func TestStatusNamesTheWorkingTreeWhenItsChangesCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := t.TempDir()
	clitest.GitInit(t, repo)
	clitest.Commit(t, repo, "init")

	err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.HasPrefix(printed.Stderr, "The working tree could not be read: ") {
		t.Errorf("status = %v, stderr %q, want a note naming the working tree", err, printed.Stderr)
	}
}

func TestStatusSaysTheForgeIsNotSetUpWhenOriginNamesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	// No origin, and no Jira: the forge is not set up, and the issue, read
	// from the forge's issues, is no number it could hold.
	repo := clitest.PRRepo(t, "fix/PROJ-9-x")

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")

	// Assert
	want := "The forge is not set up: the base remote (upstream in a fork's clone, else origin), " +
		"or a branch's push remote, does not name a repository on a forge; point it at the repository\n"
	if err != nil || printed.Stderr != want {
		t.Errorf("status = %v, stderr %q, want only %q", err, printed.Stderr, want)
	}
}

func TestStatusSaysTheForgeIsNotSetUpWhenItHasNoToken(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{SignedOut: true})
	repo := clitest.JiraFeatureRepo(t, server.URL)
	// Not through gh's own transport, so the forge needs a token, and none is
	// set or signed in.
	clitest.WriteFile(
		t, repo, `{"jira":{"base_url":"`+server.URL+`","token":"t"},"forge":{"kind":"github","host":"github.com"}}`,
	)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "CI none") {
		t.Errorf("status = %v, stdout %q, want the line, CI none", err, printed.Stdout)
	}

	if strings.Count(printed.Stderr, "\n") != 1 || !strings.HasPrefix(printed.Stderr, "GitHub is not set up: ") ||
		!strings.Contains(printed.Stderr, "$GITHUB_TOKEN") {
		t.Errorf("stderr = %q, want one note saying GitHub is not set up and how to set it up", printed.Stderr)
	}
}

func TestStatusAcrossLabelsEachNoteWithItsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "{"})
	repo := clitest.JiraFeatureRepo(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "status", repo)

	// Assert
	want := filepath.Base(repo) + ": GitHub could not be read: "
	if err != nil || !strings.HasPrefix(printed.Stderr, want) {
		t.Errorf("status DIR = %v, stderr %q, want a note beginning %q", err, printed.Stderr, want)
	}
}
