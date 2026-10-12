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

// outsideContents is what a file a configuration file links to holds before
// a save.
const outsideContents = "export PATH=$HOME/bin:$PATH\n"

// linkedTo makes the file for place in where a link to a file outside it
// holding outsideContents, and returns that file.
func linkedTo(t *testing.T, where setup.Where, place setup.Place) string {
	t.Helper()

	target := filepath.Join(t.TempDir(), "profile")

	err := os.WriteFile(target, []byte(outsideContents), 0o600)
	if err != nil {
		t.Fatalf("writing the file the link names: %v", err)
	}

	err = os.Symlink(target, where.Path(place))
	if err != nil {
		t.Fatalf("linking the file: %v", err)
	}

	return target
}

// contentsOf is what the file at path holds, failing the test unless it
// could be read.
func contentsOf(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(contents)
}

func TestSaveRefusesARepositoryFileThatIsALink(t *testing.T) {
	t.Parallel()

	// Arrange
	where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}
	target := linkedTo(t, where, setup.Repository)

	// Act
	err := where.Save(setup.Repository, config.Default(), config.Revision{})

	// Assert
	if !errors.Is(err, config.ErrLinkedFile) || contentsOf(t, target) != outsideContents {
		t.Errorf("Save through a linked repository file = %v, and the file it names holds %q; "+
			"want ErrLinkedFile and that file left as it was", err, contentsOf(t, target))
	}
}

func TestSaveFollowsAHomeFileThatIsALink(t *testing.T) {
	t.Parallel()

	// Arrange
	where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}
	target := linkedTo(t, where, setup.Home)
	cfg := config.Default()
	cfg.Jira.BaseURL = jiraAddress

	// Act
	err := where.Save(setup.Home, cfg, config.Revision{})

	// Assert
	saved, _, loadErr := config.LoadLayersAt(config.Files{Home: target})
	if err != nil || loadErr != nil || saved.Jira.BaseURL != jiraAddress {
		t.Errorf("Save through a linked home file = %v; the file it names loads %+v (%v); want it written there",
			err, saved.Jira, loadErr)
	}
}

func TestRefuseLinkedTakesARepositoryFileThatIsNoLink(t *testing.T) {
	t.Parallel()

	cases := map[string]string{"a file": `{}`, "nothing": ""}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			where := setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}
			if contents != "" {
				err := os.WriteFile(where.Path(setup.Repository), []byte(contents), config.FileMode)
				if err != nil {
					t.Fatalf("writing the repository's file: %v", err)
				}
			}

			// Act
			err := where.RefuseLinked(setup.Repository)
			// Assert
			if err != nil {
				t.Errorf("RefuseLinked of %s = %v, want it taken", name, err)
			}
		})
	}
}
