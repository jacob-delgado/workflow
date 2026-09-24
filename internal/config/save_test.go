// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// The file contents these tests write, as a hand edit would leave them.
const (
	readContents   = `{"jira": {"base_url": "https://read.example.com"}}`
	editedContents = `{"jira": {"base_url": "https://edited.example.com"}}`
)

// revisionOf is the revision of the file at path, failing the test when it
// cannot be read.
func revisionOf(t *testing.T, path string) config.Revision {
	t.Helper()

	revision, err := config.RevisionOf(path)
	if err != nil {
		t.Fatalf("RevisionOf(%s): %v", path, err)
	}

	return revision
}

// withBaseURL is the default configuration with Jira at baseURL.
func withBaseURL(baseURL string) config.Config {
	cfg := config.Default()
	cfg.Jira.BaseURL = baseURL

	return cfg
}

func TestSaveOverWritesOverTheRevisionItWasGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	path := write(t, t.TempDir(), readContents)
	read := revisionOf(t, path)

	// Act
	written, err := config.SaveOver(path, withBaseURL("https://saved.example.com"), read)
	// Assert
	if err != nil {
		t.Fatalf("SaveOver: %v", err)
	}

	saved, err := config.LoadFile(path)
	if err != nil || saved.Jira.BaseURL != "https://saved.example.com" {
		t.Errorf("the file reads back as Jira at %q (%v), want the saved configuration", saved.Jira.BaseURL, err)
	}

	if now := revisionOf(t, path); written != now || !written.Exists() {
		t.Errorf("SaveOver returned revision %v, want the file's revision now, %v", written, now)
	}
}

func TestSaveOverWritesAFileThatIsStillMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)

	// Act
	written, err := config.SaveOver(path, withBaseURL("https://saved.example.com"), config.Revision{})
	// Assert
	if err != nil {
		t.Fatalf("SaveOver: %v", err)
	}

	if now := revisionOf(t, path); written != now || !now.Exists() {
		t.Errorf("SaveOver returned revision %v, want the revision of the file it made, %v", written, now)
	}
}

func TestSaveOverRefusesAFileThatChangedSinceItsRevision(t *testing.T) {
	t.Parallel()

	// "" stands for no file, before the read or after the change.
	cases := map[string]struct{ before, after string }{
		"edited since it was read":          {before: readContents, after: editedContents},
		"made since it was read as missing": {before: "", after: editedContents},
		"deleted since it was read":         {before: readContents, after: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := filepath.Join(t.TempDir(), config.FileName)
			putFile(t, path, tt.before)
			read := revisionOf(t, path)
			putFile(t, path, tt.after)

			// Act
			_, err := config.SaveOver(path, withBaseURL("https://saved.example.com"), read)

			// Assert
			if !errors.Is(err, config.ErrChangedOnDisk) {
				t.Errorf("SaveOver = %v, want ErrChangedOnDisk", err)
			}

			if got := fileContents(t, path); got != tt.after {
				t.Errorf("the file holds %q, want the change left as it was, %q", got, tt.after)
			}
		})
	}
}

func TestSaveOverReportsAPathItCannotUse(t *testing.T) {
	t.Parallel()

	cases := map[string]func(dir string) string{
		// Reading the revision is what fails.
		"a directory": func(dir string) string { return dir },
		// The revision reads as no file; writing it is what fails.
		"a file in a missing directory": func(dir string) string { return filepath.Join(dir, "missing", config.FileName) },
	}

	for name, pathIn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := pathIn(t.TempDir())

			// Act
			_, err := config.SaveOver(path, config.Default(), config.Revision{})

			// Assert
			if err == nil || errors.Is(err, config.ErrChangedOnDisk) {
				t.Errorf("SaveOver = %v, want the failure to read or write the path", err)
			}
		})
	}
}

func TestRevisionOfNoFileIsNoFile(t *testing.T) {
	t.Parallel()

	cases := map[string]func(dir string) string{
		"a missing file": func(dir string) string { return filepath.Join(dir, config.FileName) },
		"an empty path":  func(string) string { return "" },
	}

	for name, pathIn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			revision, err := config.RevisionOf(pathIn(t.TempDir()))

			// Assert
			if err != nil || revision.Exists() || revision != (config.Revision{}) {
				t.Errorf("RevisionOf = %v, %v; want the no-file revision and no error", revision, err)
			}
		})
	}
}

func TestRevisionOfReportsAFileItCannotRead(t *testing.T) {
	t.Parallel()

	// Act
	_, err := config.RevisionOf(t.TempDir())

	// Assert
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("RevisionOf a directory = %v, want the read's own failure", err)
	}
}

func TestARevisionIsTheFilesContents(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		rewritten string
		same      bool
	}{
		"the same bytes written again": {rewritten: readContents, same: true},
		"one value changed":            {rewritten: editedContents, same: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := write(t, t.TempDir(), readContents)
			read := revisionOf(t, path)
			putFile(t, path, tt.rewritten)

			// Act
			now := revisionOf(t, path)

			// Assert
			if (now == read) != tt.same {
				t.Errorf("revision after the rewrite = %v, before = %v; want the same: %t", now, read, tt.same)
			}
		})
	}
}

func TestARevisionIsNoPlainHashOfTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file holds credentials, and its revision leaves the process as an
	// ETag, so the revision must not be a digest anyone can compute from it.
	path := write(t, t.TempDir(), readContents)
	plain := sha256.Sum256([]byte(readContents))

	// Act
	revision := revisionOf(t, path)

	// Assert
	if revision.String() == hex.EncodeToString(plain[:]) {
		t.Errorf("revision %v is the file's plain SHA-256, want a keyed one", revision)
	}
}

func TestARevisionReadsBackFromItsText(t *testing.T) {
	t.Parallel()

	cases := map[string]string{"a file": readContents, "no file": ""}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := filepath.Join(t.TempDir(), config.FileName)
			putFile(t, path, contents)
			revision := revisionOf(t, path)

			// Act
			parsed, err := config.ParseRevision(revision.String())

			// Assert
			if err != nil || parsed != revision {
				t.Errorf("ParseRevision(%q) = %v, %v; want %v back", revision.String(), parsed, err, revision)
			}
		})
	}
}

func TestParseRevisionRefusesTextThatNamesNoRevision(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"nothing":                 "",
		"not hexadecimal":         "not-a-revision",
		"a digest cut short":      "0123456789abcdef",
		"a quoted revision":       `"none"`,
		"a digest one digit over": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0",
	}

	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.ParseRevision(text)

			// Assert
			if !errors.Is(err, config.ErrNotARevision) {
				t.Errorf("ParseRevision(%q) = %v, want ErrNotARevision", text, err)
			}
		})
	}
}

// putFile leaves path holding contents, or removes it when contents is empty.
func putFile(t *testing.T, path, contents string) {
	t.Helper()

	if contents == "" {
		err := os.Remove(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("removing %s: %v", path, err)
		}

		return
	}

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// fileContents is what the file at path holds, or "" when there is none.
func fileContents(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}

	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(contents)
}
