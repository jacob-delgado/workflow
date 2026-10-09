// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPRReportsAFailedOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch is published, but the forge rejects the open.
	fakeGh(t, ghResponses{createError: true})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Errorf("pr = %v, want the failed open reported", err)
	}
}

func TestPRReportsAFailedPush(t *testing.T) {
	t.Parallel()

	// Arrange
	// The push remote does not exist, so the branch cannot be published and no
	// pull request is opened.
	fakeGh(t, ghResponses{})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	git(t, repo, "remote", "add", "push-target", filepath.Join(t.TempDir(), "nonexistent.git"))
	git(t, repo, "config", "remote.pushDefault", "push-target")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pushed") {
		t.Errorf("pr = %v, want a failed push reported", err)
	}

	wantExit(t, err, 1)
}

func TestPRRefusesASecondPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch already has an open pull request, which the forge reports.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("pr = %v, want it to refuse a second pull request", err)
	}

	wantExit(t, err, 4)
}

func TestPRRefusesASecondPullRequestInItsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refusal is the command line's own sentence, not the shared layer's.
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "an open pull request already exists for this branch") {
		t.Errorf("pr = %v, want the command line's refusal of a second pull request", err)
	}
}

func TestPullAlreadyOpenCarriesItsURL(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGh(t, ghResponses{pulls: openPull("Add login")})
	repo := githubRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "https://github.com/owner/repo/pull/7") {
		t.Errorf("pr = %v, want the refusal to carry the open pull request's address", err)
	}
}

func TestPRRefusesABranchWithNoCommits(t *testing.T) {
	t.Parallel()

	// Arrange
	// A branch level with main has nothing to propose.
	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")

	// Act
	_, err := run(t, repo, "pr", "--yes")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "commits") {
		t.Errorf("pr = %v, want a refusal that there are no commits to open", err)
	}

	wantExit(t, err, 4)
}
