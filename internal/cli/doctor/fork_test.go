// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
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
	clitest.GitInit(t, dir)
	clitest.Git(t, dir, "remote", "add", "origin", "https://github.com/contributor/repo.git")
	clitest.Git(t, dir, "remote", "add", "upstream", "https://github.com/owner/repo.git")

	return dir
}

func TestDoctorNamesTheBaseRemoteOfAFork(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := clitest.Run(t, forkClone(t), "doctor")

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
	printed, _ := clitest.RunStreams(t, forkClone(t), clitest.UnusedPrompt(t), "doctor", "--json")
	output := printed.Stdout

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
	clitest.GitInit(t, dir)
	clitest.Git(t, dir, "remote", "add", "origin", "https://github.com/owner/repo.git")

	// Act
	printed, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--json")
	output := printed.Stdout

	// Assert
	var report doctorRepository

	err := json.Unmarshal([]byte(output), &report)
	_, named := report.Repository["base_remote"]
	_, addressed := report.Repository["base"]

	if err != nil || named || addressed {
		t.Errorf("doctor --json repository = %v, %v; want no base outside a fork", report.Repository, err)
	}
}

// tokenUpstream is a fork's upstream whose address carries a credential.
const tokenUpstream = "https://user:secret-token@github.com/owner/repo.git"

// forkWithATokenUpstream is forkClone whose upstream's address carries a
// credential, as a remote cloned with one in its URL does.
func forkWithATokenUpstream(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	clitest.GitInit(t, dir)
	clitest.Git(t, dir, "remote", "add", "origin", "https://github.com/contributor/repo.git")
	clitest.Git(t, dir, "remote", "add", "upstream", tokenUpstream)

	return dir
}

func TestDoctorMasksACredentialInTheBaseRemote(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := clitest.Run(t, forkWithATokenUpstream(t), "doctor")

	// Assert
	if got := fieldValue(output, "Base remote"); got != "upstream (https://xxxxx@github.com/owner/repo.git)" ||
		strings.Contains(output, "secret-token") {
		t.Errorf("Base remote = %q, want the upstream's credential masked:\n%s", got, output)
	}
}

func TestDoctorJSONMasksACredentialInTheBaseRemote(t *testing.T) {
	t.Parallel()

	// Act
	printed, _ := clitest.RunStreams(t, forkWithATokenUpstream(t), clitest.UnusedPrompt(t), "doctor", "--json")

	// Assert
	var report doctorRepository

	err := json.Unmarshal([]byte(printed.Stdout), &report)
	if err != nil || report.Repository["base"] != "https://xxxxx@github.com/owner/repo.git" ||
		strings.Contains(printed.Stdout, "secret-token") {
		t.Errorf("doctor --json repository = %v, %v; want the upstream's credential masked", report.Repository, err)
	}
}

func TestDoctorNamesNoBaseRemoteOutsideAFork(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.GitInit(t, dir)
	clitest.Git(t, dir, "remote", "add", "origin", "https://github.com/owner/repo.git")

	// Act
	output, _ := clitest.Run(t, dir, "doctor")

	// Assert
	if strings.Contains(output, "Base remote:") {
		t.Errorf("doctor names a base remote for a repository that is no fork:\n%s", output)
	}
}
