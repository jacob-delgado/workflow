// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"cmp"
	"slices"
	"strings"
	"testing"
)

// Each pane is a key context of its own, so a ui.keys map may give two panes
// the same key; ? lists each pane's keys under the pane's own heading, so the
// key shared reads as one key on each pane rather than two on one.

// sharedKey is the key each case moves a second pane's action onto.
const sharedKey = "P"

func TestAKeyTwoPanesShareIsListedOnceUnderEachPanesHeading(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		overrides map[string]string
		// want is, by heading, the one line ? lists on the shared key there.
		want map[string]string
	}{
		"commit on the Branch pane's push key": {
			overrides: map[string]string{commitAction: sharedKey},
			want:      map[string]string{branchGroup: "P push", commitsGroup: "P commit"},
		},
		"merge on the Slack pane's people and groups key": {
			overrides: map[string]string{mergeAction: sharedKey},
			want:      map[string]string{reviewGroup: "P merge", messagingGroup: "P people and groups"},
		},
	}

	for name, sharing := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			listing := helpListing(t, openHelp(t, sharing.overrides))

			// Assert
			for heading, line := range sharing.want {
				var onKey []string

				for _, binding := range groupNamed(listing, heading).bindings {
					if binding.key == sharedKey {
						onKey = append(onKey, binding.key+" "+binding.help)
					}
				}

				if len(onKey) != 1 || onKey[0] != line {
					t.Errorf("? lists %q on %s under %s, want %q alone", onKey, sharedKey, heading, line)
				}
			}
		})
	}
}

// documentedGroups is the configuration page's action list by group: each
// item's bold name, and the actions it names, sorted.
func documentedGroups(t *testing.T) map[string][]string {
	t.Helper()

	groups := map[string][]string{}

	for _, item := range strings.Split(actionList(t), "- **")[1:] {
		name, actions, _ := strings.Cut(item, "**")
		groups[strings.TrimSuffix(name, ":")] = slices.Sorted(slices.Values(backticked(actions)))
	}

	return groups
}

func TestTheConfigurationPageListsEachPanesActionsUnderThePane(t *testing.T) {
	t.Parallel()

	// Arrange
	// The page names the messaging pane's group for no particular service.
	headings := map[string]string{messagingGroup: "Your messaging service"}

	// Act
	documented := documentedGroups(t)

	// Assert
	for _, group := range placedBindings() {
		bindings := movable(group.bindings)

		want := make([]string, 0, len(bindings))
		for _, binding := range bindings {
			want = append(want, binding.action)
		}

		heading := cmp.Or(headings[group.name], group.name)
		if got := documented[heading]; !slices.Equal(got, slices.Sorted(slices.Values(want))) {
			t.Errorf("%s lists %q under %q, want %q, as ? groups them", configurationPage, got, heading, want)
		}
	}
}
