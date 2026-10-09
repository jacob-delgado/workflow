// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package gittest keeps a package's tests on the repositories they make and
// off every other: it clears the variables a git hook exports, which would
// point every git a test starts at the hook's repository, runs git with the
// developer's own configuration kept out, and gives the test that proves a
// hook's environment changes nothing what it compares. Only tests import it.
package gittest

import (
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

// Run runs git in dir with args, the developer's global and system git
// configuration and language kept out, and returns what it wrote to standard
// output, trimmed. When git fails, so does tb, with git's own reason.
func Run(tb testing.TB, dir string, args ...string) string {
	tb.Helper()

	output, err := proc.RunCommand(tb.Context(), proc.Command{
		Name: "git", Args: append([]string{"-C", dir}, args...),
		Env: []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C"},
	})
	if err != nil {
		tb.Fatalf("git %v: %v", args, err)
	}

	return strings.TrimSpace(string(output))
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
