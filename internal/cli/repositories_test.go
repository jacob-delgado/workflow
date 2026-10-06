// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// repositoriesReport is what `workflow repositories --json` prints, as far as
// these tests read it.
type repositoriesReport struct {
	Here struct {
		Dir  string `json:"dir"`
		Root string `json:"root"`
	} `json:"here"`
	Worktrees []struct {
		Dir    string `json:"dir"`
		Branch string `json:"branch"`
		State  string `json:"state"`
	} `json:"worktrees"`
	WorktreesError string            `json:"worktrees_error"`
	Favorites      []json.RawMessage `json:"favorites"`
	FavoritesKept  bool              `json:"favorites_kept"`
}

// decodeRepositories parses stdout as `repositories --json`'s report, failing
// the test when it is anything else.
func decodeRepositories(t *testing.T, stdout string) repositoriesReport {
	t.Helper()

	var report repositoriesReport

	err := json.Unmarshal([]byte(stdout), &report)
	if err != nil {
		t.Fatalf("repositories --json printed something other than JSON: %v\n%s", err, stdout)
	}

	return report
}

// repoWithWorktree is a repository, its root as git names it, and a second
// worktree beside it on the branch "other".
func repoWithWorktree(t *testing.T) (string, string) {
	t.Helper()

	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(parent, "api")
	git(t, parent, "init", "--quiet", "api")
	commit(t, repo, "init")

	second := filepath.Join(parent, "api-other")
	git(t, repo, "worktree", "add", "--quiet", "-b", "other", second)

	return repo, second
}

func TestRepositoriesAsJSONListsTheWorktrees(t *testing.T) {
	// Arrange
	repo, second := repoWithWorktree(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "repositories", "--json")
	// Assert
	if err != nil {
		t.Fatalf("repositories --json: %v (%+v)", err, printed)
	}

	report := decodeRepositories(t, printed.stdout)
	if report.Here.Dir != repo || report.Here.Root != repo {
		t.Errorf("here = %+v, want %s, the repository's root", report.Here, repo)
	}

	if len(report.Worktrees) != 2 || report.Worktrees[0].Dir != repo || report.Worktrees[0].State != "here" ||
		report.Worktrees[1].Dir != second || report.Worktrees[1].Branch != "other" {
		t.Errorf("worktrees = %+v, want this one here, then %s on other", report.Worktrees, second)
	}
}

func TestRepositoriesAsJSONSaysFavoritesCanBeKept(t *testing.T) {
	// Arrange
	repo, _ := repoWithWorktree(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "repositories", "--json")
	// Assert
	if err != nil {
		t.Fatalf("repositories --json: %v (%+v)", err, printed)
	}

	report := decodeRepositories(t, printed.stdout)
	if report.Favorites == nil || len(report.Favorites) != 0 || !report.FavoritesKept {
		t.Errorf("favorites = %v, kept %v; want an empty list a store keeps", report.Favorites, report.FavoritesKept)
	}
}

func TestRepositoriesListsWhereItWorks(t *testing.T) {
	// Arrange
	repo, second := repoWithWorktree(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "repositories")
	// Assert
	if err != nil {
		t.Fatalf("repositories: %v (%+v)", err, printed)
	}

	lines := strings.Split(strings.TrimSpace(printed.stdout), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "here") || !strings.Contains(lines[0], repo) ||
		!strings.HasPrefix(lines[2], "worktree") || !strings.Contains(lines[2], second) ||
		!strings.Contains(lines[2], "other") {
		t.Errorf("repositories printed:\n%s\nwant where it works, then each worktree with its branch", printed.stdout)
	}
}

func TestRepositoriesOutsideARepositoryHasNoWorktrees(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "repositories", "--json")
	// Assert
	if err != nil {
		t.Fatalf("repositories --json: %v (%+v)", err, printed)
	}

	report := decodeRepositories(t, printed.stdout)
	if report.Worktrees == nil || len(report.Worktrees) != 0 || report.Here.Root != "" {
		t.Errorf("outside a repository repositories --json printed:\n%s\nwant no root and an empty worktree list",
			printed.stdout)
	}
}
