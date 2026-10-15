// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestPRPreviewsTheCodeOwnersAsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := ownedRepo(t)

	var asked []string

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.Stdout, "  reviewers ana, acme/control-plane (code owners)\n") {
		t.Errorf("pr printed %q, want the code owners but its author as reviewers", printed.Stdout)
	}
}

func TestPRRequestsTheCodeOwnersReview(t *testing.T) {
	t.Parallel()

	// Arrange
	sent := filepath.Join(t.TempDir(), "sent")
	clitest.FakeGh(t, clitest.GhResponses{SentLog: sent})
	repo := ownedRepo(t)

	// Act
	_, err := clitest.Run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v", err)
	}

	log, _ := os.ReadFile(sent)
	requested := string(log)

	person, team := `"reviewers":["ana"]`, `"team_reviewers":["control-plane"]`
	if !strings.Contains(requested, person) || !strings.Contains(requested, team) {
		t.Errorf("gh was sent %s, want ana as a reviewer and control-plane as a team", log)
	}
}

func TestPRPreviewsNoReviewersWithoutCodeOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	output, err := clitest.Run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if strings.Contains(output, "reviewers") {
		t.Errorf("pr printed %q, want no reviewers line", output)
	}
}

func TestPRSaysTheMergeRequestOpenedWhenAReviewerCouldNotBeAdded(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab knows no user named ghost, so the merge request opens without
	// that reviewer; it is open all the same.
	clitest.FakeGlab(t)
	repo := t.TempDir()
	gitInit(t, repo)
	writeRepoFile(t, repo, "CODEOWNERS", "* @ghost\n")
	git(t, repo, "add", ".")
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	writeRepoFile(t, repo, "api/pull.go", "package api\n")
	git(t, repo, "add", ".")
	commit(t, repo, "fix: guard the api")
	git(t, repo, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"gitlab","host":"gitlab.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes = %v (%+v), want it to succeed: the merge request is open", err, printed)
	}

	if !strings.Contains(printed.Stdout, "Opened !7 "+clitest.GitlabMergeRequest) ||
		!strings.Contains(printed.Stderr, "ghost") {
		t.Errorf("pr printed %+v, want the merge request opened and ghost named as not added", printed)
	}
}
