// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// preCommit is the hook the lefthook tests generate and run, prePush the one
// their configurations list beside it, and idleScript a hook script that does
// nothing.
const (
	preCommit  = "pre-commit"
	prePush    = "pre-push"
	idleScript = "#!/bin/sh\n"
)

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

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act: read the hooks git runs
	found, configured := seams.Existing(t.Context())

	// Assert: the one hook, and no configuration yet
	if configured || len(found) != 1 || found[0].Name != preCommit {
		t.Fatalf("Existing = %+v, %v; want the one hook and no configuration", found, configured)
	}

	// Act: write a configuration for it
	err := seams.Write(t.Context(), hooks.Structured(found))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert: lefthook is installed, and the configuration is seen
	installed, _ := os.ReadFile(filepath.Join(root, ".git", "hooks", preCommit))
	if !strings.Contains(string(installed), "lefthook") {
		t.Errorf("lefthook was not installed in the repository's hooks:\n%s", installed)
	}

	if _, nowConfigured := seams.Existing(t.Context()); !nowConfigured {
		t.Error("Existing does not see the configuration just written")
	}

	// Act: run pre-commit, with something staged — lefthook skips a pre-commit
	// job when nothing is, as a commit would
	write(t, filepath.Join(root, "staged.txt"), "x\n", 0o600)
	git(t, root, "add", "staged.txt")

	output, err := seams.Run(t.Context(), preCommit)
	if err != nil {
		t.Fatalf("Run did not start: %v", err)
	}

	lines, err := drained(output)

	// Assert: the hook ran through lefthook
	if err != nil || !strings.Contains(strings.Join(lines, "\n"), "checked") {
		t.Errorf("Run = %v, %q; want the hook to have run", err, lines)
	}
}

func TestTheHookSeamsListWhatLefthooksMergedConfigurationDefines(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  commands:\n    vet:\n      run: go vet ./...\n"+
		"commit-msg:\n  commands:\n    subject:\n      run: head -1 {1}\n"+
		"lint-all:\n  jobs:\n    - run: golangci-lint run\n", 0o600)
	write(t, filepath.Join(root, "lefthook-local.yml"), "pre-push:\n  jobs:\n    - run: go test ./...\n", 0o600)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Configured: %v", err)
	}

	if want := []string{preCommit, prePush, "lint-all"}; !slices.Equal(configured.Runnable, want) {
		t.Errorf("Runnable = %q, want %q, the local override's pre-push included", configured.Runnable, want)
	}

	if len(configured.GitOnly) != 1 || configured.GitOnly[0].Name != "commit-msg" {
		t.Errorf("GitOnly = %+v, want commit-msg alone", configured.GitOnly)
	}
}

func TestTheHookSeamsListLefthooksHooksWhileItLogsVerbosely(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)
	t.Setenv("LEFTHOOK_VERBOSE", "1")

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  jobs:\n    - run: go vet ./...\n", 0o600)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())

	// Assert
	if err != nil || !slices.Equal(configured.Runnable, []string{preCommit}) {
		t.Errorf("Configured = %+v, %v; want pre-commit, read past lefthook's debug log", configured, err)
	}
}

func TestTheHookSeamsListALocalOverrideThatStandsAlone(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook-local.yml"), "pre-push:\n  jobs:\n    - run: go test ./...\n", 0o600)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())

	// Assert
	if err != nil || !slices.Equal(configured.Runnable, []string{prePush}) {
		t.Errorf("Configured = %+v, %v; want the local override's pre-push", configured, err)
	}
}

func TestTheHookSeamsListNothingWhereLefthookFindsNoConfiguration(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	seams := wired(t, config.Default(), onOrigin(repository(t), ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())

	// Assert
	if err != nil || len(configured.Runnable) != 0 || len(configured.GitOnly) != 0 {
		t.Errorf("Configured = %+v, %v; want nothing listed and no failure", configured, err)
	}
}

func TestTheHookSeamsOfferNothingOutsideARepository(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	seams := wired(t, config.Default(), onOrigin(t.TempDir(), ""), nil).Hooks

	// Act
	found, configured := seams.Existing(t.Context())

	// Assert
	if len(found) != 0 || configured {
		t.Errorf("Existing outside a repository = %+v, %v; want nothing found and not configured", found, configured)
	}
}

func TestTheHookSeamsNeverOverwriteAConfiguration(t *testing.T) {
	// Arrange
	requireLefthook(t)
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "# mine\n", 0o600)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	err := seams.Write(t.Context(), hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: idleScript}}))

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
	seams := wired(t, config.Default(), onOrigin(t.TempDir(), ""), nil).Hooks

	// Assert
	if seams.Run != nil || seams.Existing != nil || seams.Write != nil || seams.Configured != nil {
		t.Error("hook seams are offered with no lefthook to answer them")
	}
}

// dumpingLefthook is a lefthook that answers only lefthook dump --format json,
// with a configuration of commit-msg and pre-push.
const dumpingLefthook = `#!/bin/sh
[ "$*" = "dump --format json" ] || exit 2
echo '{"commit-msg": {"commands": {"c": {"run": "x {1}"}}}, "pre-push": {"jobs": [{"run": "go test"}]}}'
`

func TestTheHookSeamsListWhatLefthookDumps(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "pre-push:\n", 0o600)
	t.Setenv("PATH", pathWithGitAnd(t, "lefthook", dumpingLefthook))

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Configured: %v", err)
	}

	if !slices.Equal(configured.Runnable, []string{prePush}) ||
		len(configured.GitOnly) != 1 || configured.GitOnly[0].Name != "commit-msg" {
		t.Errorf("Configured = %+v, want pre-push runnable and commit-msg git's alone", configured)
	}
}

func TestTheHookSeamsListNothingWhereNoLefthookIsConfigured(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	t.Setenv("PATH", pathWithGitAnd(t, "lefthook",
		"#!/bin/sh\necho \"Error: couldn't load config: No config files have been found\" >&2\nexit 1\n"))

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	configured, err := seams.Configured(t.Context())

	// Assert
	if err != nil || len(configured.Runnable) != 0 || len(configured.GitOnly) != 0 {
		t.Errorf("Configured = %+v, %v; want nothing listed, lefthook's refusal read as no configuration",
			configured, err)
	}
}

func TestAHookListLefthookRefusesSaysWhatLefthookSaid(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	write(t, filepath.Join(root, "lefthook.yml"), "pre-commit: [\n", 0o600)
	t.Setenv("PATH", pathWithGitAnd(t, "lefthook",
		"#!/bin/sh\necho \"couldn't load config: yaml: line 3\" >&2\nexit 1\n"))

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	_, err := seams.Configured(t.Context())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "couldn't load config: yaml: line 3") {
		t.Errorf("Configured = %v, want lefthook's own reason", err)
	}
}

func TestAFailedLefthookInstallSaysWhatLefthookSaid(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	bin := pathWithGitAnd(t, "lefthook", "#!/bin/sh\necho 'no hooks path for you' >&2\nexit 3\n")

	t.Setenv("PATH", bin)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	// Act
	err := seams.Write(t.Context(), hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: idleScript}}))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no hooks path for you") {
		t.Errorf("Write = %v, want lefthook's own reason", err)
	}
}

func TestALefthookGoneBeforeTheWriteSaysItCouldNotBeInstalled(t *testing.T) {
	// Arrange
	// lefthook was there when the seams were built, and is gone by the time
	// the configuration is written.
	isolateGit(t)

	root := repository(t)
	bin := pathWithGitAnd(t, "lefthook", "#!/bin/sh\nexit 0\n")

	t.Setenv("PATH", bin)

	seams := wired(t, config.Default(), onOrigin(root, ""), nil).Hooks

	err := os.Remove(filepath.Join(bin, "lefthook"))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = seams.Write(t.Context(), hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: idleScript}}))

	// Assert
	if !errors.Is(err, proc.ErrNotFound) || !strings.Contains(err.Error(), "installing lefthook") {
		t.Errorf("Write = %v, want lefthook reported missing as it was to be installed", err)
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
