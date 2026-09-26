// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// configurationPage is the page that tells a reader which actions ui.keys can
// rebind.
const configurationPage = "../../docs/content/docs/configuration.md"

// The sentence that opens the page's action list, and the one after it.
const (
	actionListOpens = "The actions you can rebind, grouped by where they work, are:"
	actionListEnds  = "A key is named"
)

func TestTheConfigurationPageListsEveryRebindableActionAndNoOther(t *testing.T) {
	t.Parallel()

	// Arrange
	rebindable := rebindableActions()

	// Act
	documented := documentedActions(t)

	// Assert
	if missing := without(rebindable, documented); len(missing) > 0 {
		t.Errorf("%s leaves out the rebindable actions %q", configurationPage, missing)
	}

	if unbound := without(documented, rebindable); len(unbound) > 0 {
		t.Errorf("%s lists %q, which ui.keys cannot rebind", configurationPage, unbound)
	}
}

// rebindableActions is every placed action ui.keys can move, once each.
func rebindableActions() []string {
	var actions []string

	for _, group := range placedBindings() {
		for _, binding := range movable(group.bindings) {
			actions = append(actions, binding.action)
		}
	}

	return actions
}

// documentedActions reads the configuration page's action list: every
// backticked name between the sentence that opens it and the one after it.
// Whitespace is folded first, so rewrapping the page moves nothing.
func documentedActions(t *testing.T) []string {
	t.Helper()

	contents, err := os.ReadFile(configurationPage)
	if err != nil {
		t.Fatalf("read %s: %v", configurationPage, err)
	}

	page := strings.Join(strings.Fields(string(contents)), " ")

	_, list, opened := strings.Cut(page, actionListOpens)
	list, _, ended := strings.Cut(list, actionListEnds)

	if !opened || !ended {
		t.Fatalf("%s has no action list between %q and %q", configurationPage, actionListOpens, actionListEnds)
	}

	var actions []string

	for index, part := range strings.Split(list, "`") {
		if index%2 == 1 {
			actions = append(actions, part)
		}
	}

	return actions
}

// without is the names in from that are not in these, once each and sorted.
func without(from, these []string) []string {
	var left []string

	for _, name := range from {
		if !slices.Contains(these, name) && !slices.Contains(left, name) {
			left = append(left, name)
		}
	}

	slices.Sort(left)

	return left
}
