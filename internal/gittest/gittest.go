// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package gittest keeps a package's tests on the repositories they make and
// off every other: it clears the variables a git hook exports, which would
// point every git a test starts at the hook's repository; it has git read a
// configuration of the tests' own, the developer's kept out and git's
// automatic maintenance off, in the git it runs and, through a TestMain, in
// every git a process starts; and it gives the tests that prove a hook's
// environment changes nothing, and that no git outlives its test, what they
// compare. Only tests import it, and internal/cli/clitest, which only tests
// import in turn.
package gittest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// LocalVariables are the variables that tell git which repository to use, in
// git's own words for the purpose: what `git rev-parse --local-env-vars`
// lists. A hook exports them (githooks(5)), and from a linked worktree GIT_DIR
// is an absolute path into the shared repository.
func LocalVariables() []string {
	return []string{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
		"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX",
		"GIT_SHALLOW_FILE", "GIT_COMMON_DIR",
	}
}

// ClearLocalVariables unsets every one of LocalVariables, for a TestMain to
// call before any test runs: every git the tests start, theirs or the code's
// under test, would otherwise inherit them and work on the hook's repository
// instead of its own temporary one. Each is unset whatever became of the
// others, and any that could not be is reported.
func ClearLocalVariables() error {
	failures := make([]error, 0, len(LocalVariables()))
	for _, name := range LocalVariables() {
		failures = append(failures, os.Unsetenv(name))
	}

	return errors.Join(failures...)
}

// configuration is the global git configuration every git a test starts
// reads: git's automatic maintenance off. Since git 2.47 a commit, fetch,
// merge or rebase, and the receive-pack a push starts, return with `git
// maintenance run --auto` detached from them and still running, which can be
// writing into the repository as the test's TempDir is removed. That
// receive-pack, in a repository on disk, reads only its own repository's
// configuration and this one, never a -c the pushing git was handed; an older
// git's runs gc instead, which receive.autoGc stops too. It is the script
// tests' and the e2e fixture's scripts/lib/test-gitconfig as well, which Go
// cannot embed from outside this package, so a test holds the two to each
// other.
const configuration = "[maintenance]\n\tauto = false\n[receive]\n\tautoGc = false\n"

// configurationMode is the mode of the file configuration is written to.
const configurationMode = 0o600

// Configure writes into dir the global configuration every git a test
// starts is to read, then hands setenv each variable that points git at it
// and keeps the developer's own configuration and the system's out, reporting
// every one setenv could not set. No git that reads it leaves maintenance of
// its own running behind it. A TestMain hands it os.Setenv, so that every git
// its process starts, the tests' or the code's under test, reads it.
func Configure(dir string, setenv func(name, value string) error) error {
	path := filepath.Join(dir, ".gitconfig")

	err := os.WriteFile(path, []byte(configuration), configurationMode)
	if err != nil {
		return fmt.Errorf("writing the tests' git configuration: %w", err)
	}

	return errors.Join(setenv("GIT_CONFIG_GLOBAL", path), setenv("GIT_CONFIG_NOSYSTEM", "1"))
}

// Run runs git in dir with args, in one language and reading the
// configuration Configure writes into a directory of its own, and returns
// what it wrote to standard output, trimmed. When git fails, or its
// configuration cannot be written, so does tb, with the reason.
func Run(tb testing.TB, dir string, args ...string) string {
	tb.Helper()

	output, err := run(tb, dir, args)
	if err != nil {
		tb.Fatalf("git %v: %v", args, err)
	}

	return strings.TrimSpace(string(output))
}

// run is Run's git, its configuration written into a directory of tb's and
// the variables that point it there added to the environment git starts in.
func run(tb testing.TB, dir string, args []string) ([]byte, error) {
	tb.Helper()

	environment := []string{"LC_ALL=C"}

	err := Configure(tb.TempDir(), func(name, value string) error {
		environment = append(environment, name+"="+value)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return proc.RunCommand(tb.Context(), proc.Command{
		Name: "git", Args: append([]string{"-C", dir}, args...), Env: environment,
	})
}

// HookEnvironment is what git exports to a hook in a linked worktree of repo:
// the absolute path of its git directory, and of its index to a pre-commit.
func HookEnvironment(repo string) []string {
	gitDir := filepath.Join(repo, ".git")

	return []string{"GIT_DIR=" + gitDir, "GIT_INDEX_FILE=" + filepath.Join(gitDir, "index")}
}

// FilesUnder is every file below dir, keyed by its path within dir, holding
// its contents.
func FilesUnder(tb testing.TB, dir string) map[string]string {
	tb.Helper()

	files := map[string]string{}
	tree := os.DirFS(dir)

	err := fs.WalkDir(tree, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		contents, err := fs.ReadFile(tree, path)
		if err != nil {
			return fmt.Errorf("reading what the tests left: %w", err)
		}

		files[path] = string(contents)

		return nil
	})
	if err != nil {
		tb.Fatalf("reading %s: %v", dir, err)
	}

	return files
}

// ChangedFiles are the paths added, removed or rewritten between two
// readings of FilesUnder, sorted.
func ChangedFiles(before, after map[string]string) []string {
	var changed []string

	for path, contents := range after {
		if was, found := before[path]; !found || was != contents {
			changed = append(changed, path)
		}
	}

	for path := range before {
		if _, found := after[path]; !found {
			changed = append(changed, path)
		}
	}

	slices.Sort(changed)

	return changed
}

// MaintenanceStarted is every automatic maintenance or gc started by the gits
// that traced their events into dir (GIT_TRACE2_EVENT naming a directory),
// each as its command line, sorted.
func MaintenanceStarted(tb testing.TB, dir string) []string {
	tb.Helper()

	var started []string

	for path, events := range FilesUnder(tb, dir) {
		for line := range strings.Lines(events) {
			var event traceEvent

			err := json.Unmarshal([]byte(line), &event)
			if err != nil {
				tb.Fatalf("reading git's trace %s: %v", path, err)

				return nil
			}

			if event.startsMaintenance() {
				started = append(started, strings.Join(event.Argv, " "))
			}
		}
	}

	slices.Sort(started)

	return started
}

// traceEvent is what MaintenanceStarted reads of one trace2 event.
type traceEvent struct {
	Event string   `json:"event"`
	Argv  []string `json:"argv"`
}

// startsMaintenance reports a child git started to maintain a repository: a
// `git maintenance` or a `git gc`.
func (e traceEvent) startsMaintenance() bool {
	return e.Event == "child_start" && len(e.Argv) > 1 && (e.Argv[1] == "maintenance" || e.Argv[1] == "gc")
}
