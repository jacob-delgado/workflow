// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

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
	gittest.Run(t, dir, args...)
}

func TestDoctorReportsTheRepositoryItIsIn(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	if got := fieldValue(output, "Repository"); !strings.Contains(got, "not in a git work tree") {
		t.Errorf("Repository = %q, want it to say the directory is outside a repository:\n%s", got, output)
	}
}

func TestDoctorReportsADetachedHead(t *testing.T) {
	t.Parallel()

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

// repoWithRemote makes dir a repository whose origin is remote.
func repoWithRemote(t *testing.T, remote string) string {
	t.Helper()

	dir := t.TempDir()
	gitInit(t, dir)
	git(t, dir, "remote", "add", "origin", remote)

	return dir
}

func TestDoctorNamesTheForgeAndItsAPI(t *testing.T) {
	t.Parallel()

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
			remote: onPremisesRemote,
			want:   "acme/thing on git.example.com (cannot tell GitHub Enterprise from self-managed GitLab)",
		},
		// A remote on the local disk has no host, so there is no forge to name.
		"a remote that is a local path": {
			remote: "/srv/git/thing.git",
			want:   "(the remote does not name a repository)",
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
			t.Parallel()

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
	t.Parallel()

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
