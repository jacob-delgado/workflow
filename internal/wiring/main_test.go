// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook exports them (githooks(5)), and from a linked worktree
// GIT_DIR is an absolute path into the shared repository: every git these tests
// start, theirs, lefthook's or a seam's, would inherit it and work on that
// repository instead of its temporary one — committing to it, and pushing to
// its origin. The names are git's own list for the purpose, `git rev-parse
// --local-env-vars`.
func TestMain(m *testing.M) {
	for _, name := range []string{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
		"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX",
		"GIT_SHALLOW_FILE", "GIT_COMMON_DIR",
	} {
		// Running the tests anyway would be running them against that repository.
		err := os.Unsetenv(name)
		if err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	// Arrange
	// A repository made, committed to and branched, and a push to origin: the
	// test's own repository has none, but the one a hook names does.
	fixtures := []string{
		"TestTheGitSeamsReportAPushGitRefuses",
		"TestTheGitSeamsCommitWhatIsStagedAndLeaveNoMessageBehind",
	}

	isolateGit(t)

	named := t.TempDir()
	git(t, named, "init", "--quiet")

	before := filesUnder(t, named)

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^("+strings.Join(fixtures, "|")+")$", "-test.v")

	child.Env = append(os.Environ(), hookEnvironment(named)...)

	// Act
	output, err := child.CombinedOutput()

	// Assert
	if changed := changedFiles(before, filesUnder(t, named)); len(changed) != 0 {
		t.Errorf("the tests wrote into the repository git's environment names: %q", changed)
	}

	if err != nil {
		t.Fatalf("the tests failed with git's environment naming another repository: %v\n%s", err, output)
	}

	for _, fixture := range fixtures {
		if !strings.Contains(string(output), "--- PASS: "+fixture+" ") {
			t.Errorf("%s did not pass with git's environment naming another repository:\n%s", fixture, output)
		}
	}
}

// hookEnvironment is what git exports to a hook in a linked worktree of repo:
// the absolute path of its git directory, and of its index to a pre-commit.
func hookEnvironment(repo string) []string {
	gitDir := filepath.Join(repo, ".git")

	return []string{"GIT_DIR=" + gitDir, "GIT_INDEX_FILE=" + filepath.Join(gitDir, "index")}
}

// filesUnder is every file below dir, keyed by its path within dir.
func filesUnder(t *testing.T, dir string) map[string]string {
	t.Helper()

	files := map[string]string{}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading what the tests left: %w", err)
		}

		files[strings.TrimPrefix(path, dir)] = string(contents)

		return nil
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	return files
}

// changedFiles are the paths added, removed or rewritten between two readings.
func changedFiles(before, after map[string]string) []string {
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
