// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

func TestLayersStandAloneWhereTheRepositoryIsHome(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()

	err := os.WriteFile(filepath.Join(home, config.FileName), []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	// Act
	layers := setup.Where{WorkDir: home, HomeDir: home}.Layers(setup.Repository)

	// Assert
	if layers != (config.Files{}) {
		t.Errorf("Layers = %+v, want the file to stand alone", layers)
	}
}

//nolint:paralleltest // t.Chdir moves the whole process, so this runs serially.
func TestLayersStandAloneWithoutAHomeDirectory(t *testing.T) {
	// Arrange
	workDir := t.TempDir()
	t.Chdir(workDir)

	err := os.WriteFile(filepath.Join(workDir, config.FileName), []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing a file in the working directory: %v", err)
	}

	// Act
	layers := setup.Where{WorkDir: t.TempDir()}.Layers(setup.Repository)

	// Assert
	if layers != (config.Files{}) {
		t.Errorf("Layers without a home directory = %+v, want the file to stand alone", layers)
	}
}

func TestBeneathRefusesAHomeFileThatIsNoConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	// The home file can be read, so the pair has a revision, but it holds no
	// configuration for a new file to start from. A write would be refused
	// again when it read the pair to save, but a dry run starts from this
	// alone.
	where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}

	err := os.WriteFile(where.Path(setup.Home), []byte(`{"jira":`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	// Act
	_, _, err = setup.Beneath(where.Layers(setup.Repository))

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Beneath = %v, want %v: no configuration to start from", err, config.ErrInvalid)
	}
}

func TestWriteInARepositoryLiesOverTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	home := guide.Where.Path(setup.Home)

	err := os.WriteFile(home, []byte(`{"jira":{"base_url":"https://home.example.com","project":"HOME"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	// Act
	written, err := guide.Write(t.Context(), answered(setup.Repository))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	cfg, _, err := config.LoadLayersAt(guide.Where.Layers(setup.Repository))
	if err != nil || cfg.Jira.Project != "HOME" || cfg.Jira.BaseURL != jiraAddress {
		t.Errorf("read %+v (%v) over %s, want the repository's answers over the home file's project",
			cfg.Jira, err, written.Path)
	}
}

func TestWriteOverAHomeFileItCannotReadWritesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		contents string
		mode     os.FileMode
		want     error
	}{
		"a home file others may write": {
			contents: `{"jira":{"project":"HOME"}}`, mode: 0o666, want: config.ErrUntrustedFile,
		},
		"a home file that is no configuration": {
			contents: `{"jira":`, mode: config.FileMode, want: config.ErrInvalid,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			guide := guideIn(t, acceptingJira())
			home := guide.Where.Path(setup.Home)
			writeHomeFile(t, guide, tt.contents)

			// os.WriteFile leaves the mode to the umask; the case sets its own.
			err := os.Chmod(home, tt.mode)
			if err != nil {
				t.Fatalf("setting the home file's mode: %v", err)
			}

			// Act
			_, err = guide.Write(t.Context(), answered(setup.Repository))

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Write = %v, want %v", err, tt.want)
			}

			_, statErr := os.Lstat(guide.Where.Path(setup.Repository))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("the repository's file is there (%v), want nothing written", statErr)
			}
		})
	}
}
