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
		files []file
		want  []file
	}{
		{
			name: "only an executable task counts",
			files: []file{
				{dir: 0, name: taskProgram, mode: 0o755},
				{dir: 1, name: taskProgram, mode: 0o644},
				{dir: 2, name: taskProgram, mode: 0o755},
			},
			want: []file{{dir: 0, name: taskProgram}, {dir: 2, name: taskProgram}},
		},
		{
			name:  "a link to an executable task counts",
			files: []file{{dir: 1, name: taskProgram, kind: symlink}},
			want:  []file{{dir: 1, name: taskProgram}},
		},
		{
			name: "a directory named task does not",
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

			pathList := strings.Join(append([]string{""}, dirs...), ":")

			var want []string
			for _, program := range test.want {
				want = append(want, filepath.Join(dirs[program.dir], program.name))
			}

			// Act
			got := taskwarrior.Candidates(pathList)

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
	got := taskwarrior.Candidates(pathList)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want only the absolute entry's %q", pathList, got, want)
	}
}

func TestCandidatesTakesAQuoteAsPartOfAName(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), `say"hi`)
	place(t, dir, directory, 0o755)
	place(t, filepath.Join(dir, taskProgram), plainFile, 0o755)

	// Act
	got := taskwarrior.Candidates(dir)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want %q", dir, got, want)
	}
}
