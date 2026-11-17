// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wiring_test

// GitLab's answer of whether a name is a group is kept only under the
// settings it was asked under, even when it lands after they were replaced.
// These hold an ask in glab, which says it has begun through a named pipe and
// answers once another is written to, and only Unix makes one.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// heldGroupGLab is a stand-in glab whose first ask for team's group writes
// to begun, waits for release to be written, then answers GitLab's 404; every
// later ask answers team's members. It notes each ask for the group in
// members-asked.
type heldGroupGLab struct {
	dir, begun, release string
}

// installHeldGroupGLab puts a heldGroupGLab on PATH.
func installHeldGroupGLab(t *testing.T) heldGroupGLab {
	t.Helper()

	glab := installForgeCLI(t, "glab", forgeReplies{})
	held := heldGroupGLab{
		dir: glab.dir, begun: filepath.Join(glab.dir, "begun"), release: filepath.Join(glab.dir, "release"),
	}

	for _, pipe := range []string{held.begun, held.release} {
		err := syscall.Mkfifo(pipe, 0o600)
		if err != nil {
			t.Fatalf("making the pipe %s: %v", pipe, err)
		}
	}

	write(t, filepath.Join(glab.dir, "glab"), held.script(), 0o755)

	return held
}

// script is the stand-in's shell body.
func (g heldGroupGLab) script() string {
	return "#!/bin/sh\n" + tokenLookup(g.dir) +
		"for a in \"$@\"; do url=\"$a\"; done\n" +
		"case \"$url\" in\n" +
		"  *\"/members\"*)\n" +
		"    printf x >> '" + g.dir + "/members-asked'\n" +
		"    if [ ! -e '" + g.dir + "/held' ]; then : > '" + g.dir + "/held'\n" +
		"      printf x > '" + g.begun + "'\n" +
		"      cat '" + g.release + "' > /dev/null\n" +
		"      printf 'HTTP/1.1 404 Not Found\\r\\nContent-Type: application/json\\r\\n\\r\\n{}'; exit 0; fi\n" +
		"    body='[{\"username\":\"dan\",\"state\":\"active\",\"access_level\":30}]' ;;\n" +
		"  *) body='[]' ;;\n" +
		"esac\n" +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n%s' \"$body\"\n"
}

// answer lets the held ask answer.
func (g heldGroupGLab) answer(t *testing.T) {
	t.Helper()

	err := os.WriteFile(g.release, []byte("x"), 0o600)
	if err != nil {
		t.Fatalf("letting glab answer: %v", err)
	}
}

// groupAsks is how many times glab was asked for team's group.
func (g heldGroupGLab) groupAsks() int {
	data, _ := os.ReadFile(filepath.Join(g.dir, "members-asked"))

	return strings.Count(string(data), "x")
}

func TestAGroupAnswerLandingAfterASwapIsNotKeptUnderTheNewSettings(t *testing.T) {
	// Arrange
	// An ask under t1 is held in glab while the settings swap to t2 and an
	// ask under t2 is answered: team is a group. The t1 ask then lands with
	// GitLab's 404.
	glab := installHeldGroupGLab(t)
	where := onOrigin(t.TempDir(), remoteGitLab)
	deps, controls := processEnvironment().Deps(config.Config{Forge: gitlabWithToken("t1")}, where, nil)

	var underT1 sync.WaitGroup

	underT1.Go(func() { _, _ = deps.Forge.IsGroup(t.Context(), "team") })
	awaitBegun(t, glab.begun)

	controls.UseForgeSettings(gitlabWithToken("t2"))

	_, err := deps.Forge.IsGroup(t.Context(), "team")
	if err != nil {
		t.Fatalf("the ask under t2: %v", err)
	}

	glab.answer(t)
	underT1.Wait()

	// Act
	group, err := deps.Forge.IsGroup(t.Context(), "team")

	// Assert
	if err != nil || !group {
		t.Errorf("IsGroup(team) after t1's late answer = %v, %v; want t2's group", group, err)
	}

	if asked := glab.groupAsks(); asked != 2 {
		t.Errorf("glab was asked for the group %d times, want once under each settings", asked)
	}
}
