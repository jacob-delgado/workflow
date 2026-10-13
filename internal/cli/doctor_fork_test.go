// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// doctorRepository is the repository section of doctor --json, as a script
// reads it.
type doctorRepository struct {
	Repository map[string]any `json:"repository"`
}

// forkClone is a repository whose origin is a fork and whose upstream is the
// repository it was forked from.
func forkClone(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "remote", "add", "origin", "https://github.com/contributor/repo.git")
	git(t, dir, "remote", "add", "upstream", "https://github.com/owner/repo.git")

	return dir
}

func TestDoctorNamesTheBaseRemoteOfAFork(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := run(t, forkClone(t), "doctor")

	// Assert
	if got := fieldValue(output, "Base remote"); got != "upstream (https://github.com/owner/repo.git)" {
		t.Errorf("Base remote = %q, want upstream and its address:\n%s", got, output)
	}

	if got := fieldValue(output, "Forge"); !strings.Contains(got, "owner/repo") {
		t.Errorf("Forge = %q, want the base's repository:\n%s", got, output)
	}
}

func TestDoctorJSONNamesTheBaseRemoteOfAFork(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := run(t, forkClone(t), "doctor", "--json")

	// Assert
	var report doctorRepository

	err := json.Unmarshal([]byte(output), &report)
	if err != nil || report.Repository["base_remote"] != "upstream" ||
		report.Repository["base"] != "https://github.com/owner/repo.git" {
		t.Errorf("doctor --json repository = %v, %v; want base_remote upstream and its address", report.Repository, err)
	}
}

func TestDoctorJSONNamesNoBaseOutsideAFork(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "remote", "add", "origin", "https://github.com/owner/repo.git")

	// Act
	output, _ := run(t, dir, "doctor", "--json")

	// Assert
	var report doctorRepository

	err := json.Unmarshal([]byte(output), &report)
	_, named := report.Repository["base_remote"]
	_, addressed := report.Repository["base"]

	if err != nil || named || addressed {
		t.Errorf("doctor --json repository = %v, %v; want no base outside a fork", report.Repository, err)
	}
}
