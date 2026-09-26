// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// Action ids that several conflict cases name, kept as constants so the table
// does not repeat the same literal.
const (
	refreshAction = "refresh"
	mergeAction   = "merge"
)

func TestAKeyOverrideRebindsAnActionAndShowsItInTheHelp(t *testing.T) {
	t.Parallel()

	// Arrange
	// comment is bound to c by default; move it to C, which nothing else on the
	// Issues pane uses.
	rebound := newWorld()
	rebound.cfg.UI.Keys = map[string]string{"comment": "C"}
	rebound.edited = greeting
	started := rebound.live(t, 120, 50)

	// Act: press the new key on the Issues pane
	onNewKey := typing(t, started, "C").View().Content

	// Assert: the comment composer opened, so the action fired on its new key
	requireScreen(t, onNewKey, greeting)

	// Act: press the key it used to be bound to
	onOldKey := typing(t, started, "c").View().Content

	// Assert: nothing happens — c no longer comments
	refuseScreen(t, onOldKey, greeting)

	// Act: open the help
	helpView := typing(t, started, "?").View().Content

	// Assert: the help lists comment on its new key
	requireScreen(t, helpView, "C"+strings.Repeat(" ", 9)+"comment")
}

func TestCheckKeysAcceptsTheDefaultBindings(t *testing.T) {
	t.Parallel()

	// Act
	// The default set is chosen so no two actions collide in any one context;
	// were that not so, the interface would refuse to start.
	err := tui.CheckKeys(nil)
	// Assert
	if err != nil {
		t.Errorf("CheckKeys(nil) = %v, want no conflict in the default bindings", err)
	}
}

func TestCheckKeysRefusesTwoActionsSharingAKeyInOneContext(t *testing.T) {
	t.Parallel()

	// Arrange
	// commit and stage-all both live on the Branch and Commits panes; binding
	// commit to stage-all's key makes a press there ambiguous.
	colliding := map[string]string{"commit": "a"}

	// Act
	err := tui.CheckKeys(colliding)

	// Assert
	if !errors.Is(err, tui.ErrKeyConflict) {
		t.Fatalf("CheckKeys = %v, want ErrKeyConflict", err)
	}

	if got := err.Error(); !strings.Contains(got, "commit") || !strings.Contains(got, "stage-all") {
		t.Errorf("CheckKeys error %q does not name both colliding actions", got)
	}
}

func TestCheckKeysNamesTheReviewPanesForNoParticularService(t *testing.T) {
	t.Parallel()

	// Arrange
	// merge and edit both answer on the Review pane. The check runs before any
	// service is chosen, so the context it names must fit Teams as well as Slack.
	colliding := map[string]string{mergeAction: "e"}

	// Act
	err := tui.CheckKeys(colliding)

	// Assert
	if err == nil {
		t.Fatal("CheckKeys accepted merge rebound onto edit's key")
	}

	if got := err.Error(); !strings.Contains(got, "the Review and messaging panes") {
		t.Errorf("CheckKeys error %q does not name the Review and messaging panes", got)
	}
}

func TestCheckKeysCatchesConflictsOnCrossPaneKeys(t *testing.T) {
	t.Parallel()

	// Each rebind collides with a binding that is handled on a pane whose help
	// group differs from the rebound action's — the list actions refresh,
	// open-link and copy-link, edit on the Review pane, up in a field form. A
	// context modeled by help group alone would accept these and leave the press
	// ambiguous at runtime, so CheckKeys must still refuse them.
	cases := []struct {
		name     string
		override map[string]string
		collides string
	}{
		{"rebase onto refresh, Branch pane", map[string]string{"rebase": "r"}, refreshAction},
		{"commit onto refresh, Commits pane", map[string]string{"commit": "r"}, refreshAction},
		{"refresh onto push, Branch pane", map[string]string{refreshAction: "P"}, "push"},
		{"checks onto open-link, Review pane", map[string]string{"checks": "o"}, "open-link"},
		{"merge onto refresh, Review pane", map[string]string{mergeAction: "r"}, refreshAction},
		{"rerun onto copy-link, Review pane", map[string]string{"rerun-checks": "y"}, "copy-link"},
		{"merge onto edit, Review pane", map[string]string{mergeAction: "e"}, "edit"},
		{"open-link onto merge, Review pane", map[string]string{"open-link": "M"}, mergeAction},
		{"toggle-option onto up, a field form", map[string]string{"toggle-option": "k"}, "up"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rebound, collides := testCase.override, testCase.collides

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, tui.ErrKeyConflict) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConflict", rebound, err)
			}

			for action := range rebound {
				if !strings.Contains(err.Error(), action) {
					t.Errorf("CheckKeys error %q does not name the rebound action %q", err, action)
				}
			}

			if got := err.Error(); !strings.Contains(got, collides) {
				t.Errorf("CheckKeys error %q does not name the collided action %q", got, collides)
			}
		})
	}
}

func TestCheckKeysRefusesAnOverlayKeyOnAKeyLiveBesideItInAComposer(t *testing.T) {
	t.Parallel()

	// Only the branch creator answers worktree and only the messaging preview
	// answers post-when-green, so each is live beside the composer's keys.
	cases := []struct {
		name     string
		override map[string]string
		collides string
	}{
		{"worktree onto edit", map[string]string{"worktree": "e"}, "edit"},
		{"post-when-green onto verbatim", map[string]string{"post-when-green": "v"}, "verbatim"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rebound, collides := testCase.override, testCase.collides

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, tui.ErrKeyConflict) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConflict", rebound, err)
			}

			if got := err.Error(); !strings.Contains(got, collides) || !strings.Contains(got, "a composer") {
				t.Errorf("CheckKeys error %q does not name %q in a composer", got, collides)
			}
		})
	}
}

func TestCheckKeysAcceptsAnOverlayKeyOnAKeyOfThePaneBehindIt(t *testing.T) {
	t.Parallel()

	// The pane behind an open overlay does not answer while it is open, so an
	// overlay's key may share a key with that pane's own.
	cases := map[string]map[string]string{
		"worktree onto switch-task":           {"worktree": "s"},
		"post-when-green onto merge":          {"post-when-green": "M"},
		"cycle-type-left onto open a request": {"cycle-type-left": "n"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)
			// Assert
			if err != nil {
				t.Errorf("CheckKeys(%v) = %v, want no conflict", rebound, err)
			}
		})
	}
}

func TestCheckKeysRefusesAnUnknownAction(t *testing.T) {
	t.Parallel()

	// Arrange
	misnamed := map[string]string{"no-such-action": "C"}

	// Act
	err := tui.CheckKeys(misnamed)

	// Assert
	if !errors.Is(err, tui.ErrUnknownKeyAction) {
		t.Fatalf("CheckKeys = %v, want ErrUnknownKeyAction", err)
	}

	if got := err.Error(); !strings.Contains(got, "no-such-action") {
		t.Errorf("CheckKeys error %q does not name the unknown action", got)
	}
}

func TestCheckKeysRefusesMovingJumpToPane(t *testing.T) {
	t.Parallel()

	// Arrange
	// jump-to-pane answers one digit per pane, which no single key can stand
	// in for, so moving it to a free key is refused rather than accepted.
	moved := map[string]string{"jump-to-pane": "f12"}

	// Act
	err := tui.CheckKeys(moved)

	// Assert
	if !errors.Is(err, tui.ErrKeyNotRebindable) {
		t.Fatalf("CheckKeys = %v, want ErrKeyNotRebindable", err)
	}

	if got := err.Error(); !strings.Contains(got, "jump-to-pane") {
		t.Errorf("CheckKeys error %q does not name the action", got)
	}
}
