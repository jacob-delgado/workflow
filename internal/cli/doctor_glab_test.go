// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

// gitlabSSHRemote is a GitLab origin written as an SSH remote.
const gitlabSSHRemote = "git@gitlab.com:group/proj.git"

// toolingRow is the Tooling section's line for a program, or empty.
func toolingRow(output, program string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), program+" ") {
			return line
		}
	}

	return ""
}

// gitAndGlab puts git and a fake glab on PATH, and nothing else.
func gitAndGlab(t *testing.T) {
	t.Helper()

	pathWithOnlyGit(t)
	fakeGlab(t)
}

func TestDoctorReportsGlabBesideGh(t *testing.T) {
	// Arrange
	gitAndGlab(t)

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if row := toolingRow(output, "glab"); !strings.Contains(row, "found") || strings.Contains(row, "not found") {
		t.Errorf("the glab row = %q, want it found:\n%s", row, output)
	}

	if row := toolingRow(output, "gh"); !strings.Contains(row, "not found") {
		t.Errorf("the gh row = %q, want it listed as not found:\n%s", row, output)
	}
}

func TestDoctorJSONListsGlab(t *testing.T) {
	// Arrange
	gitAndGlab(t)

	// Act
	output, _ := run(t, t.TempDir(), "doctor", "--json")

	// Assert
	tooling, _ := decodeReport(t, output)["tooling"].([]any)
	for _, entry := range tooling {
		if fact, _ := entry.(map[string]any); fact["name"] == "glab" {
			if fact["found"] != true {
				t.Errorf("the glab row = %v, want found", fact)
			}

			return
		}
	}

	t.Errorf("doctor --json lists no glab among %v", tooling)
}

func TestDoctorSaysForgeCLIHasNoGlabToGoThrough(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, gitlabSSHRemote)
	pathWithOnlyGit(t)

	// Act
	output, _ := run(t, dir, "doctor")

	// Assert
	if row := toolingRow(output, "glab"); !strings.Contains(row, "forge.cli is on") {
		t.Errorf("the glab row = %q, want it to say forge.cli has nothing to go through:\n%s", row, output)
	}
}

func TestDoctorOnlineSaysGlabsLoginIsNotRead(t *testing.T) {
	// Arrange
	clearForgeEnvironment(t)
	dir := repoWithRemote(t, gitlabSSHRemote)
	writeConfigFor(t, dir, workingJira(t))
	gitAndGlab(t)

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	for _, want := range []string{"glab is installed", "$GITLAB_TOKEN", "forge.cli"} {
		if !strings.Contains(output, want) {
			t.Errorf("doctor --online does not say %q about glab's login:\n%s", want, output)
		}
	}
}

func TestDoctorOnlineWarnsATokenThatCannotWriteToGitLab(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, gitlabSSHRemote)
	fakeGlabWithScopes(t, `{"scopes":["read_api","read_user"]}`)

	// Act
	output, err := run(t, dir, "doctor", "--online")
	// Assert
	if err != nil {
		t.Errorf("doctor --online = %v, want a token that reads to pass with a warning", err)
	}

	if !strings.Contains(output, "can read but not write: it has read_api, not api") {
		t.Errorf("doctor --online does not warn of a read-only token:\n%s", output)
	}
}

func TestDoctorOnlineSaysNothingOfAScopeItCannotRead(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, gitlabSSHRemote)
	fakeGlab(t)

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	if strings.Contains(output, "can read but not write") {
		t.Errorf("doctor --online warned of scopes it could not read:\n%s", output)
	}
}

func TestDoctorOnlineWarnsAGitLabTokenThatCannotUseTheAPI(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, gitlabSSHRemote)
	fakeGlabWithScopes(t, `{"scopes":["read_user"]}`)

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	if !strings.Contains(output, "cannot use the API: it has read_user, not api") {
		t.Errorf("doctor --online does not warn of a token without API access:\n%s", output)
	}
}
