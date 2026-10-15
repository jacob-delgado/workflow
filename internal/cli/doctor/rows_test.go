// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

// The forge's row under --online, word for word, where it says why no
// credential could be put to the forge.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestDoctorOnlineSaysWhereAGitLabTokenIsReadWhenOnlyGlabIsSignedIn(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := clitest.RepoWithRemote(t, gitlabSSHRemote)
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))
	gitAndGlab(t)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	want := "none — glab is installed, but its login is not read: set $GITLAB_TOKEN, or set forge.token, " +
		"or turn on forge.cli to go through glab"
	if row := credentialRow(output, "forge"); row != want {
		t.Errorf("doctor's forge row = %q, want %q:\n%s", row, want, output)
	}
}

func TestDoctorOnlineSaysWhyAForgeKindItCannotUseLeavesNoForgeToAsk(t *testing.T) {
	t.Parallel()

	// Arrange
	// forge.kind names GitHub, but with no forge.host there is no host it is
	// said of, so the remote's own host stays unnamed.
	dir := clitest.RepoWithRemote(t, clitest.OnPremisesRemote)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}, `+
		clitest.SlackWebhook+`, "forge": {"kind": "github"}}`)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if row, want := credentialRow(output, "forge"), forge.ErrKindNeedsHost.Error(); row != want {
		t.Errorf("doctor's forge row = %q, want %q:\n%s", row, want, output)
	}
}
