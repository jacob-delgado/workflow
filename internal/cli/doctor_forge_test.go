// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubSSHRemote is a GitHub origin written as an SSH remote, so the forge
// resolves to github.com without a token in the URL.
const githubSSHRemote = "git@github.com:owner/repo.git"

// onPremisesRemote is an origin on a host whose name says neither GitHub nor
// GitLab, so only forge.kind can say which forge it is.
const onPremisesRemote = "git@git.example.com:acme/thing.git"

// ghSignedOut puts git and a gh on PATH where gh exits non-zero for every
// invocation, standing in for a gh that is installed but signed out of every
// host: `gh auth token` then yields no credential, though gh itself is found.
func ghSignedOut(t *testing.T) {
	t.Helper()

	ghSignedOutRecording(t)
}

// ghSignedOutRecording is ghSignedOut with a gh that also creates a file each
// time it runs, and returns that file's path, for a test that asserts gh was
// never reached.
func ghSignedOutRecording(t *testing.T) string {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("these tests need git: %v", err)
	}

	dir := t.TempDir()

	err = os.Symlink(gitPath, filepath.Join(dir, "git"))
	if err != nil {
		t.Fatalf("linking git: %v", err)
	}

	ran := filepath.Join(dir, "gh-ran")
	script := "#!/bin/sh\n: >'" + ran + "'\nexit 1\n"

	err = os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755)
	if err != nil {
		t.Fatalf("writing the fake gh: %v", err)
	}

	t.Setenv("PATH", dir)

	return ran
}

func TestDoctorOnlineExplainsWhyNoForgeTokenResolved(t *testing.T) {
	cases := map[string]struct {
		remote string
		onPath func(*testing.T)
		want   string
	}{
		"gh installed but signed out names the host": {
			remote: githubSSHRemote,
			onPath: ghSignedOut,
			want:   "gh is installed but not signed in to github.com — run `gh auth login`",
		},
		"gh absent lists the sources instead": {
			remote: githubSSHRemote,
			onPath: pathWithOnlyGit,
			want:   "none — " + forge.Sources(forge.KindGitHub, "github.com"),
		},
		"a GitLab remote has no gh hint": {
			remote: "git@gitlab.com:group/proj.git",
			onPath: pathWithOnlyGit,
			want:   "none — " + forge.Sources(forge.KindGitLab, "gitlab.com"),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			// Every forge variable is cleared, so a laptop's or CI's own token
			// cannot resolve; the tracker answers, so only the forge check fails.
			clearForgeEnvironment(t)
			dir := repoWithRemote(t, tt.remote)
			writeConfigFor(t, dir, workingJira(t))
			tt.onPath(t)

			// Act
			output, err := run(t, dir, "doctor", "--online")

			// Assert
			if err == nil {
				t.Fatalf("doctor accepted a missing forge token:\n%s", output)
			}

			if !strings.Contains(output, tt.want) {
				t.Errorf("doctor does not explain the missing token: want %q\n%s", tt.want, output)
			}
		})
	}
}

func TestDoctorNamesAMissingGitOnTheRepositoryLine(t *testing.T) {
	// Arrange
	// An empty PATH is the portable way to make git unfindable.
	t.Setenv("PATH", "")

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if got := fieldValue(output, "Repository"); !strings.Contains(got, "git is not on PATH") {
		t.Errorf("Repository = %q, want it to say git is missing:\n%s", got, output)
	}
}

// forgeCLIRepository is a repository whose origin is remote, configured to reach
// the forge through its CLI with no forge token anywhere: none in the
// environment, where a laptop's or CI's own would otherwise resolve, and none in
// the file.
func forgeCLIRepository(t *testing.T, remote string) string {
	t.Helper()

	clearForgeEnvironment(t)
	dir := repoWithRemote(t, remote)
	writeFile(t, dir, `{"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}, `+slackWebhook+`,`+
		` "forge": {"cli": true}}`)

	return dir
}

// ghSignedOutButAnswering puts a gh on PATH whose `gh auth token` yields nothing
// while its `api` command answers, so a forge check can pass only by asking
// through gh itself.
func ghSignedOutButAnswering(t *testing.T) {
	t.Helper()

	fakeGh(t, ghResponses{signedOut: true})
}

func TestDoctorOnlineAsksTheForgeThroughItsCLI(t *testing.T) {
	cases := map[string]struct {
		remote string
		onPath func(*testing.T)
		want   string
	}{
		"a GitHub remote asks through gh": {
			remote: githubSSHRemote,
			onPath: ghSignedOutButAnswering,
			want:   "authenticates as octo (through gh)",
		},
		"a GitLab remote asks through glab": {
			remote: "git@gitlab.com:group/proj.git",
			onPath: fakeGlab,
			want:   "authenticates as tanuki (through glab)",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := forgeCLIRepository(t, tt.remote)
			tt.onPath(t)

			// Act
			output, err := run(t, dir, "doctor", "--online")
			// Assert
			if err != nil {
				t.Fatalf("doctor --online = %v, want a forge its CLI answers for to pass:\n%s", err, output)
			}

			if !strings.Contains(output, tt.want) {
				t.Errorf("doctor does not name the CLI it asked through: want %q\n%s", tt.want, output)
			}
		})
	}
}

func TestDoctorJSONOnlineCallsAForgeItsCLIAnswersForOK(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, githubSSHRemote)
	ghSignedOutButAnswering(t)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 0)

	if got := credentialStatusIn(decodeReport(t, output), "forge"); got != "ok" {
		t.Errorf("the online report calls a forge gh answered for %q, want ok:\n%s", got, output)
	}
}

// A signed-out CLI cannot reach the forge for doctor any more than for the
// commands, which exit 5 over it; no token to resolve is not what is wrong.
func TestDoctorJSONOnlineCallsAForgeItsSignedOutCLIUnreachable(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, githubSSHRemote)
	ghSignedOut(t)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 5)

	if got := credentialStatusIn(decodeReport(t, output), "forge"); got != unreachableStatus {
		t.Errorf("the online report calls a forge a signed-out gh could not reach %q, want unreachable:\n%s",
			got, output)
	}

	if !strings.Contains(output, "through gh") {
		t.Errorf("the forge's detail does not say it was asked through gh:\n%s", output)
	}
}

// onlineRequestTimeout is the request timeout a test sets when it waits for
// doctor to give up on a check.
const onlineRequestTimeout = "500ms"

func TestDoctorOnlineGivesUpOnAForgeCLIThatNeverAnswers(t *testing.T) {
	// Arrange
	clearForgeEnvironment(t)
	dir := repoWithRemote(t, githubSSHRemote)
	writeFile(t, dir, `{"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}, `+slackWebhook+`,`+
		` "forge": {"cli": true}, "timing": {"request_timeout": "`+onlineRequestTimeout+`"}}`)
	fakeGh(t, ghResponses{signedOut: true, apiHangs: true})

	started := time.Now()

	// Act
	output, err := run(t, dir, "doctor", "--online")

	// Assert
	if elapsed := time.Since(started); elapsed >= ghHang/2 {
		t.Fatalf("doctor --online took %s over a gh that never answers, want it to give up after %s:\n%s",
			elapsed, onlineRequestTimeout, output)
	}

	wantExit(t, err, 5)

	if want := "gave up after " + onlineRequestTimeout + " (through gh)"; !strings.Contains(output, want) {
		t.Errorf("doctor does not say it gave up on the forge: want %q\n%s", want, output)
	}
}

func TestLogReachesDoctorOnlinesForgeCheckThroughItsCLI(t *testing.T) {
	// Arrange
	dir := forgeCLIRepository(t, githubSSHRemote)
	ghSignedOutButAnswering(t)
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := run(t, dir, "doctor", "--online", "--log", logPath)
	// Assert
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	if logged := readLog(t, logPath); !strings.Contains(logged, "forge GET  /user") {
		t.Errorf("the request log does not outline doctor's check of the forge:\n%s", logged)
	}
}

// workTree is a directory for doctor to run in: a repository whose origin is
// remote, or a plain directory outside any repository when remote is empty.
func workTree(t *testing.T, remote string) string {
	t.Helper()

	if remote == "" {
		return t.TempDir()
	}

	return repoWithRemote(t, remote)
}

// jiraTracker is a configuration with Jira as the tracker at baseURL, and a
// webhook doctor never posts to.
func jiraTracker(baseURL string) string {
	return `{"jira": {"base_url": "` + baseURL + `", "token": "t"}, ` + slackWebhook + `}`
}

func TestDoctorNamesTheTrackerInEffect(t *testing.T) {
	cases := map[string]struct {
		remote string
		config string
		want   string
	}{
		"no jira.base_url and a GitHub remote names GitHub's issues": {
			remote: githubSSHRemote,
			config: `{` + slackWebhook + `}`,
			want:   "GitHub issues (no jira.base_url)",
		},
		"forge.kind names an on-premises forge's issues": {
			remote: onPremisesRemote,
			config: `{` + slackWebhook + `, "forge": {"kind": "gitlab", "host": "git.example.com"}}`,
			want:   "GitLab issues (no jira.base_url)",
		},
		"an on-premises host without forge.kind leaves the forge unnamed": {
			remote: onPremisesRemote,
			config: `{` + slackWebhook + `}`,
			want:   "the forge's issues (no jira.base_url)",
		},
		"no remote leaves the forge unnamed": {
			remote: "",
			config: `{` + slackWebhook + `}`,
			want:   "the forge's issues (no jira.base_url)",
		},
		"a jira.base_url makes Jira the tracker": {
			remote: githubSSHRemote,
			config: jiraTracker("https://jira.example.com"),
			want:   "Jira at https://jira.example.com",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := workTree(t, tt.remote)
			writeFile(t, dir, tt.config)

			// Act
			output, err := run(t, dir, "doctor")

			// Assert
			wantExit(t, err, 0)

			if got := fieldValue(output, "Tracker"); got != tt.want {
				t.Errorf("Tracker = %q, want %q:\n%s", got, tt.want, output)
			}
		})
	}
}

// A base URL can carry a password, and this output is what the bug report
// template invites people to paste. The exit is left unasserted: it answers to
// how the configuration reads a password there, not to the masking.
func TestDoctorMasksAJiraBaseURLsPassword(t *testing.T) {
	// Arrange
	dir := workTree(t, githubSSHRemote)
	writeFile(t, dir, jiraTracker("https://fred:hunter2@jira.example.com"))

	// Act
	output, _ := run(t, dir, "doctor")

	// Assert
	if got := fieldValue(output, "Tracker"); got != "Jira at https://xxxxx@jira.example.com" {
		t.Errorf("Tracker = %q, want the password masked:\n%s", got, output)
	}

	if strings.Contains(output, "hunter2") {
		t.Errorf("doctor printed jira.base_url's password:\n%s", output)
	}
}

func TestDoctorJSONNamesTheTrackerInEffect(t *testing.T) {
	cases := map[string]struct {
		config string
		want   string
	}{
		"a jira.base_url makes Jira the tracker": {
			config: jiraTracker("https://jira.example.com"),
			want:   "jira",
		},
		"no jira.base_url leaves the forge's issues": {
			config: `{` + slackWebhook + `}`,
			want:   "forge",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := repoWithRemote(t, githubSSHRemote)
			writeFile(t, dir, tt.config)

			// Act
			output, err := run(t, dir, "doctor", "--json")

			// Assert
			wantExit(t, err, 0)

			configuration, _ := decodeReport(t, output)["configuration"].(map[string]any)
			if got := configuration["tracker"]; got != tt.want {
				t.Errorf("configuration.tracker = %v, want %q:\n%s", got, tt.want, output)
			}
		})
	}
}

// forgeTrackerRepository is a GitHub repository with no Jira configured, so
// the forge's issues are the tracker, and whose forge answers through a
// signed-out gh, so the forge check passes.
func forgeTrackerRepository(t *testing.T) string {
	t.Helper()

	clearForgeEnvironment(t)
	dir := repoWithRemote(t, githubSSHRemote)
	writeFile(t, dir, `{`+slackWebhook+`, "forge": {"cli": true}}`)
	ghSignedOutButAnswering(t)

	return dir
}

func TestDoctorOnlineSaysTheForgeStandsInForJira(t *testing.T) {
	// Arrange
	dir := forgeTrackerRepository(t)

	// Act
	output, err := run(t, dir, "doctor", "--online")

	// Assert
	wantExit(t, err, 0)

	if !strings.Contains(output, "jira       not configured — the forge's issues are the tracker") {
		t.Errorf("doctor --online does not say why Jira went unchecked:\n%s", output)
	}
}

func TestDoctorJSONOnlineCallsJiraUncheckedWhenTheForgeIsTheTracker(t *testing.T) {
	// Arrange
	dir := forgeTrackerRepository(t)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 0)

	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != uncheckedStatus {
		t.Errorf("the online report calls Jira %q with no jira.base_url, want unchecked:\n%s", got, output)
	}
}
