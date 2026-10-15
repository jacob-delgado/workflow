// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

// The prose report, word for word, for a directory outside any repository
// with only git on PATH, and the verdict and Credentials section a run of
// --online on the test's own PATH ends with: what a person pastes into a bug
// report is what these pin. The store's directory is the platform's, so it is
// read back rather than written out.

import (
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

// notChecked is the report's last line when --online is left off.
const notChecked = "Credentials were not checked. Add --online to ask Jira, your forge and Slack; " +
	"a webhook is left unchecked.\n"

// webhookJira is the configuration section of completeConfig.
func webhookJira(path string) string {
	return "Configuration: " + path + "\n" +
		"Tracker:       Jira at https://jira.example.com\n" +
		"Jira:          https://jira.example.com (bearer token)\n" +
		"Slack:         the channel its webhook is bound to (incoming webhook)\n"
}

// reportHead is every section the report gives before the configuration's,
// for dir outside any repository, with only git on PATH and the store in
// store.
func reportHead(dir, store string) string {
	return "Version:       " + buildinfo.Current() + "\n\n" +
		"Repository:    (none — " + dir + " is not in a git work tree)\n\n" +
		"Tooling:\n" +
		"  git        found\n" +
		"  lefthook   not found — the hook keys are not offered without it\n" +
		"  gh         not found — supplies a GitHub token, and carries GitHub calls when forge.cli is on\n" +
		"  glab       not found — carries GitLab calls when forge.cli is on\n" +
		"  taskwarrior not found — the Tasks pane and section are not offered without it\n\n" +
		"Store:         " + store + "\n\n"
}

// reportIn runs doctor in dir with only git on PATH and returns its report,
// and the report it should be, headed by reportHead and followed by tail.
func reportIn(t *testing.T, dir, tail string) (string, string) {
	t.Helper()

	pathWithOnlyGit(t)

	printed, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor")

	return printed.Stdout, reportHead(dir, fieldValue(printed.Stdout, "Store")) + tail
}

func TestDoctorReportsACompleteConfigurationWordForWord(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, completeConfig)

	// Act
	got, want := reportIn(t, dir, webhookJira(path)+"\nEverything required is set.\n\n"+notChecked)

	// Assert
	if got != want {
		t.Errorf("doctor =\n%s\nwant\n%s", got, want)
	}
}

func TestDoctorReportsWhatIsMissingAndWrongWordForWord(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"}, `+
		`"ui": {"keys": {"commit": "a"}}}`)

	// Act
	got, want := reportIn(t, dir, "Configuration: "+path+"\n"+
		"Tracker:       Jira at https://jira.example.com\n"+
		"Jira:          https://jira.example.com (bearer token)\n"+
		"Slack:         (not set) (none)\n\n"+
		"Missing:\n"+
		"  - messaging.client_id (workflow slack login) or messaging.webhook_url\n\n"+
		"Problems:\n"+
		"  - ui.keys binds two actions to one key in the same context: "+
		"stage-all and commit both bind \"a\" in the Commits pane\n\n"+
		"Edit "+path+", then run `workflow doctor` again.\n"+
		"`workflow --help` explains how to create each token.\n\n"+
		notChecked)

	// Assert
	if got != want {
		t.Errorf("doctor =\n%s\nwant\n%s", got, want)
	}
}

func TestDoctorReportsAFileOthersCanReachWordForWord(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, completeConfig)

	err := os.Chmod(path, 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Act
	got, want := reportIn(t, dir, webhookJira(path)+
		"Permissions:   0644, so other users can reach this file\n\n"+
		"It holds credentials. Make it yours alone with `chmod 600 "+path+"`.\n\n"+
		"Everything required is set.\n\n"+notChecked)

	// Assert
	if got != want {
		t.Errorf("doctor =\n%s\nwant\n%s", got, want)
	}
}

func TestDoctorReportsNoFileWordForWord(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	got, want := reportIn(t, dir, config.NoConfigHeadline+"\n\n"+config.InitStep+"\n"+config.DoctorStep+"\n"+
		"`workflow --help` explains how to create each token.\n")

	// Assert
	if got != want {
		t.Errorf("doctor =\n%s\nwant\n%s", got, want)
	}
}

func TestDoctorOnlineReportsTheCredentialsItCheckedWordForWord(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	// Act
	printed, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--online")

	// Assert
	want := "\nEverything required is set.\n\n" +
		"Credentials:\n" +
		"  jira       authenticates as Fred F. User (fred) (token from the configuration file)\n" +
		"  slack      an incoming webhook cannot be checked without posting with it\n" +
		"  forge      no repository remote, so there is no forge to ask\n"
	if !strings.HasSuffix(printed.Stdout, want) {
		t.Errorf("doctor --online =\n%s\nwant it to end\n%s", printed.Stdout, want)
	}
}
