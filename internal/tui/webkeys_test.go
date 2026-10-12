// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// boundActionsFile is the web page's own list of the actions it has a
// control for.
const boundActionsFile = "../../web/src/features/keyboard/boundActions.ts"

// mockKeysFile is the key list the web page's mockup serves in place of the
// server, pane sections and all.
const mockKeysFile = "../../web/src/dev/mockKeys.ts"

// The web page shows the Commits pane inside its Branch section, but it has
// a control only for the actions it binds: an action only the terminal
// answers is never pressed there, so it may share a key with the other
// pane's.
func TestCheckKeysLetsATerminalOnlyActionShareAKeyInASectionTheWebShowsTogether(t *testing.T) {
	t.Parallel()

	// Arrange
	// P is the Branch pane's push, which the page binds; the page has no
	// control for discard-change, the Commits pane's x.
	rebound := map[string]string{"discard-change": "P"}

	// Act
	err := tui.CheckKeys(rebound)
	// Assert
	if err != nil {
		t.Errorf("CheckKeys(%v) = %v, want discard-change, which the page does not bind, free to share push's key",
			rebound, err)
	}
}

func TestWebBoundActionsAreThePagesOwnList(t *testing.T) {
	t.Parallel()

	// Arrange
	page := pageBoundActions(t)

	// Act
	server := slices.Sorted(slices.Values(tui.WebBoundActions()))

	// Assert
	if !slices.Equal(server, page) {
		t.Errorf("WebBoundActions = %v, want the page's boundActions.ts, %v", server, page)
	}
}

func TestEveryWebBoundActionIsOneTheKeymapNames(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := tui.KeyActions("pull request", "Slack", nil)

	// Act
	bound := tui.WebBoundActions()

	// Assert
	if len(bound) == 0 {
		t.Fatal("WebBoundActions lists no action")
	}

	for _, action := range bound {
		if !slices.ContainsFunc(listed, func(named seams.KeyAction) bool { return named.Action == action }) {
			t.Errorf("WebBoundActions names %q, which the keymap does not define", action)
		}
	}
}

func TestTheMockupShowsEachPaneInTheSectionTheServerDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	mockup := quotedBetween(t, mockKeysFile, "pane_sections: [", "]")

	// Act
	sections := tui.PaneSections()

	// Assert
	server := make([]string, 0, len(sections))
	for _, section := range sections {
		server = append(server, string(section))
	}

	if !slices.Equal(server, mockup) {
		t.Errorf("PaneSections = %v, want the mockup's pane_sections in mockKeys.ts, %v", server, mockup)
	}
}

// pageBoundActions is the names boundActions.ts puts in its set, sorted.
func pageBoundActions(t *testing.T) []string {
	t.Helper()

	return slices.Sorted(slices.Values(quotedBetween(t, boundActionsFile, "new Set([", "])")))
}

// quotedBetween is every single-quoted string in a file between the first
// opens and the closes after it, in the order the file holds them.
func quotedBetween(t *testing.T, file, opens, closes string) []string {
	t.Helper()

	contents, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}

	_, held, found := strings.Cut(string(contents), opens)
	held, _, closed := strings.Cut(held, closes)

	if !found || !closed {
		t.Fatalf("%s holds nothing between %q and %q", file, opens, closes)
	}

	quoted := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(held, -1)

	names := make([]string, 0, len(quoted))
	for _, name := range quoted {
		names = append(names, name[1])
	}

	return names
}
