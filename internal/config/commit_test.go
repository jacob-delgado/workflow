// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestCommitSectionIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"commit": {"default_scope": "api"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Commit.DefaultScope != "api" {
		t.Errorf("commit = %+v, want the configured default scope", cfg.Commit)
	}
}

func TestADefaultScopeWithBadCharactersIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"commit": {"default_scope": "Two Words"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidCommit) {
		t.Errorf("Load returned %v, want it refused for an ill-formed default scope", err)
	}
}

func TestTheCommitConventionFieldsAreRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"commit": {"types": ["hotfix", "chore"], "subject_limit": 50, "refs_trailer": "Closes"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	commit := cfg.Commit
	if len(commit.Types) != 2 || commit.Types[0] != "hotfix" ||
		commit.SubjectLimit != 50 || commit.RefsTrailer != "Closes" {
		t.Errorf("commit = %+v, want the configured convention", commit)
	}
}

func TestAnIllFormedCommitConventionIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a capitalized type":       `{"commit": {"types": ["Feat"]}}`,
		"a negative subject limit": `{"commit": {"subject_limit": -5}}`,
		"a trailer with a colon":   `{"commit": {"refs_trailer": "Refs:"}}`,
		"a trailer with two words": `{"commit": {"refs_trailer": "See also"}}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, body)

			// Act
			_, err := config.Load(workDir, t.TempDir())

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidCommit) {
				t.Errorf("Load returned %v, want the ill-formed commit convention refused", err)
			}
		})
	}
}
