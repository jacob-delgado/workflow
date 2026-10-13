// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forkRepo is a fork's clone on a branch with a commit of its own: origin is
// the contributor's fork, whose pushes land in a bare repository on disk, and
// upstream the repository it was forked from, fetched. It returns the clone and
// the bare repository.
func forkRepo(t *testing.T, branch string) (string, string) {
	t.Helper()

	repo := prRepo(t, branch)
	bare := t.TempDir()
	git(t, bare, "init", "--bare", "--quiet")
	git(t, repo, "remote", "add", "origin", "https://github.com/contributor/repo.git")
	git(t, repo, "config", "remote.origin.pushurl", bare)
	git(t, repo, "remote", "add", "upstream", "https://github.com/owner/repo.git")
	git(t, repo, "update-ref", "refs/remotes/upstream/main", "main")

	return repo, bare
}

// In a fork's clone the pull request opens on upstream, from the branch on
// the fork origin names: GitHub's owner:branch head.
func TestPROpensOnTheBaseFromAForksPushRemote(t *testing.T) {
	t.Parallel()

	// Arrange
	sent := filepath.Join(t.TempDir(), "sent")
	fakeGh(t, ghResponses{sentLog: sent})
	repo, bare := forkRepo(t, "fix/PROJ-2-thing")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	output, err := run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%s)", err, output)
	}

	if !strings.Contains(gitOutput(t, bare, "branch", "--list", "fix/PROJ-2-thing"), "fix/PROJ-2-thing") {
		t.Error("the branch was not pushed to the fork before opening")
	}

	log, _ := os.ReadFile(sent)
	if !strings.Contains(string(log), "repos/owner/repo/pulls") ||
		!strings.Contains(string(log), `"head":"contributor:fix/PROJ-2-thing"`) ||
		!strings.Contains(string(log), `"base":"main"`) {
		t.Errorf("gh was sent %s, want the pull request opened on owner/repo from contributor's branch into main", log)
	}
}
