// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusAcrossLabelsTheCurrentDirectory(t *testing.T) {
	t.Parallel()

	// Act
	// "." names the working directory, which is no repository here.
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", ".")

	// Assert
	if !strings.HasPrefix(printed.stdout, ".  ") || !strings.Contains(printed.stdout, "not a git repository") {
		t.Errorf("expected a labeled line for the current directory:\n%s", printed.stdout)
	}

	wantExit(t, err, 4)
}

func TestStatusAcrossReportsEachRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", notRepo, repo)

	// Assert
	// Every directory gets its line, even after one that cannot be read, and
	// the one that is no repository still fails the command, as bare status
	// fails outside one.
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected the repository's issue and a note for the non-repository:\n%s", output)
	}

	wantExit(t, err, 4)
}

func TestStatusAcrossPassesWhenEveryDirectoryIsARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if err != nil || !strings.Contains(output, "PROJ-2") {
		t.Errorf("status across a repository = %v, want its line and success:\n%s", err, output)
	}
}

func TestStatusAcrossSaysWhyARepositoryCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository whose branch name holds a character with no width: it is a
	// repository, but its branch cannot be shown as it is, so it is not read.
	repo := featureRepo(t)
	git(t, repo, "checkout", "-b", "fix/PROJ\u200b-3")

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if strings.Contains(output, "not a git repository") || !strings.Contains(output, "cannot be shown") {
		t.Errorf("status called a repository it could not read no repository:\n%s", output)
	}

	wantExit(t, err, 1)
}

func TestStatusAcrossRefusesADirectoryWhoseConfigurationCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)
	writeFile(t, repo, "{not json")

	// Act
	output, err := run(t, t.TempDir(), "status", repo)

	// Assert
	if strings.Contains(output, "PROJ-2") || !strings.Contains(output, "invalid") {
		t.Errorf("status read a repository whose configuration it could not:\n%s", output)
	}

	wantExit(t, err, 3)
}

func TestStatusAcrossAsJSONIsAnArray(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "status", "--json", notRepo, repo)

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Issue      string `json:"issue"`
		Error      string `json:"error"`
	}

	decodeErr := json.Unmarshal([]byte(printed.stdout), &reports)
	if decodeErr != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", decodeErr, printed.stdout)
	}

	if len(reports) != 2 || reports[0].Error != "not a git repository" || reports[1].Issue != "PROJ-2" {
		t.Errorf("status JSON = %+v, want the repository and the non-repository's error", reports)
	}

	wantExit(t, err, 4)
}

// twoOfOneName are two directories, neither a repository, that share the base
// name api, as ~/work/api and ~/oss/api do.
func twoOfOneName(t *testing.T) (string, string) {
	t.Helper()

	parent := t.TempDir()
	first, second := filepath.Join(parent, "work", "api"), filepath.Join(parent, "oss", "api")

	for _, dir := range []string{first, second} {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			t.Fatalf("making %s: %v", dir, err)
		}
	}

	return first, second
}

func TestStatusAcrossLabelsDirectoriesOfOneNameApart(t *testing.T) {
	t.Parallel()

	// Arrange
	first, second := twoOfOneName(t)

	// Act
	printed, _ := runStreams(t, t.TempDir(), unusedPrompt(t), "status", first, second)

	// Assert
	lines := strings.Split(strings.TrimSpace(printed.stdout), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], first+"  ") || !strings.HasPrefix(lines[1], second+"  ") {
		t.Errorf("status of two directories named api printed:\n%s\nwant each line labeled with its path",
			printed.stdout)
	}
}

func TestStatusAcrossAsJSONNamesEachDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	first, second := twoOfOneName(t)

	// Act
	printed, _ := runStreams(t, t.TempDir(), unusedPrompt(t), "status", "--json", first, second)

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Dir        string `json:"dir"`
	}

	err := json.Unmarshal([]byte(printed.stdout), &reports)
	if err != nil || len(reports) != 2 || reports[0].Dir != first || reports[1].Dir != second ||
		reports[0].Repository == reports[1].Repository {
		t.Errorf("status --json = %+v (%v), want each entry labeled apart and naming its directory", reports, err)
	}
}
