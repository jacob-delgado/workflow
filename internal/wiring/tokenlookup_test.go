// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// A gh stopped before it gave a token has not said it holds none: the call
// that asked finds the forge not reached, not that no token was found.
func TestAForgeCallWhoseTokenLookupWasCutShortFindsTheForgeUnreachable(t *testing.T) {
	// Arrange
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	tokenLookup := newHeldProgram(t, "gh")
	t.Setenv("PATH", filepath.Dir(tokenLookup.path)+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := config.Config{Forge: config.Forge{Kind: githubKind, Host: hostGitHub}}
	forgeSide := wired(t, cfg, onOrigin(t.TempDir(), remoteGitHub), nil).Forge

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	// Act
	_, err := forgeSide.ReviewRequests(ctx)

	// Assert
	if !errors.Is(err, forge.ErrUnreachable) || errors.Is(err, forge.ErrNoToken) {
		t.Errorf("ReviewRequests = %v, want %v, not %v", err, forge.ErrUnreachable, forge.ErrNoToken)
	}
}
