// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestStatusAcrossLabelsARelativeDirectoryByWhereItLeads(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		dir  string
		want string
	}{
		"the working directory": {dir: ".", want: "api"},
		"its parent":            {dir: "..", want: "work"},
		"itself, from above":    {dir: "../api/", want: "api"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// None of these is a repository, so each line says so after its label.
			working := filepath.Join(t.TempDir(), "work", "api")
			makeDirectory(t, working)

			// Act
			printed, err := clitest.RunStreams(t, working, clitest.UnusedPrompt(t), "status", tt.dir)

			// Assert
			if !strings.HasPrefix(printed.Stdout, tt.want+"  ") ||
				!strings.Contains(printed.Stdout, "not a git repository") {
				t.Errorf("status %s from %s printed:\n%s\nwant its line labeled %q",
					tt.dir, working, printed.Stdout, tt.want)
			}

			wantExit(t, err, 4)
		})
	}
}

func TestStatusAcrossAsJSONLabelsTheCurrentRepositoryByItsName(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "status", "--json", ".")

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Dir        string `json:"dir"`
		Issue      string `json:"issue"`
	}

	decodeErr := json.Unmarshal([]byte(printed.Stdout), &reports)
	if decodeErr != nil || len(reports) != 1 || reports[0].Repository != filepath.Base(repo) ||
		reports[0].Dir != "." || reports[0].Issue != "PROJ-2" {
		t.Errorf("status --json . = %+v (%v), want the repository named %q, its directory as given",
			reports, decodeErr, filepath.Base(repo))
	}

	wantExit(t, err, 0)
}

func TestStatusAcrossReportsEachRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := clitest.Run(t, t.TempDir(), "status", notRepo, repo)

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
	output, err := clitest.Run(t, t.TempDir(), "status", repo)

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
	output, err := clitest.Run(t, t.TempDir(), "status", repo)

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
	clitest.WriteFile(t, repo, "{not json")

	// Act
	output, err := clitest.Run(t, t.TempDir(), "status", repo)

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
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "status", "--json", notRepo, repo)

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Issue      string `json:"issue"`
		Error      string `json:"error"`
	}

	decodeErr := json.Unmarshal([]byte(printed.Stdout), &reports)
	if decodeErr != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", decodeErr, printed.Stdout)
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
		makeDirectory(t, dir)
	}

	return first, second
}

// makeDirectory makes dir and every directory above it that is missing.
func makeDirectory(t *testing.T, dir string) {
	t.Helper()

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		t.Fatalf("making %s: %v", dir, err)
	}
}

func TestStatusAcrossLabelsDirectoriesOfOneNameApart(t *testing.T) {
	t.Parallel()

	// Arrange
	first, second := twoOfOneName(t)

	// Act
	printed, _ := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "status", first, second)

	// Assert
	lines := strings.Split(strings.TrimSpace(printed.Stdout), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], first+"  ") || !strings.HasPrefix(lines[1], second+"  ") {
		t.Errorf("status of two directories named api printed:\n%s\nwant each line labeled with its path",
			printed.Stdout)
	}
}

func TestStatusAcrossAsJSONNamesEachDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	first, second := twoOfOneName(t)

	// Act
	printed, _ := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "status", "--json", first, second)

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Dir        string `json:"dir"`
	}

	err := json.Unmarshal([]byte(printed.Stdout), &reports)
	if err != nil || len(reports) != 2 || reports[0].Dir != first || reports[1].Dir != second ||
		reports[0].Repository == reports[1].Repository {
		t.Errorf("status --json = %+v (%v), want each entry labeled apart and naming its directory", reports, err)
	}
}

func TestStatusAcrossLabelsRelativeDirectoriesOfOneNameByTheirPathsFromHome(t *testing.T) {
	t.Parallel()

	// Arrange
	first, _ := twoOfOneName(t)
	parent := filepath.Dir(filepath.Dir(first))

	// Act
	printed, _ := clitest.RunStreamsAt(
		t, clitest.Place{Dir: parent, Home: parent}, clitest.UnusedPrompt(t), "status", "work/api", "oss/api",
	)

	// Assert
	lines := strings.Split(strings.TrimSpace(printed.Stdout), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "~/work/api  ") || !strings.HasPrefix(lines[1], "~/oss/api  ") {
		t.Errorf("status work/api oss/api from home printed:\n%s\nwant each line labeled with its path from home",
			printed.Stdout)
	}
}
