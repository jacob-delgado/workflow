// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// Action ids that several conflict cases name, kept as constants so the table
// does not repeat the same literal.
const (
	refreshAction       = "refresh"
	mergeAction         = "merge"
	openLinkAction      = "open-link"
	addTaskAction       = "add-task"
	worktreeAction      = "worktree"
	editAction          = "edit"
	applyAction         = "apply"
	commitAction        = "commit"
	postWhenGreenAction = "post-when-green"
	toggleDraftAction   = "toggle-draft"
	editBodyAction      = "edit-body"
)

// reboundRefreshKey is the key a test's ui.keys binds refresh to.
const reboundRefreshKey = "ctrl+r"

func TestAKeyOverrideRebindsAnActionAndShowsItInTheHelp(t *testing.T) {
	t.Parallel()

	// Arrange
	// comment is bound to c by default; move it to C, which nothing else on the
	// Issues pane uses.
	rebound := newWorld()
	rebound.cfg.UI.Keys = map[string]string{"comment": "C"}
	started := rebound.live(t, 120, 50)

	// Act: press the new key on the Issues pane
	onNewKey := typing(t, started, "C").View().Content

	// Assert: the comment composer opened, so the action fired on its new key
	requireScreen(t, onNewKey, "Comment on "+issueKey)

	// Act: press the key it used to be bound to
	onOldKey := typing(t, started, "c").View().Content

	// Assert: nothing happens — c no longer comments
	refuseScreen(t, onOldKey, "Comment on "+issueKey)

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
	// commit and stage-all both live on the Commits pane; binding commit to
	// stage-all's key makes a press there ambiguous.
	colliding := map[string]string{commitAction: "a"}

	// Act
	err := tui.CheckKeys(colliding)

	// Assert
	if !errors.Is(err, config.ErrKeyConflict) {
		t.Fatalf("CheckKeys = %v, want ErrKeyConflict", err)
	}

	if got := err.Error(); !strings.Contains(got, "commit") || !strings.Contains(got, "stage-all") {
		t.Errorf("CheckKeys error %q does not name both colliding actions", got)
	}
}

func TestCheckKeysNamesTheMessagingPaneForNoParticularService(t *testing.T) {
	t.Parallel()

	// Arrange
	// post and people-and-groups both answer on the messaging pane. The check
	// runs before any service is chosen, so the pane it names must fit Teams as
	// well as Slack.
	colliding := map[string]string{"post": "P"}

	// Act
	err := tui.CheckKeys(colliding)

	// Assert
	if !errors.Is(err, config.ErrKeyConflict) {
		t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConflict", colliding, err)
	}

	if got := err.Error(); !strings.Contains(got, "the messaging pane") {
		t.Errorf("CheckKeys error %q does not name the messaging pane", got)
	}
}

// Each pane is a keyboard surface of its own: an action on one and an action
// on another are never live at once, so one key may serve both.
func TestCheckKeysLetsTwoPanesShareAKey(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"merge on the Review pane onto post on the messaging pane":     {mergeAction: "p"},
		"new-branch on the Branch pane onto stage on the Commits pane": {"new-branch": "space"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)
			// Assert
			if err != nil {
				t.Errorf("CheckKeys(%v) = %v, want the two panes' actions free to share a key", rebound, err)
			}
		})
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
		{"checks onto open-link, Review pane", map[string]string{"checks": "o"}, openLinkAction},
		{"merge onto refresh, Review pane", map[string]string{mergeAction: "r"}, refreshAction},
		{"rerun onto copy-link, Review pane", map[string]string{"rerun-checks": "y"}, "copy-link"},
		{"merge onto edit, Review pane", map[string]string{mergeAction: "e"}, editAction},
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
			if !errors.Is(err, config.ErrKeyConflict) {
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

func TestCheckKeysRefusesAnOverlayKeyOnAnotherKeyTheSameOverlayAnswers(t *testing.T) {
	t.Parallel()

	// Only the branch creator answers worktree and only the announcement
	// preview answers post-when-green, so each is refused only on a key its
	// own overlay answers.
	cases := []struct {
		name     string
		override map[string]string
		collides string
		overlay  string
	}{
		{"worktree onto apply", map[string]string{worktreeAction: "enter"}, applyAction, "the branch creator"},
		{"post-when-green onto edit", map[string]string{postWhenGreenAction: "e"}, editAction, "the announcement preview"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rebound, collides, overlay := testCase.override, testCase.collides, testCase.overlay

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, config.ErrKeyConflict) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConflict", rebound, err)
			}

			if got := err.Error(); !strings.Contains(got, collides) || !strings.Contains(got, overlay) {
				t.Errorf("CheckKeys error %q does not name %q in %s", got, collides, overlay)
			}
		})
	}
}

// Each overlay is a keyboard surface of its own: an action one answers and an
// action only another answers are never live at once, so one key may serve
// both. A running command answers no pane number, so its keys may take one.
func TestCheckKeysLetsTwoOverlaysShareAKey(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"remove-cache in Local data onto edit in the announcement preview": {"remove-cache": "e"},
		"post-when-green onto verbatim in the lefthook offer":              {postWhenGreenAction: "v"},
		"stop in a running command onto a pane number":                     {"stop": "1"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)
			// Assert
			if err != nil {
				t.Errorf("CheckKeys(%v) = %v, want the two overlays' actions free to share a key", rebound, err)
			}
		})
	}
}

func TestCheckKeysRefusesAKeyATextFieldEditsWithWhereOneHasTheFocus(t *testing.T) {
	t.Parallel()

	// Each action is answered by an overlay before its focused text field sees
	// the key, so on a key the field edits with it would take that edit away.
	cases := map[string]map[string]string{
		"worktree onto delete a word":            {worktreeAction: "ctrl+w"},
		"breaking onto back a character":         {"toggle-breaking": "ctrl+b"},
		"draft onto the line's start":            {toggleDraftAction: "ctrl+a"},
		"the next template onto next suggestion": {"next-template": "ctrl+n"},
		"edit body onto previous suggestion":     {editBodyAction: "ctrl+p"},
		"worktree onto a letter":                 {worktreeAction: "s"},
		"worktree onto the left arrow":           {worktreeAction: keyLeft},
		"draft onto the right arrow":             {toggleDraftAction: keyRight},
		"draft onto home":                        {toggleDraftAction: keyHome},
		"edit body onto end":                     {editBodyAction: keyEnd},
		"breaking onto the down arrow":           {"toggle-breaking": keyDown},
		"the next template onto the up arrow":    {"next-template": keyUp},
		"previous field onto back a word":        {"previous-field": "ctrl+left"},
		"next field onto delete a word forward":  {"next-field": "alt+d"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, config.ErrTextFieldKey) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrTextFieldKey", rebound, err)
			}

			for action := range rebound {
				if !strings.Contains(err.Error(), action) {
					t.Errorf("CheckKeys error %q does not name %s", err, action)
				}
			}
		})
	}
}

func TestCheckKeysAcceptsAnOverlayKeyOnAKeyOfThePaneBehindIt(t *testing.T) {
	t.Parallel()

	// The pane behind an open overlay does not answer while it is open, so an
	// overlay's key may share a key with that pane's own.
	cases := map[string]map[string]string{
		"verbatim onto switch-branch":         {"verbatim": "s"},
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

func TestRebindingAStartStopKeyOntoAnotherTasksKeyIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// s starts and stops a task; add-task moved onto it would make a press on
	// the Tasks pane ambiguous.
	colliding := map[string]string{addTaskAction: "s"}

	// Act
	err := tui.CheckKeys(colliding)

	// Assert
	if !errors.Is(err, config.ErrKeyConflict) {
		t.Fatalf("CheckKeys = %v, want ErrKeyConflict", err)
	}

	for _, named := range []string{"start-stop", addTaskAction, "the Tasks pane"} {
		if !strings.Contains(err.Error(), named) {
			t.Errorf("CheckKeys error %q does not name %q", err, named)
		}
	}
}

func TestRebindingATasksKeyOntoALinkOrRefreshKeyIsRefused(t *testing.T) {
	t.Parallel()

	// Opening and copying a link and refreshing are filed under Issues, yet the
	// Tasks pane answers them too.
	cases := map[string]struct {
		key, collides string
	}{
		"onto open":     {key: "o", collides: openLinkAction},
		"onto copy url": {key: "y", collides: "copy-link"},
		"onto refresh":  {key: "r", collides: refreshAction},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rebound := map[string]string{addTaskAction: tt.key}

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, config.ErrKeyConflict) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConflict", rebound, err)
			}

			for _, named := range []string{tt.collides, addTaskAction, "the Tasks pane"} {
				if !strings.Contains(err.Error(), named) {
					t.Errorf("CheckKeys error %q does not name %q", err, named)
				}
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
	if !errors.Is(err, config.ErrUnknownKeyAction) {
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
	if !errors.Is(err, config.ErrKeyNotRebindable) {
		t.Fatalf("CheckKeys = %v, want ErrKeyNotRebindable", err)
	}

	if got := err.Error(); !strings.Contains(got, "jump-to-pane") {
		t.Errorf("CheckKeys error %q does not name the action", got)
	}
}

func TestCheckKeysRefusesMovingInterruptOntoAKeyThatTypesOrEdits(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a letter": "x", "a digit": "5", "a space": "space", "a symbol": "/",
		"a letter with an accent": "é", "an emoji with its variation selector": "\u2764\ufe0f",
		"backspace, which edits text": "backspace", "delete, which edits text": "delete",
	}

	for name, typed := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			// Interrupt is answered before any filter, prompt or text box, so on a
			// key that types it would quit mid-sentence and lose what was written.
			err := tui.CheckKeys(map[string]string{"interrupt": typed})

			// Assert
			if !errors.Is(err, config.ErrInterruptEdits) {
				t.Fatalf("CheckKeys = %v, want ErrInterruptEdits", err)
			}

			if got := err.Error(); !strings.Contains(got, "interrupt") {
				t.Errorf("CheckKeys error %q does not name the action", got)
			}
		})
	}
}

func TestCheckKeysAcceptsInterruptOnAKeyThatTypesNothing(t *testing.T) {
	t.Parallel()

	// Act
	err := tui.CheckKeys(map[string]string{"interrupt": "ctrl+q"})
	// Assert
	if err != nil {
		t.Errorf("CheckKeys = %v, want a control key accepted for interrupt", err)
	}
}
