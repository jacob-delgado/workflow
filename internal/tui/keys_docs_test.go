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

// usagePage is the page whose key table holds every key ? lists, each on its
// own action's row, by where it works.
const usagePage = "../../docs/content/docs/usage.md"

// keyTableHeader is the key table's header row.
const keyTableHeader = "| Where | Key | Does |"

// reviewsGroup is the help's group for the Reviews pane.
const reviewsGroup = "Reviews"

// The help's groups for the Branch, Commits, Review and messaging panes, each
// named for its pane, the messaging pane's for Slack.
const (
	branchGroup    = "Branch"
	commitsGroup   = "Commits"
	reviewGroup    = "Review"
	messagingGroup = "Slack"
)

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

func TestTheUsagePagesKeyTablePutsEachKeyOnItsActionsRow(t *testing.T) {
	t.Parallel()

	for _, group := range placedBindings() {
		t.Run(group.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			sections := usageSectionsOf(group.name)
			openings := usageRowOpenings()

			// Act
			rows := usageRows(t, sections)

			// Assert
			for _, binding := range group.listedActions() {
				opening, named := openings[binding.action]
				if !named {
					t.Errorf("no row of %s's key table is named for %s (%s): add the opening of its Does cell "+
						"to usageRowOpenings", usagePage, binding.action, binding.help)

					continue
				}

				matched := rowsOpening(rows, opening)
				if len(matched) != 1 {
					t.Errorf("%d rows under %q open %q, the row named for %s; want one", len(matched), sections, opening,
						binding.action)

					continue
				}

				for _, key := range keyNames(binding.key) {
					if !slices.Contains(matched[0].keys, key) {
						t.Errorf("%s's row %q under %q has no `%s` for %s (%s)",
							usagePage, matched[0].does, matched[0].where, key, binding.action, binding.help)
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
		branchGroup:                {"2 Branch"},
		commitsGroup:               {"3 Commits"},
		reviewGroup:                {"4 Review"},
		messagingGroup:             {"5, your service"},
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

// usageRow is one row of usage.md's key table: the section it is under, the
// keys its Key column names, and what its Does column says.
type usageRow struct {
	where string
	keys  []string
	does  string
}

// usageRows is every row of usage.md's key table under any of the sections
// named. A row with an empty Where belongs to the section above it.
func usageRows(t *testing.T, sections []string) []usageRow {
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
		rows  []usageRow
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
			rows = append(rows, usageRow{where: where, keys: backticked(cells[2]), does: strings.TrimSpace(cells[3])})
		}
	}

	return rows
}

// rowsOpening is the rows whose Does column is opening, or, when none is,
// the rows whose Does column opens with it.
func rowsOpening(rows []usageRow, opening string) []usageRow {
	exact := slices.DeleteFunc(slices.Clone(rows), func(row usageRow) bool { return row.does != opening })
	if len(exact) > 0 {
		return exact
	}

	return slices.DeleteFunc(slices.Clone(rows), func(row usageRow) bool { return !strings.HasPrefix(row.does, opening) })
}

// usageRowOpenings names each listed action's row in usage.md's key table by
// how its Does column opens, so a key is held to its own action's row rather
// than to anywhere in the section. Each line is an action, then the opening.
func usageRowOpenings() map[string]string {
	openings := map[string]string{}

	for line := range strings.Lines(`
next-pane           Next pane, previous pane
previous-pane       Next pane, previous pane
jump-to-pane        Jump to a pane
up                  Move within a list
down                Move within a list
first               Jump to the first or last row
last                Jump to the first or last row
scroll-up           Scroll the detail pane
scroll-down         Scroll the detail pane

change-status       Change the selected issue's status
comment             Comment on it
assign              Assign it
log-work            Log work on it
start-work          Start work on it
search-issues       Search the list
filter-issues       Filter the list
switch-view         Switch which issue list
load-more           Load the next page
open-link           Open the issue in the browser
copy-link           Open the issue in the browser
refresh             Search again
track-issue         Track the issue in Taskwarrior

new-branch          Start a branch
switch-branch       Switch branch
link-issue          Link the branch to an issue
rebase              Rebase the branch
push                Push a branch
stage               Stage or unstage
stage-all           Stage every file
unstage-all         Unstage every file
discard-change      Discard the selected
commit              Commit what is staged
amend               Amend the last unpushed commit
fixup               Record what is staged as a
run-pre-commit      Run the pre-commit hook
set-up-lefthook     Set up lefthook

open-pull-request   Open a pull or merge request
checks              List its CI checks
rerun-checks        Re-run failed CI
merge               Merge a green
finish-branch       Finish a merged branch
post                Preview the announcement
people-and-groups   People and groups

sort-reviews        Sort them oldest first
filter-reviews      Filter them by repository

start-stop          Start the selected task
mark-done           Mark it done
add-task            Add a task
annotate-task       Annotate it
modify-task         Modify it
undo-task           Undo Taskwarrior
sync-tasks          Sync Taskwarrior
search-tasks        Search them as you type
filter-tasks        Filter them by state
sort-tasks          Sort them by urgency

earlier             The period before or after
later               The period before or after
today               Today
calendar            Pick a day
copy-summary        Copy the summary as Markdown
post-summary        Post the summary

favorite-directory  Add the selected directory to your favorites
go-to-directory     Type a directory to switch to
settings            Settings:
local-data          Local data:

edit                Edit an announcement
edit-body           Write the commit's body
next-template       Use the repository's next pull request template
toggle-draft        Open the pull request as a draft
toggle-breaking     Mark the commit a breaking change
verbatim            Keep every existing hook whole
show-log            In the checks list
next-field          Next field, previous field
previous-field      Next field, previous field
cycle-type-left     Change the commit's type
toggle-option       Pick an option
worktree            In the branch creator
post-when-green     In the announcement preview, announce once CI passes
unlink-issue        In the link form
remove-cache        In Local data, remove the cache
remove-everything   In Local data, remove the cache
save-settings       In Settings, save
remove-entry        In Settings, remove
link-to-slack       In the announcement preview, link
not-on-slack        In the announcement preview, remember
forget-owner        In People and groups, forget

insert              Type before or after the cursor
append              Type before or after the cursor
append-line         Type at the end of the line
open-line           Type at the end of the line
cursor-left         Move the cursor
cursor-right        Move the cursor

stop                Stop it
run-again           Run it again
full-output         Show its full output

apply               Do what the bottom row names
close               Leave without doing it
toggle-mouse        Turn mouse capture off or on
toggle-help         Every key
quit                Quit
interrupt           Quit from anywhere
`) {
		action, opening, found := strings.Cut(strings.TrimSpace(line), " ")
		if found {
			openings[action] = strings.TrimSpace(opening)
		}
	}

	return openings
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
// backticked name in it.
func documentedActions(t *testing.T) []string {
	t.Helper()

	return backticked(actionList(t))
}

// actionList is the configuration page's action list: the text between the
// sentence that opens it and the one after it. Whitespace is folded first, so
// rewrapping the page moves nothing.
func actionList(t *testing.T) string {
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

	return list
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
