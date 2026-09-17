// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/testshape"
)

// The test files the command-line tests write.
const (
	firstTest  = "a_test.go"
	secondTest = "b_test.go"
)

// unmarked is a test file whose one test, named name, has no markers; its
// function name is on line 5, column 6.
func unmarked(name string, helpers ...string) string {
	return header + "func " + name + "(t *testing.T) {\n\t_ = 1\n}\n" + strings.Join(helpers, "")
}

// write writes files into dir and returns each one's path, by name.
func write(t *testing.T, dir string, files map[string]string) map[string]string {
	t.Helper()

	paths := make(map[string]string, len(files))

	for name, contents := range files {
		path := filepath.Join(dir, name)

		err := os.WriteFile(path, []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}

		paths[name] = path
	}

	return paths
}

// run runs the command line and returns its exit code and output.
func run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer

	code := testshape.Run(args, &stdout, &stderr)

	return code, stdout.String(), stderr.String()
}

func TestRunPassesAPackageThatFollowsThePattern(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	paths := write(t, dir, map[string]string{
		firstTest: asserting("\texpect(t, x)\n", expectHelper),
		"a.go":    "package a\n",
	})

	err := os.Mkdir(filepath.Join(dir, "nested_test.go"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	code, stdout, stderr := run(paths[firstTest])

	// Assert
	if code != 0 || stdout != "testshape: every test marks its Arrange, Act and Assert.\n" || stderr != "" {
		t.Errorf("Run = %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunReportsViolationsInFileOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	paths := write(t, t.TempDir(), map[string]string{firstTest: unmarked("TestA"), secondTest: unmarked("TestB")})

	// Act
	code, stdout, stderr := run(paths[secondTest], paths[firstTest])

	// Assert
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if code != 1 || stdout != "" || len(lines) != 2 ||
		!strings.HasPrefix(lines[0], paths[firstTest]+":5:6: TestA: [missing-markers] ") ||
		!strings.HasPrefix(lines[1], paths[secondTest]+":5:6: TestB: [missing-markers] ") {
		t.Errorf("Run = %d, stdout %q, stderr:\n%s", code, stdout, stderr)
	}
}

func TestRunChecksTheWholePackageOfEachNamedFile(t *testing.T) {
	t.Parallel()

	// Arrange
	paths := write(t, t.TempDir(), map[string]string{
		firstTest:  asserting("\texpect(t, x)\n"),
		secondTest: unmarked("TestB", expectHelper),
	})

	// Act
	code, _, stderr := run(paths[firstTest], paths[firstTest], paths[secondTest])

	// Assert
	want := paths[secondTest] + ":5:6: TestB: [missing-markers] "
	if code != 1 || strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, want) {
		t.Errorf("Run = %d, stderr %q; want one violation, in b_test.go", code, stderr)
	}
}

func TestRunReportsADirectoryItCannotList(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	paths := write(t, dir, map[string]string{firstTest: unmarked("TestA")})

	err := os.Chmod(dir, 0o100)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	// Act
	code, stdout, stderr := run(paths[firstTest])

	// Assert
	if code != 2 || stdout != "" || !strings.Contains(stderr, "listing") {
		t.Errorf("Run = %d, stdout %q, stderr %q; want 2, saying it could not list the directory", code, stdout, stderr)
	}
}

func TestRunRefusesWhatItCannotCheck(t *testing.T) {
	t.Parallel()

	// Each case names the files to check, in a directory holding main.go and a
	// broken_test.go that does not parse.
	cases := map[string][]string{
		"no files":                        nil,
		"a file that is not a test":       {"main.go"},
		"a test file that does not exist": {"missing_test.go"},
		"a test file that does not parse": {"broken_test.go"},
	}

	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			write(t, dir, map[string]string{"main.go": "package main\n", "broken_test.go": "package broken_test\n\nfunc {\n"})

			args := make([]string, 0, len(files))
			for _, file := range files {
				args = append(args, filepath.Join(dir, file))
			}

			// Act
			code, stdout, stderr := run(args...)

			// Assert
			if code != 2 || stdout != "" || stderr == "" {
				t.Errorf("Run = %d, stdout %q, stderr %q; want 2 and a reason", code, stdout, stderr)
			}
		})
	}
}
