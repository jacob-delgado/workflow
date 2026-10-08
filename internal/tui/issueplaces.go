// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/places"
)

// issueBranchesListed carries which issues a local or remote branch names back
// into the update loop. A listing git could not make leaves in flight unknown
// rather than failing the list.
type issueBranchesListed struct {
	keys map[jira.Key]bool
	err  error
}

var _ applier = issueBranchesListed{}

// apply records which issues have a branch, and reads the selected issue when
// the change moved the selection. A listing that failed changes nothing: the
// branches last listed still stand, rather than every issue dropping out of
// flight at once.
func (msg issueBranchesListed) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}

	m.issues = m.issues.keepingSelection(func(l issueList) issueList {
		l.branchKeys, l.branchesKnown = msg.keys, true

		return l
	})

	return m.loadDetail()
}

// listIssueBranches is the command that lists the local and remote branches and
// reads the issue each names; nil with no git to ask.
func listIssueBranches(cfg config.Config, deps Deps) tea.Cmd {
	lister := branchLister{
		local: deps.Git.Branches, remote: deps.Git.RemoteBranches, links: deps.Git.IssueLinks,
		project: cfg.Jira.Project,
	}
	if lister.local == nil {
		return nil
	}

	return func() tea.Msg {
		listed := lister.list()
		if listed.err != nil {
			return issueBranchesListed{err: listed.err}
		}

		keys := map[jira.Key]bool{}

		for _, name := range slices.Concat(listed.local, listed.remote) {
			if issueKey, named := loop.NamedIssue(name, listed.links, lister.project); named {
				keys[jira.Key(issueKey.Key)] = true
			}
		}

		return issueBranchesListed{keys: keys}
	}
}

// withTaskWords is the model with the list told how each issue's tasks stand,
// once Taskwarrior has answered, so the places filter by what the rows show.
func (m Model) withTaskWords() Model {
	var words map[jira.Key]string

	if m.tasks.answered() {
		words = map[jira.Key]string{}

		for _, task := range m.tasks.linked {
			issueKey := jira.Key(task.IssueKey)
			words[issueKey] = m.tasks.issueWord(m.kit(), issueKey)
		}
	}

	m.issues = m.issues.keepingSelection(func(l issueList) issueList {
		l.taskWords = words

		return l
	})

	return m
}

// marksOf is the marks an issue is in: in flight when a branch names it, and
// how its tasks stand when one is linked.
func (l issueList) marksOf(issueKey jira.Key) []string {
	return places.Standing{
		InFlight: l.branchesKnown && l.branchKeys[issueKey], Task: l.taskWords[issueKey], Forge: isForgeKey(issueKey),
	}.Marks()
}

// keepingSelection is the list after change, with the selection held on the
// issue it was on, or in range when that issue is no longer admitted.
func (l issueList) keepingSelection(change func(issueList) issueList) issueList {
	selected, ok := l.current()
	l = change(l)

	if ok {
		return l.selectKey(selected.Key)
	}

	return l.clampSelection()
}

var (
	_ overlay   = checklist[places.Place]{}
	_ clickable = checklist[places.Place]{}
	_ steppable = checklist[places.Place]{}
)

// openPlacePicker opens the checklist on the places the loaded issues are in,
// with those already picked checked. Applying it keeps the selection on the
// issue it was on while that issue is still listed.
func (m Model) openPlacePicker() (Model, tea.Cmd) {
	choices := places.Choices(m.issues.found.Issues, m.issues.marksOf, m.issues.places)
	offers := make([]offered[places.Place], 0, len(choices))

	for _, choice := range choices {
		offers = append(offers, offered[places.Place]{value: choice.Place, count: choice.Count})
	}

	m.overlay = checklist[places.Place]{
		title: filterTitle, none: "no issue to filter",
		choices: pickList[offered[places.Place]]{items: offers},
		chosen:  slices.Clone(m.issues.places),
		label:   func(picked places.Place) string { return picked.Name },
		apply: func(m Model, chosen []places.Place) (Model, tea.Cmd) {
			m.issues = m.issues.keepingSelection(func(l issueList) issueList {
				l.places = chosen

				return l
			})

			return m.loadDetail()
		},
	}

	return m, nil
}

// narrowingLine says what narrows the list, for the notice row: the picked
// places, then the filter while it is typed or applied.
func (l issueList) narrowingLine(marks glyphs) string {
	var parts []string

	if len(l.places) > 0 {
		names := make([]string, 0, len(l.places))
		for _, picked := range l.places {
			names = append(names, picked.Name)
		}

		parts = append(parts, "places: "+strings.Join(names, ", "))
	}

	if l.filtering || l.filter != "" {
		parts = append(parts, "search: "+l.filter)
	}

	return strings.Join(parts, marks.separator)
}

// inFlightColumn is a row's in-flight mark, in git's hue when a branch names the
// issue. There is no column until the branches could be listed, since a column
// of "no branch" would claim what is not known, nor while no listed issue has a
// branch, when it would say nothing.
func (l issueList) inFlightColumn(kit renderKit, issueKey jira.Key) string {
	switch {
	case !l.branchesKnown || !l.anyInFlight():
		return ""
	case l.branchKeys[issueKey]:
		return kit.styles.git.Render(kit.marks.inFlight)
	default:
		return kit.marks.unknown
	}
}

// anyInFlight reports that a branch names at least one listed issue.
func (l issueList) anyInFlight() bool {
	return slices.ContainsFunc(l.found.Issues, func(issue jira.Issue) bool { return l.branchKeys[issue.Key] })
}

// nothingAdmitted says what emptied the list — the filter, the places, or both —
// in few enough words to fit the rail.
func (l issueList) nothingAdmitted() string {
	switch {
	case len(l.places) == 0:
		return "no issue matches the filters"
	case l.filter == "":
		return "no issue in those places"
	default:
		return "no match for filter and places"
	}
}
