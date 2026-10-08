// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// outsideFile is a file outside any repository a link could point at.
const outsideFile = `{"jira": {"project": "OUT"}}`

// linkedRepository is a home file, and a repository's file that is a link to
// target.
func linkedRepository(t *testing.T, target string) config.Files {
	t.Helper()

	repo := filepath.Join(t.TempDir(), config.FileName)

	err := os.Symlink(target, repo)
	if err != nil {
		t.Skipf("making a link: %v", err)
	}

	return config.Files{Home: write(t, t.TempDir(), `{}`), Repo: repo}
}

func TestARepositoryFileThatIsALinkIsNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	files := linkedRepository(t, write(t, t.TempDir(), outsideFile))

	// Act
	_, _, err := config.LoadLayersAt(files)

	// Assert
	if !errors.Is(err, config.ErrLinkedFile) {
		t.Errorf("LoadLayersAt = %v; want ErrLinkedFile", err)
	}
}

func TestASaveNeverWritesThroughARepositoryFileThatIsALink(t *testing.T) {
	t.Parallel()

	for name, outside := range map[string]bool{"to a file": true, "to nothing yet": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			target := filepath.Join(t.TempDir(), "target")
			if outside {
				target = write(t, filepath.Dir(target), outsideFile)
			}

			files := linkedRepository(t, target)
			cfg := config.Default()
			cfg.Jira.Project = ossProject

			// The revision a read through the link would have found, so the
			// save is refused for the link rather than for a changed file.
			over, _ := config.RevisionOfLayers(files)

			// Act
			_, err := config.SaveLayers(files, cfg, over)

			// Assert
			kept, readErr := os.ReadFile(target)
			if !errors.Is(err, config.ErrLinkedFile) || (outside && string(kept) != outsideFile) ||
				(!outside && !errors.Is(readErr, os.ErrNotExist)) {
				t.Errorf("SaveLayers = %v; the link's target holds %q (%v); want ErrLinkedFile and it untouched",
					err, kept, readErr)
			}
		})
	}
}

func TestAHomeFileThatIsALinkIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	home := filepath.Join(t.TempDir(), config.FileName)

	err := os.Symlink(write(t, t.TempDir(), `{"jira": {"project": "HOME"}}`), home)
	if err != nil {
		t.Skipf("making a link: %v", err)
	}

	// Act
	cfg, _, err := config.LoadLayersAt(config.Files{Home: home})

	// Assert
	if err != nil || cfg.Jira.Project != "HOME" {
		t.Errorf("LoadLayersAt = %+v, %v; want the linked home file read", cfg.Jira, err)
	}
}
