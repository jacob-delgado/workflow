// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// newLayer is a home file holding homeFile, and where a repository's file
// over it will go, not yet written.
func newLayer(t *testing.T) config.Files {
	t.Helper()

	return config.Files{
		Home: write(t, t.TempDir(), homeFile), Repo: filepath.Join(t.TempDir(), config.FileName),
	}
}

// overNewLayer is the home file's configuration with the project changed, and
// the revision of the pair, as setup reads them before it writes.
func overNewLayer(t *testing.T, files config.Files) (config.Config, config.Revision) {
	t.Helper()

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the home file: %v", err)
	}

	cfg.Jira.Project = ossProject

	return cfg, over
}

func TestCreateLayersWritesTheNewFileAsTheLayerOverHome(t *testing.T) {
	t.Parallel()

	// Arrange
	files := newLayer(t)
	cfg, over := overNewLayer(t, files)

	// Act
	err := config.CreateLayers(files, cfg, over)

	// Assert
	written, readErr := os.ReadFile(files.Repo)
	if err != nil || readErr != nil || !strings.Contains(string(written), ossProject) ||
		strings.Contains(string(written), homeToken) {
		t.Errorf("CreateLayers = %v; the new file holds %q (%v); want the project alone, no inherited token",
			err, written, readErr)
	}
}

func TestCreateLayersOverAHomeFileChangedSinceItsReadWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	files := newLayer(t)
	cfg, over := overNewLayer(t, files)

	err := os.WriteFile(files.Home, []byte(`{"jira": {"project": "EDITED"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("editing the home file: %v", err)
	}

	// Act
	err = config.CreateLayers(files, cfg, over)

	// Assert
	_, statErr := os.Lstat(files.Repo)
	if !errors.Is(err, config.ErrChangedOnDisk) || !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("CreateLayers = %v, new file %v; want ErrChangedOnDisk and nothing written", err, statErr)
	}
}

func TestCreateLayersWritesNothingOverAFileAlreadyThere(t *testing.T) {
	t.Parallel()

	// Arrange
	files := newLayer(t)
	cfg, _ := overNewLayer(t, files)
	write(t, filepath.Dir(files.Repo), `{}`)

	// The pair's revision once the file is there, so the write is refused
	// for the file rather than for the change.
	over, err := config.RevisionOfLayers(files)
	if err != nil {
		t.Fatalf("reading the revision: %v", err)
	}

	// Act
	err = config.CreateLayers(files, cfg, over)

	// Assert
	kept, readErr := os.ReadFile(files.Repo)
	if !errors.Is(err, fs.ErrExist) || readErr != nil || string(kept) != `{}` {
		t.Errorf("CreateLayers = %v, file %q (%v); want fs.ErrExist and the file kept", err, kept, readErr)
	}
}

func TestCreateLayersRefusesWhatItCouldNotWriteSafely(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange func(t *testing.T, files config.Files, cfg *config.Config)
		want    error
	}{
		"a link where the file goes": {
			arrange: func(t *testing.T, files config.Files, _ *config.Config) {
				t.Helper()

				err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), files.Repo)
				if err != nil {
					t.Skipf("making a link: %v", err)
				}
			},
			want: config.ErrLinkedFile,
		},
		"a setting only the home file may make": {
			arrange: func(t *testing.T, _ config.Files, cfg *config.Config) {
				t.Helper()

				cfg.Jira.TokenCommand = jiraTokenCommand
			},
			want: config.ErrHomeOnly,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := newLayer(t)
			cfg, over := overNewLayer(t, files)
			tt.arrange(t, files, &cfg)

			// Act
			err := config.CreateLayers(files, cfg, over)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("CreateLayers = %v, want %v", err, tt.want)
			}
		})
	}
}
