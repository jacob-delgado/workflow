// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// placeTitle titles the detail pane while the place picker is open.
const placeTitle = "Where"

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

// placeChoice is a place the picker offers, with how many loaded issues are in
// it.
type placeChoice struct {
	place place
	count int
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
func placeChoices(issues []jira.Issue, marksOf func(jira.Key) []string, picked []place) []placeChoice {
	counts := map[place]int{}

	for _, issue := range issues {
		counts[place{kind: placeStatus, name: issue.Status}]++

		for _, mark := range marksOf(issue.Key) {
			counts[place{kind: placeMark, name: mark}]++
		}
	}

	var choices []placeChoice

	for _, offered := range slices.Concat(statusPlaces(issues), pickedStatusesGone(issues, picked), markPlaces()) {
		if counts[offered] > 0 || slices.Contains(picked, offered) {
			choices = append(choices, placeChoice{place: offered, count: counts[offered]})
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
	marks := []string{markInFlight, markTaskActive, markTracked, markTaskDone}
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
	lister := branchLister{local: m.deps.Git.Branches, remote: m.deps.Git.RemoteBranches, project: m.cfg.Jira.Project}
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
			if issueKey, named := convention.IssueKey(name, lister.project); named {
				keys[jira.Key(issueKey)] = true
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

// placePicker is the checklist of places to narrow the Issues list to.
type placePicker struct {
	marks   glyphs
	choices pickList[placeChoice]
	chosen  []place
}

var (
	_ overlay   = placePicker{}
	_ clickable = placePicker{}
	_ steppable = placePicker{}
)

// openPlacePicker opens the checklist on the places the loaded issues are in,
// with those already picked checked.
func (m Model) openPlacePicker() (Model, tea.Cmd) {
	choices := placeChoices(m.issues.found.Issues, m.issues.marksOf, m.issues.places)
	m.overlay = placePicker{
		marks: m.marks, choices: pickList[placeChoice]{items: choices}, chosen: slices.Clone(m.issues.places),
	}

	return m, nil
}

// view draws the checklist in as many rows as fit.
func (p placePicker) view(_, rows int) (string, string) {
	if len(p.choices.items) == 0 {
		return placeTitle, "no issue to narrow"
	}

	return placeTitle, strings.Join(p.choices.rows(p.marks, rows, p.choiceRow), "\n")
}

// choiceRow is a place, checked when it is picked, with how many issues are in
// it.
func (p placePicker) choiceRow(choice placeChoice) string {
	return p.marks.checkbox(slices.Contains(p.chosen, choice.place)) + choice.place.name + "  " +
		strconv.Itoa(choice.count)
}

// footer offers moving, checking a place, applying and canceling.
func (placePicker) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		keys.up, keys.down, keys.toggleOption,
		relabel(keys.confirm, "apply"), relabel(keys.closeOverlay, "cancel"),
	}
}

// handleKey answers a key while the checklist has the keyboard.
func (p placePicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.apply(m)
	case key.Matches(msg, m.keys.toggleOption):
		p = p.toggled()
	case key.Matches(msg, m.keys.down):
		return p.step(m, 1), nil
	case key.Matches(msg, m.keys.up):
		return p.step(m, -1), nil
	}

	m.overlay = p

	return m, nil
}

// toggled is the checklist with the place under the cursor picked, or unpicked
// when it was.
func (p placePicker) toggled() placePicker {
	choice, ok := p.choices.chosen()
	if !ok {
		return p
	}

	if index := slices.Index(p.chosen, choice.place); index >= 0 {
		p.chosen = slices.Delete(slices.Clone(p.chosen), index, index+1)

		return p
	}

	p.chosen = append(slices.Clone(p.chosen), choice.place)

	return p
}

// apply narrows the list to the checked places and closes the checklist.
func (p placePicker) apply(m Model) (Model, tea.Cmd) {
	m.issues = m.issues.keepingSelection(func(l issueList) issueList {
		l.places = p.chosen

		return l
	})
	m = m.closeOverlay()

	return m.loadDetail()
}

// step moves the cursor by delta.
func (p placePicker) step(m Model, delta int) Model {
	p.choices = p.choices.moved(delta)
	m.overlay = p

	return m
}

// click moves the cursor to the clicked place.
func (p placePicker) click(m Model, line int) (Model, tea.Cmd) {
	p.choices = p.choices.clicked(line, m.detailRows())
	m.overlay = p

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
		parts = append(parts, "filter: "+l.filter)
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
		return "no issue matches the filter"
	case l.filter == "":
		return "no issue in those places"
	default:
		return "no match for filter and places"
	}
}
