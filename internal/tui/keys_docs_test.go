// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// configurationPage is the page that tells a reader which actions ui.keys can
// rebind.
const configurationPage = "../../docs/content/docs/configuration.md"

// usagePage is the page whose key table holds every key ? lists, by where it
// works.
const usagePage = "../../docs/content/docs/usage.md"

// keyTableHeader is the key table's header row.
const keyTableHeader = "| Where | Key | Does |"

// reviewsGroup is the help's group for the Reviews pane.
const reviewsGroup = "Reviews"

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

func TestTheUsagePagesKeyTableHoldsEveryKeyTheHelpLists(t *testing.T) {
	t.Parallel()

	for _, group := range placedBindings() {
		t.Run(group.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			sections := usageSectionsOf(group.name)

			// Act
			documented := documentedKeys(t, sections)

			// Assert
			for _, binding := range group.listedActions() {
				for _, key := range keyNames(binding.key) {
					if !slices.Contains(documented, key) {
						t.Errorf("%s's key table has no `%s` under %q for %s (%s)",
							usagePage, key, sections, binding.action, binding.help)
					}
				}
			}
		})
	}
}

// usageSectionsOf is the sections of usage.md's key table that hold a help
// group's keys: a pane's group under its pane's numbered section, any other
// group under its own name.
func usageSectionsOf(group string) []string {
	numbered := map[string][]string{
		issuesGroup:                {"1 Issues"},
		"Branch and Commits":       {"2 Branch", "3 Commits"},
		"Review and Slack":         {"4 Review", "5, your service"},
		reviewsGroup:               {"6 Reviews"},
		tasksTitle:                 {"7 Tasks"},
		summaryTitle:               {"8 Summary"},
		reposTitle:                 {"9 Repositories"},
		"In a composer or preview": {"A composer or preview"},
	}

	sections, isNumbered := numbered[group]
	if !isNumbered {
		return []string{group}
	}

	return sections
}

// documentedKeys is every backticked key in the Key column of usage.md's key
// table, under any of the sections named. A row with an empty Where belongs to
// the section above it.
func documentedKeys(t *testing.T, sections []string) []string {
	t.Helper()

	contents, err := os.ReadFile(usagePage)
	if err != nil {
		t.Fatalf("read %s: %v", usagePage, err)
	}

	_, table, found := strings.Cut(string(contents), keyTableHeader)
	if !found {
		t.Fatalf("%s has no key table headed %q", usagePage, keyTableHeader)
	}

	var (
		keys  []string
		where string
	)

	for line := range strings.Lines(strings.TrimLeft(table, "\n")) {
		cells := strings.Split(line, "|")
		if !strings.HasPrefix(line, "|") || len(cells) < 4 {
			break
		}

		if section := strings.TrimSpace(cells[1]); section != "" {
			where = section
		}

		if slices.Contains(sections, where) {
			keys = append(keys, backticked(cells[2])...)
		}
	}

	return keys
}

// backticked is every backticked name in text.
func backticked(text string) []string {
	var names []string

	for index, part := range strings.Split(text, "`") {
		if index%2 == 1 {
			names = append(names, part)
		}
	}

	return names
}

// keyNames is each key a binding's shown key names: `↑/k` is ↑ and k, and the
// pane numbers' `1-9` are 1 and 9, as the table writes them `1`–`9`.
func keyNames(shown string) []string {
	if utf8.RuneCountInString(shown) == 1 {
		return []string{shown}
	}

	var names []string

	for part := range strings.SplitSeq(shown, "/") {
		first, last, isRange := strings.Cut(part, "-")
		if isRange && first != "" && last != "" {
			names = append(names, first, last)

			continue
		}

		names = append(names, part)
	}

	return names
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

	return backticked(list)
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
