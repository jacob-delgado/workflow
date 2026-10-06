// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// The marks an issue can be in, in the order the picker lists them, as the web
// words them.
//
// Trade-off TRADE-21: these words and the place rules below are written again in
// web/src/features/issues/issuePlaces.ts.
const (
	markInFlight   = "in flight"
	markTaskActive = "task active"
	markTracked    = "tracked"
	markTaskDone   = "task done"
	markForge      = "forge issue"
)

// placeKind is which of the two groups a place is in. Places in one group
// widen the list, and the two groups narrow it together.
type placeKind int

const (
	placeStatus placeKind = iota
	placeMark
)

// place is where an issue can be: one of its tracker's status names, or one of
// workflow's own marks.
type place struct {
	kind placeKind
	name string
}

// admits reports whether an issue in status, with marks, is in the picked
// places: in any picked status, when a status is picked, and holding any picked
// mark, when a mark is picked.
func admits(picked []place, status string, marks []string) bool {
	return admitsIn(picked, placeStatus, []string{status}) && admitsIn(picked, placeMark, marks)
}

// admitsIn reports whether any of names is picked in the kind's group, or
// nothing is picked there at all.
func admitsIn(picked []place, kind placeKind, names []string) bool {
	constrained := false

	for _, chosen := range picked {
		if chosen.kind != kind {
			continue
		}

		constrained = true

		if slices.Contains(names, chosen.name) {
			return true
		}
	}

	return !constrained
}

// placeChoices is every place the loaded issues are in, with how many are in
// each — statuses by category, not started first, then as they first appear,
// then the marks in their fixed order — and every picked place no loaded issue
// is in, at zero, so it can still be unpicked.
func placeChoices(issues []jira.Issue, marksOf func(jira.Key) []string, picked []place) []offered[place] {
	counts := map[place]int{}

	for _, issue := range issues {
		counts[place{kind: placeStatus, name: issue.Status}]++

		for _, mark := range marksOf(issue.Key) {
			counts[place{kind: placeMark, name: mark}]++
		}
	}

	var choices []offered[place]

	for _, offering := range slices.Concat(statusPlaces(issues), pickedStatusesGone(issues, picked), markPlaces()) {
		if counts[offering] > 0 || slices.Contains(picked, offering) {
			choices = append(choices, offered[place]{value: offering, count: counts[offering]})
		}
	}

	return choices
}

// statusPlaces is each status the issues are in, once, not started first, then
// in flight, then done, then any in a category Jira does not name (its "No
// Category", or none), and within each in the order they first appear.
func statusPlaces(issues []jira.Issue) []place {
	var places []place

	add := func(keep func(jira.StatusCategory) bool) {
		for _, issue := range issues {
			status := place{kind: placeStatus, name: issue.Status}
			if keep(issue.StatusCategory) && !slices.Contains(places, status) {
				places = append(places, status)
			}
		}
	}

	named := []jira.StatusCategory{jira.CategoryNew, jira.CategoryIndeterminate, jira.CategoryDone}
	for _, category := range named {
		add(func(of jira.StatusCategory) bool { return of == category })
	}

	add(func(of jira.StatusCategory) bool { return !slices.Contains(named, of) })

	return places
}

// pickedStatusesGone is each picked status no loaded issue is in any more, in
// the order it was picked, so the picker can still offer to unpick it.
func pickedStatusesGone(issues []jira.Issue, picked []place) []place {
	var gone []place

	for _, chosen := range picked {
		if chosen.kind == placeStatus && !slices.ContainsFunc(issues, func(issue jira.Issue) bool {
			return issue.Status == chosen.name
		}) {
			gone = append(gone, chosen)
		}
	}

	return gone
}

// markPlaces is every mark, in the order the picker lists them.
func markPlaces() []place {
	marks := []string{markInFlight, markTaskActive, markTracked, markTaskDone, markForge}
	places := make([]place, 0, len(marks))

	for _, mark := range marks {
		places = append(places, place{kind: placeMark, name: mark})
	}

	return places
}

// issueBranchesListed carries which issues a local or remote branch names back
// into the update loop. A listing git could not make leaves in flight unknown
// rather than failing the list.
type issueBranchesListed struct {
	keys map[jira.Key]bool
	err  error
}

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
func (m Model) listIssueBranches() tea.Cmd {
	lister := branchLister{
		local: m.deps.Git.Branches, remote: m.deps.Git.RemoteBranches, links: m.deps.Git.IssueLinks,
		project: m.cfg.Jira.Project,
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
			words[issueKey] = m.taskWord(issueKey)
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
	var marks []string

	if l.branchesKnown && l.branchKeys[issueKey] {
		marks = append(marks, markInFlight)
	}

	if word := l.taskWords[issueKey]; word != "" {
		marks = append(marks, word)
	}

	if isForgeKey(issueKey) {
		marks = append(marks, markForge)
	}

	return marks
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
	_ overlay   = checklist[place]{}
	_ clickable = checklist[place]{}
	_ steppable = checklist[place]{}
)

// openPlacePicker opens the checklist on the places the loaded issues are in,
// with those already picked checked. Applying it keeps the selection on the
// issue it was on while that issue is still listed.
func (m Model) openPlacePicker() (Model, tea.Cmd) {
	m.overlay = checklist[place]{
		marks: m.marks, title: filterTitle, none: "no issue to filter",
		choices: pickList[offered[place]]{items: placeChoices(m.issues.found.Issues, m.issues.marksOf, m.issues.places)},
		chosen:  slices.Clone(m.issues.places),
		label:   func(picked place) string { return picked.name },
		apply: func(m Model, chosen []place) (Model, tea.Cmd) {
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
			names = append(names, picked.name)
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
func (l issueList) inFlightColumn(marks glyphs, sty styles, issueKey jira.Key) string {
	switch {
	case !l.branchesKnown || !l.anyInFlight():
		return ""
	case l.branchKeys[issueKey]:
		return sty.git.Render(marks.inFlight)
	default:
		return marks.unknown
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
