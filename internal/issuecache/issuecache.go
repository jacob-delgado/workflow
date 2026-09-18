// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package issuecache stores the assigned-issue list on disk so the Issues pane
// can paint before Jira answers. It holds only what the pane shows — an issue's
// key, summary and status — never the heavier detail, and in a file only its
// owner can read, because issue text is at rest in it.
package issuecache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// FileMode is the permission the cache is written with. It holds issue text, so
// nobody but the owner may read it.
const FileMode os.FileMode = 0o600

// dirMode is the permission the cache directory is created with.
const dirMode os.FileMode = 0o700

// Read returns the issues last cached at path, or nil when there is no usable
// cache — a missing file, or one that cannot be decoded. A cache is only a head
// start, so any doubt about it is settled by ignoring it and waiting for Jira.
func Read(path string) []jira.Issue {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the user's own cache file, by design
	if err != nil {
		return nil
	}

	var issues []jira.Issue

	err = json.Unmarshal(contents, &issues)
	if err != nil {
		return nil
	}

	return issues
}

// Write saves issues to path in a file only its owner can read, creating the
// cache directory if it is not there yet.
func Write(path string, issues []jira.Issue) error {
	encoded, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("encoding the issue cache: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		return fmt.Errorf("making the cache directory: %w", err)
	}

	return writePrivate(path, encoded)
}

// writePrivate writes contents to path so only its owner can read it. Opening
// with a mode is not enough for a file that already exists — the cache is
// rewritten every session — so the mode is set on the open handle before
// anything is written into it.
func writePrivate(path string, contents []byte) error {
	//nolint:gosec // the path is the user's own cache file, by design
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, FileMode)
	if err != nil {
		return fmt.Errorf("opening the cache: %w", err)
	}

	err = file.Chmod(FileMode)
	if err == nil {
		_, err = file.Write(contents)
	}

	return errors.Join(err, file.Close())
}

// Path is where a Jira instance's issue list is cached: a per-user cache
// directory, in a file named for the instance so that two instances never share
// one list.
func Path(baseURL string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache directory: %w", err)
	}

	digest := sha256.Sum256([]byte(baseURL))
	name := "issues-" + hex.EncodeToString(digest[:8]) + ".json"

	return filepath.Join(dir, "workflow", name), nil
}
