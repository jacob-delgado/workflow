// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// Forge settings saved while workflow runs — from the web's Settings — reach the
// next forge call, and replace a connection already made with the settings
// before them. The remote is on a host that names no forge, so only the
// settings say which it is, and no call ever leaves for a real one.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

const (
	onPremisesHost   = "git.example.com"
	onPremisesRemote = "https://" + onPremisesHost + "/owner/repo.git"
)

// throughCLIOnPremises is the settings that say the remote's host is a GitHub
// reached through gh.
func throughCLIOnPremises() config.Forge {
	return config.Forge{CLI: true, Kind: githubKind, Host: onPremisesHost}
}

func TestForgeSettingsSavedWhileRunningReachTheNextCall(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})
	where := wiring.Workspace{Root: t.TempDir(), Remote: onPremisesRemote}
	deps, controls := processEnvironment().Deps(t.Context(), config.Config{}, where, nil)

	// Act
	kind := controls.UseForgeSettings(throughCLIOnPremises())
	_, _, err := deps.Forge.FindPullRequest("feat/x")

	// Assert
	if kind != forge.KindGitHub || err != nil {
		t.Errorf("after the save: kind %v, FindPullRequest %v; want GitHub, answered", kind, err)
	}
}

func TestForgeSettingsSavedWhileRunningReplaceTheConnectionMade(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})
	where := wiring.Workspace{Root: t.TempDir(), Remote: onPremisesRemote}
	deps, controls := processEnvironment().Deps(t.Context(), config.Config{Forge: throughCLIOnPremises()}, where, nil)

	_, _, err := deps.Forge.FindPullRequest("feat/x")
	if err != nil {
		t.Fatalf("the first call = %v, want it answered through gh", err)
	}

	// Act
	controls.UseForgeSettings(config.Forge{})

	_, _, err = deps.Forge.FindPullRequest("feat/x")

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) {
		t.Errorf("after the save = %v, want the host no setting names any more (%v)", err, forge.ErrUnknownForge)
	}
}
