// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// preCommit is the hook the lefthook tests generate and run.
const preCommit = "pre-commit"

// requireLefthook skips a test with nothing to exercise where lefthook is not
// installed: the hook seams are then deliberately empty.
func requireLefthook(t *testing.T) {
	t.Helper()

	if !proc.Available("lefthook") {
		t.Skip("lefthook is not installed, so its seams are deliberately empty")
	}
}

func TestTheHookSeamsReadWriteAndRunLefthook(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, ".git", "hooks", preCommit), "#!/bin/sh\necho checked\n", 0o700)

	seams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: ""}, nil).Hooks

	// Act: read the hooks git runs
	found, configured := seams.Existing()

	// Assert: the one hook, and no configuration yet
	if configured || len(found) != 1 || found[0].Name != preCommit {
		t.Fatalf("Existing = %+v, %v; want the one hook and no configuration", found, configured)
	}

	// Act: write a configuration for it
	err := seams.Write(hooks.Structured(found))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert: lefthook is installed, and the configuration is seen
	installed, _ := os.ReadFile(filepath.Join(root, ".git", "hooks", preCommit))
	if !strings.Contains(string(installed), "lefthook") {
		t.Errorf("lefthook was not installed in the repository's hooks:\n%s", installed)
	}

	if _, nowConfigured := seams.Existing(); !nowConfigured {
		t.Error("Existing does not see the configuration just written")
	}

	// Act: run pre-commit, with something staged — lefthook skips a pre-commit
	// job when nothing is, as a commit would
	write(t, filepath.Join(root, "staged.txt"), "x\n", 0o600)
	git(t, root, "add", "staged.txt")

	output, err := seams.Run(preCommit)
	if err != nil {
		t.Fatalf("Run did not start: %v", err)
	}

	lines, err := drained(output)

	// Assert: the hook ran through lefthook
	if err != nil || !strings.Contains(strings.Join(lines, "\n"), "checked") {
		t.Errorf("Run = %v, %q; want the hook to have run", err, lines)
	}
}

func TestTheHookSeamsOfferNothingOutsideARepository(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	seams := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Hooks

	// Act
	found, configured := seams.Existing()

	// Assert
	if len(found) != 0 || !configured {
		t.Errorf("Existing outside a repository = %+v, %v; want nothing offered", found, configured)
	}
}

func TestTheHookSeamsNeverOverwriteAConfiguration(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "# mine\n", 0o600)

	seams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: ""}, nil).Hooks

	// Act
	err := seams.Write(hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\n"}}))

	// Assert
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("Write = %v, want fs.ErrExist for the lefthook.yml already there", err)
	}

	if kept, _ := os.ReadFile(filepath.Join(root, "lefthook.yml")); string(kept) != "# mine\n" {
		t.Errorf("lefthook.yml is now %q", kept)
	}
}

func TestWithoutLefthookNoHookActionIsOffered(t *testing.T) {
	// Arrange
	t.Setenv("PATH", t.TempDir())

	// Act
	seams := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Hooks

	// Assert
	if seams.Run != nil || seams.Existing != nil || seams.Write != nil {
		t.Error("hook seams are offered with no lefthook to answer them")
	}
}

func TestAFailedLefthookInstallSaysWhatLefthookSaid(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	bin := pathWithGitAnd(t, "lefthook", "#!/bin/sh\necho 'no hooks path for you' >&2\nexit 3\n")

	t.Setenv("PATH", bin)

	seams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: ""}, nil).Hooks

	// Act
	err := seams.Write(hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\n"}}))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no hooks path for you") {
		t.Errorf("Write = %v, want lefthook's own reason", err)
	}
}

// pathWithGitAnd is a directory holding git and one fake program, for PATH.
func pathWithGitAnd(t *testing.T, name, script string) string {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("these tests need git: %v", err)
	}

	dir := t.TempDir()

	err = os.Symlink(gitPath, filepath.Join(dir, "git"))
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(dir, name), script, 0o700)

	return dir
}
