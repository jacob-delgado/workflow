// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestCandidatesListsEveryTaskOnPathInOrder(t *testing.T) {
	t.Parallel()

	// file is one program in one of the three directories on the PATH list.
	type file struct {
		dir  int
		name string
		kind int
		mode os.FileMode
	}

	tests := []struct {
		name  string
		goos  string
		files []file
		want  []file
	}{
		{
			name: "only an executable task counts",
			goos: linux,
			files: []file{
				{dir: 0, name: taskProgram, mode: 0o755},
				{dir: 1, name: taskProgram, mode: 0o644},
				{dir: 2, name: taskProgram, mode: 0o755},
			},
			want: []file{{dir: 0, name: taskProgram}, {dir: 2, name: taskProgram}},
		},
		{
			name: "on windows task.exe counts and the bit does not",
			goos: windows,
			files: []file{
				{dir: 0, name: "task.exe", mode: 0o644},
				{dir: 0, name: taskProgram, mode: 0o644},
				{dir: 1, name: taskProgram, mode: 0o644},
			},
			want: []file{{dir: 0, name: "task.exe"}, {dir: 1, name: taskProgram}},
		},
		{
			name:  "a link to an executable task counts",
			goos:  linux,
			files: []file{{dir: 1, name: taskProgram, kind: symlink}},
			want:  []file{{dir: 1, name: taskProgram}},
		},
		{
			name: "a directory named task does not",
			goos: linux,
			files: []file{
				{dir: 0, name: taskProgram, kind: directory, mode: 0o755},
				{dir: 2, name: taskProgram, mode: 0o755},
			},
			want: []file{{dir: 2, name: taskProgram}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dirs := []string{t.TempDir(), t.TempDir(), t.TempDir()}
			for _, program := range test.files {
				place(t, filepath.Join(dirs[program.dir], program.name), program.kind, program.mode)
			}

			separator := ":"
			if test.goos == windows {
				separator = ";"
			}

			pathList := strings.Join(append([]string{""}, dirs...), separator)

			var want []string
			for _, program := range test.want {
				want = append(want, filepath.Join(dirs[program.dir], program.name))
			}

			// Act
			got := taskwarrior.Candidates(pathList, test.goos)

			// Assert
			if !slices.Equal(got, want) {
				t.Errorf("Candidates = %q, want %q", got, want)
			}
		})
	}
}

func TestCandidatesLeavesOutARelativePathEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	place(t, filepath.Join(dir, taskProgram), plainFile, 0o755)

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	relative, err := filepath.Rel(workDir, dir)
	if err != nil {
		t.Fatal(err)
	}

	pathList := relative + ":" + dir

	// Act
	got := taskwarrior.Candidates(pathList, linux)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want only the absolute entry's %q", pathList, got, want)
	}
}

func TestCandidatesReadsAQuotedWindowsPathEntry(t *testing.T) {
	t.Parallel()

	// Each case writes a directory into a Windows PATH list in double quotes,
	// as Windows allows so a ; in its name does not end the entry, beside a
	// plain one; quotedLast puts the quoted entry after the plain one.
	tests := map[string]struct {
		list       func(quoted, plain string) string
		quotedLast bool
	}{
		"a quoted entry": {
			list: func(quoted, plain string) string { return `"` + quoted + `";` + plain },
		},
		"a partly quoted entry": {
			list: func(quoted, plain string) string {
				parent, name := filepath.Split(quoted)

				return parent + `"` + name + `";` + plain
			},
		},
		"a quoted entry last": {
			list:       func(quoted, plain string) string { return plain + `;"` + quoted + `"` },
			quotedLast: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			quoted := filepath.Join(t.TempDir(), "Task;warrior")
			place(t, quoted, directory, 0o755)

			plain := t.TempDir()
			for _, dir := range []string{quoted, plain} {
				place(t, filepath.Join(dir, "task.exe"), plainFile, 0o644)
			}

			pathList := test.list(quoted, plain)

			want := []string{filepath.Join(quoted, "task.exe"), filepath.Join(plain, "task.exe")}
			if test.quotedLast {
				slices.Reverse(want)
			}

			// Act
			got := taskwarrior.Candidates(pathList, windows)

			// Assert
			if !slices.Equal(got, want) {
				t.Errorf("Candidates(%q) = %q, want %q", pathList, got, want)
			}
		})
	}
}

func TestCandidatesTakesAQuoteAsPartOfANameOutsideWindows(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), `say"hi`)
	place(t, dir, directory, 0o755)
	place(t, filepath.Join(dir, taskProgram), plainFile, 0o755)

	// Act
	got := taskwarrior.Candidates(dir, linux)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want %q", dir, got, want)
	}
}
