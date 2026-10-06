// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/pem"
	"fmt"
	"io/fs"
	"net/http/httptest"
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
// start, theirs or the command's under test, would inherit it and work on that
// repository instead of its temporary one. A pre-push once set core.bare, moved
// main and wrote a stranger's identity into it that way. The names are git's
// own list for the purpose, `git rev-parse --local-env-vars`. It also trusts
// the local servers the tests deliver to, for as long as they run, and takes
// every task program off PATH.
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

	roots := trustTestServers()
	shadows := withoutTaskPrograms()
	code := m.Run()

	for _, removed := range append(shadows, roots) {
		err := os.RemoveAll(removed)
		if err != nil {
			panic(err)
		}
	}

	os.Exit(code)
}

// withoutTaskPrograms takes every task program off PATH, and returns the
// directories it made to do so. doctor looks for Taskwarrior among them, and a
// test must never run the machine's own: go-task, or the user's Taskwarrior over
// their own tasks. A directory holding one is replaced by a directory of links to
// everything else in it, since git may live beside it.
func withoutTaskPrograms() []string {
	var (
		path    []string
		shadows []string
	)

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !holdsTaskProgram(dir) {
			path = append(path, dir)

			continue
		}

		absolute, err := filepath.Abs(dir)
		if err != nil {
			panic(err)
		}

		shadow := linksToAllButTask(absolute)
		path = append(path, shadow)
		shadows = append(shadows, shadow)
	}

	err := os.Setenv("PATH", strings.Join(path, string(os.PathListSeparator)))
	if err != nil {
		panic(err)
	}

	return shadows
}

// holdsTaskProgram reports a directory with a task or task.exe in it.
func holdsTaskProgram(dir string) bool {
	for _, name := range []string{"task", "task.exe"} {
		_, err := os.Stat(filepath.Join(dir, name))
		if err == nil {
			return true
		}
	}

	return false
}

// linksToAllButTask is a new directory linking to every entry of dir but its
// task program.
func linksToAllButTask(dir string) string {
	shadow, err := os.MkdirTemp("", "workflow-test-path-*")
	if err != nil {
		panic(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "task") || strings.EqualFold(entry.Name(), "task.exe") {
			continue
		}

		err = os.Symlink(filepath.Join(dir, entry.Name()), filepath.Join(shadow, entry.Name()))
		if err != nil {
			panic(err)
		}
	}

	return shadow
}

// trustTestServers makes the certificate every httptest TLS server presents
// the only one the process trusts, so a command can deliver to a local webhook
// over https, the one scheme the messaging client accepts. SSL_CERT_FILE names
// it, which macOS honors as Linux does since Go 1.27; the system reads its
// roots once, at the first connection, so this runs before any test. It
// returns the file it wrote, for removing once the tests are done.
func trustTestServers() string {
	server := httptest.NewTLSServer(nil)
	certificate := server.Certificate()
	server.Close()

	file, err := os.CreateTemp("", "workflow-test-roots-*.pem")
	if err != nil {
		panic(err)
	}

	err = pem.Encode(file, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err != nil {
		panic(err)
	}

	err = file.Close()
	if err != nil {
		panic(err)
	}

	err = os.Setenv("SSL_CERT_FILE", file.Name())
	if err != nil {
		panic(err)
	}

	return file.Name()
}

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	// Arrange
	// The fixtures that did the damage once: a bare push remote, a branch, an
	// identity written to a repository's configuration.
	fixtures := []string{"TestPRPushesThenOpensAnUnpublishedBranch", "TestSummaryReadsYourCommitsInThePeriod"}

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
