// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestTheTitleSourceIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"pull_request": {"title_source": "issue"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.PullRequest.TitleSource != "issue" {
		t.Errorf("pull_request = %+v, want the configured title source", cfg.PullRequest)
	}
}

func TestAnUnknownTitleSourceIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"pull_request": {"title_source": "headline"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidPullRequest) {
		t.Errorf("Load returned %v, want an unknown title source refused", err)
	}
}
