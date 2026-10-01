// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ownedRepo is a GitHub repository whose main holds a CODEOWNERS giving the
// API to ana, a team and octo — the fake gh's own user — on a branch that
// changes the API and is already pushed.
func ownedRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	writeRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @ana @acme/control-plane @octo\n")
	git(t, repo, "add", ".")
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	writeRepoFile(t, repo, "api/pull.go", "package api\n")
	git(t, repo, "add", ".")
	commit(t, repo, "fix: guard the api")
	git(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

func TestPRPreviewsTheCodeOwnersAsReviewers(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{})
	repo := ownedRepo(t)

	var asked []string

	// Act
	printed, err := runStreams(t, repo, answering("n", &asked), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stdout, "  reviewers ana, acme/control-plane (code owners)\n") {
		t.Errorf("pr printed %q, want the code owners but its author as reviewers", printed.stdout)
	}
}

func TestPRRequestsTheCodeOwnersReview(t *testing.T) {
	// Arrange
	sent := filepath.Join(t.TempDir(), "sent")
	fakeGh(t, ghResponses{sentLog: sent})
	repo := ownedRepo(t)

	// Act
	_, err := run(t, repo, "pr", "--yes")
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
	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	output, err := run(t, repo, "pr", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%s)", err, output)
	}

	if strings.Contains(output, "reviewers") {
		t.Errorf("pr printed %q, want no reviewers line", output)
	}
}
