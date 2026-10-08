// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// onceAGroupScript is a stand-in glab that knows no user named acme and
// answers acme's group members once, then fails every later ask.
func onceAGroupScript(dir string) string {
	return "#!/bin/sh\n" + tokenLookup(dir) +
		"for a in \"$@\"; do url=\"$a\"; done\n" +
		"case \"$url\" in\n" +
		"  *\"/members\"*)\n" +
		"    if [ -e \"" + dir + "/answered\" ]; then printf 'boom\\n' >&2; exit 3; fi\n" +
		"    : > \"" + dir + "/answered\"\n" +
		"    body='[{\"username\":\"dan\",\"state\":\"active\",\"access_level\":30}]' ;;\n" +
		"  *) body='[]' ;;\n" +
		"esac\n" +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n%s' \"$body\"\n"
}

func TestAGroupStaysAGroupWhenALaterLookupFails(t *testing.T) {
	// Arrange
	// The web asks at the preview and again at the post; a GitLab failing
	// only the second time must not turn the group into a person between
	// them.
	glab := installForgeCLI(t, "glab", forgeReplies{})
	write(t, filepath.Join(glab.dir, "glab"), onceAGroupScript(glab.dir), 0o755)

	cfg := config.Config{Forge: config.Forge{CLI: true}}
	where := wiring.Workspace{Root: t.TempDir(), Remote: remoteGitLab}
	isGroup := wired(t, cfg, where, nil).Forge.IsGroup
	_, _ = isGroup("acme")

	// Act
	group, err := isGroup("acme")

	// Assert
	if err != nil || !group {
		t.Errorf("IsGroup after a failed lookup = %v, %v; want the group it was already known as", group, err)
	}
}

func TestIsGroupAsksGitLabOnceSettingsSavedWhileRunningSwitchToIt(t *testing.T) {
	// Arrange
	// The remote's host names no forge, so only the settings say which it is:
	// GitHub at the start, GitLab once the web's Settings saves it.
	installForgeCLI(t, "gh", forgeReplies{})
	glab := installForgeCLI(t, "glab", forgeReplies{})
	write(t, filepath.Join(glab.dir, "glab"), onceAGroupScript(glab.dir), 0o755)

	where := wiring.Workspace{Root: t.TempDir(), Remote: onPremisesRemote}
	deps, controls := wiring.Deps(t.Context(), config.Config{Forge: throughCLIOnPremises()}, where, nil)

	controls.UseForgeSettings(config.Forge{CLI: true, Kind: "gitlab", Host: onPremisesHost})

	isGroup := deps.Forge.IsGroup
	if isGroup == nil {
		t.Fatal("IsGroup is nil, so GitLab is never asked whether a name is a group")
	}

	// Act
	group, err := isGroup("acme")

	// Assert
	if err != nil || !group {
		t.Errorf("IsGroup(acme) after switching to GitLab = %v, %v; want the group GitLab knows", group, err)
	}
}
