// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubSSHRemote is a GitHub origin written as an SSH remote, so the forge
// resolves to github.com without a token in the URL.
const githubSSHRemote = "git@github.com:owner/repo.git"

// ghSignedOut puts git and a gh on PATH where gh exits non-zero for every
// invocation, standing in for a gh that is installed but signed out of every
// host: `gh auth token` then yields no credential, though gh itself is found.
func ghSignedOut(t *testing.T) {
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

	err = os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	if err != nil {
		t.Fatalf("writing the fake gh: %v", err)
	}

	t.Setenv("PATH", dir)
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
