// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// fieldValue is the value doctor printed for a label, as in "Forge:   GitHub…",
// or empty when it printed no such line. Reading one line, rather than the
// whole report, is what stops a check from passing on text some other section
// happens to print.
func fieldValue(output, label string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if value, found := strings.CutPrefix(line, label+":"); found {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func TestDoctorReportsAMissingFile(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "doctor")

	// Assert
	if err == nil || !strings.Contains(output, "config init") {
		t.Errorf("doctor = %v, want an error saying how to create a config:\n%s", err, output)
	}
}

func TestDoctorNamesMissingFields(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if err == nil {
		t.Fatalf("expected an error for an incomplete config, got none (%s)", output)
	}

	if !strings.Contains(output, "slack.token or slack.webhook_url") {
		t.Errorf("doctor does not offer both Slack transports:\n%s", output)
	}

	// With no Slack credential at all, also demanding slack.channel would read
	// as "set three things" when either transport is the actual next step.
	if strings.Contains(output, "- slack.channel") {
		t.Errorf("doctor asked for a channel before a credential:\n%s", output)
	}
}

func TestDoctorAcceptsACompleteConfig(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "slack": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if err != nil || !strings.Contains(output, "bearer token") {
		t.Errorf("doctor = %v, want success reporting the auth mode:\n%s", err, output)
	}
}

func TestDoctorFailsOnAConfigurationOthersCanRead(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "slack": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)

	err := os.Chmod(path, 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	// It says what is wrong and the one command that puts it right.
	if err == nil || !strings.Contains(output, "0644") || !strings.Contains(output, "chmod 600 "+path) {
		t.Errorf("doctor = %v, want it to refuse a file others can read and say how to fix it:\n%s", err, output)
	}
}

// gitInit makes dir a real repository, so doctor's repository section can be
// tested against git itself rather than against an imitation of it.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "--quiet", ".")
}

// git runs one git command in dir, failing the test if it does not succeed. The
// developer's own git configuration is kept out, as run keeps it out of doctor.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
}

func TestDoctorReportsTheRepositoryItIsIn(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)

	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	// The configuration is absent, so doctor exits non-zero. The repository
	// section must still be reported: someone runs doctor precisely when
	// something is wrong, and reporting only the first problem wastes the run.
	if err == nil {
		t.Errorf("doctor succeeded with no configuration:\n%s", output)
	}

	if got := fieldValue(output, "Repository"); got != root {
		t.Errorf("Repository = %q, want %q:\n%s", got, root, output)
	}

	// A freshly initialized repository has no commits, so its branch is unborn.
	// `git branch --show-current` still names it, which is why doctor can.
	if got := fieldValue(output, "Branch"); got == "" || strings.Contains(got, "detached HEAD") {
		t.Errorf("Branch = %q, want the unborn branch named:\n%s", got, output)
	}

	if got := fieldValue(output, "Remote"); got != "(not set)" {
		t.Errorf("Remote = %q, want the missing origin reported:\n%s", got, output)
	}
}

func TestDoctorReportsADirectoryOutsideAnyRepository(t *testing.T) {
	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if got := fieldValue(output, "Repository"); !strings.Contains(got, "not in a git work tree") {
		t.Errorf("Repository = %q, want it to say the directory is outside a repository:\n%s", got, output)
	}
}

func TestDoctorReportsTheExternalTooling(t *testing.T) {
	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if !strings.Contains(output, "Tooling:") {
		t.Fatalf("doctor has no tooling section:\n%s", output)
	}

	// git is required and is present wherever these tests can run at all, so it
	// must be reported as found rather than merely mentioned.
	if !strings.Contains(output, "git        found") {
		t.Errorf("doctor does not report git as found:\n%s", output)
	}

	for _, program := range []string{"lefthook", "gh"} {
		if !strings.Contains(output, program) {
			t.Errorf("doctor does not mention the optional program %q:\n%s", program, output)
		}
	}
}

func TestDoctorReportsADetachedHead(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)

	// A commit is needed before HEAD can be detached from anything.
	git(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "--allow-empty", "--quiet", "--message", "seed")
	git(t, dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "HEAD")

	// Act
	output, _ := run(t, dir, "doctor")

	// Assert
	if got := fieldValue(output, "Branch"); !strings.Contains(got, "detached HEAD") {
		t.Errorf("Branch = %q, want a detached HEAD reported:\n%s", got, output)
	}
}

func TestDoctorFailsWhenRequiredToolingIsMissing(t *testing.T) {
	// Arrange
	// An empty PATH is the only portable way to make every program unfindable,
	// which is what proves the required/optional distinction actually bites.
	t.Setenv("PATH", "")

	// Act
	output, err := run(t, t.TempDir(), "doctor")

	// Assert
	if err == nil {
		t.Fatalf("doctor succeeded with git missing:\n%s", output)
	}

	if !strings.Contains(output, "MISSING") {
		t.Errorf("doctor does not mark the missing required program:\n%s", output)
	}

	if !strings.Contains(err.Error(), "git") {
		t.Errorf("doctor error = %v, want it to name git", err)
	}

	// An optional program that is absent reads differently from a required one.
	if !strings.Contains(output, "not found") {
		t.Errorf("doctor does not distinguish an absent optional program:\n%s", output)
	}
}

func TestDoctorReportsAMalformedConfiguration(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "{not json")

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Fatalf("doctor = %v, want the malformed configuration reported:\n%s", err, output)
	}

	// A file that exists but cannot be parsed is a different problem from no
	// file at all, and telling someone to create one they already have is worse
	// than saying nothing.
	if strings.Contains(output, "config init") {
		t.Errorf("doctor told the user to create a file that already exists:\n%s", output)
	}
}

func TestDoctorAcceptsAWebhookWithoutAChannel(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/supersecretpayload"

	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "slack": {"webhook_url": "`+webhook+`"}}`)

	// Act
	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor rejected a webhook-only configuration: %v (%s)", err, output)
	}

	// Assert
	// The whole point of the leak test: doctor's output is what
	// .github/ISSUE_TEMPLATE/bug_report.yml invites people to paste.
	if strings.Contains(output, webhook) || strings.Contains(output, "supersecretpayload") {
		t.Errorf("doctor printed the webhook URL:\n%s", output)
	}

	if strings.Contains(output, "hooks.slack.com") {
		t.Errorf("doctor printed part of the webhook URL:\n%s", output)
	}

	if !strings.Contains(output, "incoming webhook") {
		t.Errorf("doctor does not name the Slack transport:\n%s", output)
	}
}

func TestDoctorDoesNotTouchTheNetworkWithoutTheFlag(t *testing.T) {
	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	writeConfigFor(t, dir, server.URL)

	// Act
	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v (%s)", err, output)
	}

	// Assert
	if reached.Load() {
		t.Errorf("doctor called Jira without --online:\n%s", output)
	}

	if !strings.Contains(output, "--online") {
		t.Errorf("doctor does not say how to check the credentials:\n%s", output)
	}
}

// repoWithRemote makes dir a repository whose origin is remote.
func repoWithRemote(t *testing.T, remote string) string {
	t.Helper()

	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "remote", "add", "origin", remote)

	return dir
}

func TestDoctorNamesTheForgeAndItsAPI(t *testing.T) {
	// githubForge is how doctor names github.com's owner/repo, however the
	// remote is written.
	const githubForge = "GitHub owner/repo at https://api.github.com"

	cases := map[string]struct {
		remote string
		want   string
	}{
		"github over ssh": {
			remote: "git@github.com:owner/repo.git",
			want:   githubForge,
		},
		"gitlab with a subgroup": {
			remote: "https://gitlab.com/group/sub/project.git",
			want:   "GitLab group/sub/project at https://gitlab.com/api/v4",
		},
		// Neither forge announces itself in an on-premises hostname, and their
		// API paths differ, so guessing would send a token to the wrong service.
		"an on-premises host": {
			remote: "git@git.example.com:acme/thing.git",
			want:   "acme/thing on git.example.com (cannot tell GitHub Enterprise from self-managed GitLab)",
		},
		// The remote can carry userinfo; the forge is still named, without it.
		"a remote with a password in it": {
			remote: "https://alice:sekret@github.com/owner/repo.git",
			want:   githubForge,
		},
		"a remote with only a username in it": {
			remote: "https://sekret@github.com/owner/repo.git",
			want:   githubForge,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := repoWithRemote(t, tt.remote)

			// Act
			output, _ := run(t, dir, "doctor")

			// Assert
			if got := fieldValue(output, "Forge"); got != tt.want {
				t.Errorf("Forge = %q, want %q:\n%s", got, tt.want, output)
			}

			if strings.Contains(output, "sekret") {
				t.Errorf("doctor printed the remote's userinfo:\n%s", output)
			}
		})
	}
}

func TestDoctorSaysWhenThereIsNoRemote(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)

	// Act
	output, _ := run(t, dir, "doctor")

	// Assert
	if got := fieldValue(output, "Forge"); got != "(no remote)" {
		t.Errorf("Forge = %q, want it to say the repository has no remote:\n%s", got, output)
	}
}

func TestDoctorReportsSetButInvalidValues(t *testing.T) {
	// Arrange
	// Nothing is missing, but three values are filled in wrong: a base URL that
	// is not http(s), an insecure webhook, and a forge kind that names no forge.
	dir := t.TempDir()
	writeFile(t, dir, `{"jira":{"base_url":"ftp://jira.example.com","token":"t"},`+
		`"slack":{"webhook_url":"http://hooks.example.com/x"},"forge":{"kind":"githb"}}`)

	// Act
	out, err := run(t, dir, "doctor")

	// Assert
	if err == nil {
		t.Errorf("doctor passed a configuration with invalid values:\n%s", out)
	}

	for _, want := range []string{"Problems", "jira.base_url", "slack.webhook_url", "forge.kind"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor did not flag %q:\n%s", want, out)
		}
	}
}
