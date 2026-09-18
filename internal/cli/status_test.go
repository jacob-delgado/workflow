// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStatusOutsideARepositoryReportsSo(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "status")

	// Assert
	if err == nil {
		t.Error("status outside a repository returned no error")
	}
}

// featureRepo makes a repository on a branch for PROJ-2 with a commit, and
// returns its path.
func featureRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "init")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--allow-empty", "-m", "work")

	return repo
}

func TestStatusHereReportsTheCurrentRepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)

	// Act
	output, err := run(t, repo, "status")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "CI none") {
		t.Errorf("expected the current repository's status:\n%s", output)
	}
}

func TestStatusAcrossLabelsTheCurrentDirectory(t *testing.T) {
	// Act
	// "." names the working directory, which is no repository here.
	output, err := run(t, t.TempDir(), "status", ".")
	if err != nil {
		t.Fatalf("status .: %v (%s)", err, output)
	}

	// Assert
	if !strings.HasPrefix(output, ".  ") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected a labeled line for the current directory:\n%s", output)
	}
}

func TestStatusAcrossReportsEachRepository(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", repo, notRepo)
	if err != nil {
		t.Fatalf("status across directories: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "PROJ-2") || !strings.Contains(output, "not a git repository") {
		t.Errorf("expected the repository's issue and a note for the non-repository:\n%s", output)
	}
}

func TestStatusAcrossAsJSONIsAnArray(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	notRepo := t.TempDir()

	// Act
	output, err := run(t, t.TempDir(), "status", "--json", repo, notRepo)
	if err != nil {
		t.Fatalf("status --json across directories: %v (%s)", err, output)
	}

	// Assert
	var reports []struct {
		Repository string `json:"repository"`
		Issue      string `json:"issue"`
		Error      string `json:"error"`
	}

	err = json.Unmarshal([]byte(output), &reports)
	if err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, output)
	}

	if len(reports) != 2 || reports[0].Issue != "PROJ-2" || reports[1].Error == "" {
		t.Errorf("status JSON = %+v, want the repository and the non-repository's error", reports)
	}
}
