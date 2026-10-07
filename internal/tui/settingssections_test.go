// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The rows of the sections Settings carried unseen until it could edit them,
// for a configuration with no views, headers or prefixes.
const (
	addViewRow        = projectRow + 2
	addHeaderRow      = projectRow + 3
	channelsRow       = messagingKindRow + 6
	addPrefixRow      = branchTemplateRow + 2
	requestTimeoutRow = storeDisabledRow + 3
	asciiRow          = storeDisabledRow + 5
	keysRow           = storeDisabledRow + 11
)

// removeKey removes the entry the cursor is on.
const removeKey = "D"

// added opens Settings, adds an entry on row, named name, of value, and
// saves.
func added(row int, name, value string) []string {
	keys := append(toRow(row), keyEnter)
	keys = append(append(keys, letters(name)...), keyEnter)

	return append(append(keys, letters(value)...), keyEnter, saveKey)
}

// commentKeyRow is the row of comment in the key table.
func commentKeyRow(t *testing.T) int {
	t.Helper()

	listed := slices.DeleteFunc(tui.KeyActions("pull request", "Slack", nil), func(listed seams.KeyAction) bool {
		return listed.Action == "jump-to-pane"
	})

	at := slices.IndexFunc(listed, func(listed seams.KeyAction) bool { return listed.Action == commentAction })
	if at < 0 {
		t.Fatalf("no comment in the key table")
	}

	return keysRow + at
}

func TestAViewAddedInSettingsIsSavedWithItsJQL(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), added(addViewRow, "Review", "status = Review")...)

	// Assert
	want := []config.JiraView{{Name: "Review", JQL: "status = Review"}}
	if got := repo.settings.Jira.Views; !slices.Equal(got, want) {
		t.Errorf("saved jira.views %v, want %v", got, want)
	}
}

func TestAViewRemovedInSettingsIsLeftOutOfTheSave(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Views = []config.JiraView{{Name: "Mine", JQL: "assignee = currentUser()"}, {Name: "Team", JQL: "x"}}

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(addViewRow), removeKey, saveKey)...)

	// Assert
	if got := repo.settings.Jira.Views; !slices.Equal(got, []config.JiraView{{Name: "Team", JQL: "x"}}) {
		t.Errorf("saved jira.views %v, want Team alone", got)
	}
}

func TestTheViewsInSettingsAreShownByNameAndJQL(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Views = []config.JiraView{{Name: "Mine", JQL: "assignee = currentUser()"}}

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, settingsKey).View().Content

	// Assert
	requireScreen(t, view, "View Mine", "assignee = currentUser()", "Add a view")
}

func TestChannelsAddedInSettingsAreSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(channelsRow, "#ops, #releases")...)

	// Assert
	if got := repo.settings.Messaging.Channels; !slices.Equal(got, []string{"#ops", "#releases"}) {
		t.Errorf("saved messaging.channels %v, want #ops and #releases", got)
	}
}

func TestABranchPrefixAddedInSettingsIsSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), added(addPrefixRow, "Spike", "research")...)

	// Assert
	if got := repo.settings.Branch.Prefixes; !maps.Equal(got, map[string]string{"Spike": "research"}) {
		t.Errorf("saved branch.prefixes %v, want Spike as research", got)
	}
}

func TestAHeaderAddedInSettingsIsSavedAndNeverShown(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), added(addHeaderRow, "X-Proxy-Auth", typedSecret)...).View().Content

	// Assert
	if got := repo.settings.Jira.Headers["X-Proxy-Auth"]; got.Reveal() != typedSecret {
		t.Errorf("saved the header %t, want it saved", got.Reveal() == typedSecret)
	}

	refuseScreen(t, view, typedSecret)
}

func TestTheTimingIsSavedAsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(requestTimeoutRow, "45s")...)

	// Assert
	if got := repo.settings.Timing.RequestTimeout; got != "45s" {
		t.Errorf("saved timing.request_timeout %q, want 45s", got)
	}
}

func TestDrawingInASCIIIsTurnedOnInSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(asciiRow), keyEnter, saveKey)...)

	// Assert
	if !repo.settings.UI.ASCII {
		t.Errorf("saved ui.ascii off, want it on")
	}
}

func TestAKeyMovedInSettingsIsSavedInUIKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(commentKeyRow(t), "C")...)

	// Assert
	if got := repo.settings.UI.Keys; !maps.Equal(got, map[string]string{"comment": "C"}) {
		t.Errorf("saved ui.keys %v, want comment on C", got)
	}
}

func TestAKeyThatClashesIsRefusedInTheForm(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(commentKeyRow(t), "t")...).View().Content

	// Assert
	requireScreen(t, view, "would not start on this keymap")

	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times, want nothing saved", len(saves))
	}
}

func TestARefusedSaveNeverShowsATypedCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append(edited(tokenRow, typedSecret), slices.Repeat([]string{"j"}, requestTimeoutRow-tokenRow)...)
	keys = append(append(keys, keyEnter), letters("soon")...)

	// Act
	view := typing(t, newWorld().live(t, 120, 40), append(keys, keyEnter, saveKey)...).View().Content

	// Assert
	requireScreen(t, view, "not valid")
	refuseScreen(t, view, typedSecret, settingsToken)
}

func TestAKeymapRefusalSaysWhichActionsClash(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 80, 24), editing(commentKeyRow(t), "t")...).View().Content

	// Assert
	requireScreen(t, view, "change-status and comment both bind")
}

func TestKeysLeftOnTheirDefaultsAreNeitherEditedNorRemovable(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), toRow(commentKeyRow(t))...).View().Content

	// Assert
	refuseScreen(t, view, "(edited)")
	refuseScreen(t, footerLine(view), "D remove")
}
