// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gittest_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

func TestLocalVariablesAreTheOnesGitNames(t *testing.T) {
	t.Parallel()

	// Act
	named := strings.Fields(gittest.Run(t, t.TempDir(), "rev-parse", "--local-env-vars"))

	// Assert
	// A git that adds one fails here, rather than leaving it set for a test
	// to inherit from a hook.
	listed := gittest.LocalVariables()
	if !slices.Equal(slices.Sorted(slices.Values(listed)), slices.Sorted(slices.Values(named))) {
		t.Errorf("LocalVariables = %q, want git's own list %q", listed, named)
	}
}

//nolint:paralleltest // it sets and clears the process's own environment, so it runs alone.
func TestClearLocalVariablesUnsetsEachOne(t *testing.T) {
	// Arrange
	for _, name := range gittest.LocalVariables() {
		t.Setenv(name, "/somewhere/else")
	}

	// Act
	err := gittest.ClearLocalVariables()
	// Assert
	if err != nil {
		t.Fatalf("ClearLocalVariables = %v", err)
	}

	for _, name := range gittest.LocalVariables() {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set, to %q", name, value)
		}
	}
}

func TestRunKeepsTheDevelopersConfigurationOut(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Someone Else\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	name := gittest.Run(t, t.TempDir(), "config", "--default", "nobody", "user.name")

	// Assert
	if name != "nobody" {
		t.Errorf("git read user.name %q, want the developer's own configuration kept out", name)
	}
}

func TestHookEnvironmentNamesTheGitDirectoryAndIndexOfTheRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := filepath.Join("repos", "api")

	// Act
	environment := gittest.HookEnvironment(repo)

	// Assert
	want := []string{
		"GIT_DIR=" + filepath.Join(repo, ".git"), "GIT_INDEX_FILE=" + filepath.Join(repo, ".git", "index"),
	}
	if !slices.Equal(environment, want) {
		t.Errorf("HookEnvironment = %q, want %q", environment, want)
	}
}

func TestChangedFilesNamesWhatWasAddedRemovedOrRewritten(t *testing.T) {
	t.Parallel()

	// Arrange
	const rewritten = "rewritten"

	dir := t.TempDir()

	for name, contents := range map[string]string{"kept": "same", rewritten: "before", "removed": "gone"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	before := gittest.FilesUnder(t, dir)

	for name, contents := range map[string]string{rewritten: "after", "added": "new"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := os.Remove(filepath.Join(dir, "removed"))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	changed := gittest.ChangedFiles(before, gittest.FilesUnder(t, dir))

	// Assert
	if want := []string{"added", "removed", rewritten}; !slices.Equal(changed, want) {
		t.Errorf("ChangedFiles = %q, want %q", changed, want)
	}
}

// failures is a test that keeps what it was told to fail with rather than
// failing, so a helper's own failure can be checked.
type failures struct {
	testing.TB

	said []string
}

func (f *failures) Fatalf(format string, args ...any) {
	f.said = append(f.said, fmt.Sprintf(format, args...))
}

func TestRunFailsTheTestWithGitsOwnReason(t *testing.T) {
	t.Parallel()

	// Arrange
	test := &failures{TB: t}

	// Act
	gittest.Run(test, t.TempDir(), "no-such-command")

	// Assert
	if len(test.said) != 1 || !strings.Contains(test.said[0], "no-such-command") ||
		!strings.Contains(test.said[0], "is not a git command") {
		t.Errorf("Run failed the test with %q, want once, naming the command and git's reason", test.said)
	}
}

func TestFilesUnderFailsTheTestForADirectoryThatIsNotThere(t *testing.T) {
	t.Parallel()

	// Arrange
	test := &failures{TB: t}
	dir := filepath.Join(t.TempDir(), "missing")

	// Act
	files := gittest.FilesUnder(test, dir)

	// Assert
	if len(test.said) != 1 || !strings.Contains(test.said[0], "reading "+dir) || len(files) != 0 {
		t.Errorf("FilesUnder = %q, failing the test with %q; want nothing and the directory named", files, test.said)
	}
}

func TestFilesUnderFailsTheTestForAFileItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("a file's mode does not keep this user from reading it here")
	}

	test := &failures{TB: t}
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "sealed"), []byte("left behind"), 0o000)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	gittest.FilesUnder(test, dir)

	// Assert
	if len(test.said) != 1 || !strings.Contains(test.said[0], "reading what the tests left") {
		t.Errorf("FilesUnder failed the test with %q, want once, for the file it could not read", test.said)
	}
}

func TestConfigureHandsSetenvTheVariablesThatPointGitAtTheTestsConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	set := map[string]string{}

	// Act
	err := gittest.Configure(dir, func(name, value string) error {
		set[name] = value

		return nil
	})
	// Assert
	if err != nil {
		t.Fatalf("Configure = %v", err)
	}

	want := map[string]string{"GIT_CONFIG_GLOBAL": filepath.Join(dir, ".gitconfig"), "GIT_CONFIG_NOSYSTEM": "1"}
	if !maps.Equal(set, want) {
		t.Errorf("Configure set %q, want %q", set, want)
	}
}

// errRefused is what a setenv that sets nothing fails with.
var errRefused = errors.New("refused")

func TestConfigureReportsEachVariableSetenvCouldNotSet(t *testing.T) {
	t.Parallel()

	// Arrange
	var tried []string

	// Act
	err := gittest.Configure(t.TempDir(), func(name, _ string) error {
		tried = append(tried, name)

		return fmt.Errorf("%s: %w", name, errRefused)
	})

	// Assert
	if !errors.Is(err, errRefused) || !slices.Equal(tried, []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM"}) ||
		!strings.Contains(err.Error(), "GIT_CONFIG_GLOBAL") || !strings.Contains(err.Error(), "GIT_CONFIG_NOSYSTEM") {
		t.Errorf("Configure = %v, having tried %q; want each variable tried and each refusal reported", err, tried)
	}
}

func TestConfigureSetsNothingWhereItCannotWriteTheConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	var set []string

	missing := filepath.Join(t.TempDir(), "missing")

	// Act
	err := gittest.Configure(missing, func(name, _ string) error {
		set = append(set, name)

		return nil
	})

	// Assert
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "writing the tests' git configuration") ||
		len(set) != 0 {
		t.Errorf("Configure = %v, having set %q; want the write's failure and nothing set", err, set)
	}
}

// commitAsTester is the arguments of an empty commit made as a fixed identity.
func commitAsTester() []string {
	return []string{
		"-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--quiet", "--allow-empty", "-m", "work",
	}
}

func TestRunLeavesNoMaintenanceRunningAfterACommit(t *testing.T) {
	// Arrange
	// Since git 2.47 a commit returns with maintenance it detached from itself
	// still running, which can be writing into the repository as the test's
	// TempDir is removed.
	repo := t.TempDir()
	gittest.Run(t, repo, "init", "--quiet")

	trace := t.TempDir()
	t.Setenv("GIT_TRACE2_EVENT", trace)

	// Act
	gittest.Run(t, repo, commitAsTester()...)

	// Assert
	if len(gittest.FilesUnder(t, trace)) == 0 {
		t.Fatal("git traced nothing of the commit")
	}

	if started := gittest.MaintenanceStarted(t, trace); len(started) != 0 {
		t.Errorf("the commit started %q, want no maintenance to outlive it", started)
	}
}

func TestRunLeavesNoMaintenanceRunningInTheRepositoryItPushesTo(t *testing.T) {
	// Arrange
	// The receive-pack a push starts in a repository on disk reads only that
	// repository's configuration and the global one, never what the pushing
	// git was handed with -c.
	remote := t.TempDir()
	gittest.Run(t, remote, "init", "--bare", "--quiet")

	repo := t.TempDir()
	gittest.Run(t, repo, "init", "--quiet")
	gittest.Run(t, repo, commitAsTester()...)

	trace := t.TempDir()
	t.Setenv("GIT_TRACE2_EVENT", trace)

	// Act
	gittest.Run(t, repo, "push", "--quiet", remote, "HEAD:refs/heads/main")

	// Assert
	if len(gittest.FilesUnder(t, trace)) == 0 {
		t.Fatal("git traced nothing of the push")
	}

	if started := gittest.MaintenanceStarted(t, trace); len(started) != 0 {
		t.Errorf("the push started %q, want no maintenance to outlive it", started)
	}
}

// homeless is a test whose TempDir is a directory that is not there, keeping
// what it was told to fail with, so a helper's failure to write into a
// directory of its own can be checked.
type homeless struct {
	failures

	missing string
}

func (h *homeless) TempDir() string {
	return h.missing
}

func TestRunFailsTheTestWhenItCannotWriteGitsConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	test := &homeless{TB: t, missing: filepath.Join(t.TempDir(), "missing")}

	// Act
	gittest.Run(test, t.TempDir(), "version")

	// Assert
	if len(test.said) != 1 || !strings.Contains(test.said[0], "writing the tests' git configuration") {
		t.Errorf("Run failed the test with %q, want once, for the configuration it could not write", test.said)
	}
}

func TestMaintenanceStartedNamesEachMaintenanceAndGcAChildWasStartedFor(t *testing.T) {
	t.Parallel()

	// Arrange
	trace := t.TempDir()
	events := `{"event":"start","argv":["git","commit"]}` + "\n" +
		`{"event":"child_start","argv":["git","maintenance","run","--auto","--detach"]}` + "\n" +
		`{"event":"child_start","argv":["git-receive-pack '/remote'"]}` + "\n" +
		`{"event":"child_start","argv":["git","pack-objects","--stdout"]}` + "\n" +
		`{"event":"child_start","argv":["git","gc","--auto","--quiet"]}` + "\n"

	err := os.WriteFile(filepath.Join(trace, "commit"), []byte(events), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	started := gittest.MaintenanceStarted(t, trace)

	// Assert
	want := []string{"git gc --auto --quiet", "git maintenance run --auto --detach"}
	if !slices.Equal(started, want) {
		t.Errorf("MaintenanceStarted = %q, want %q", started, want)
	}
}

func TestMaintenanceStartedFailsTheTestForATraceThatIsNotGits(t *testing.T) {
	t.Parallel()

	// Arrange
	test := &failures{TB: t}
	trace := t.TempDir()

	err := os.WriteFile(filepath.Join(trace, "other"), []byte("not an event\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	started := gittest.MaintenanceStarted(test, trace)

	// Assert
	if len(test.said) != 1 || !strings.Contains(test.said[0], "reading git's trace other") || len(started) != 0 {
		t.Errorf("MaintenanceStarted = %q, failing the test with %q; want nothing and the trace named", started, test.said)
	}
}

func TestConfigurationIsTheOneTheScriptTestsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	want, err := os.ReadFile(filepath.Join("..", "..", "scripts", "lib", "test-gitconfig"))
	if err != nil {
		t.Fatalf("reading the script tests' git configuration: %v", err)
	}

	// Act
	err = gittest.Configure(dir, func(string, string) error { return nil })
	// Assert
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".gitconfig"))
	if err != nil {
		t.Fatalf("reading what Configure wrote: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("gittest's configuration differs from scripts/lib/test-gitconfig:\n got %q\nwant %q", got, want)
	}
}
