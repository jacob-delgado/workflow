// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// filterTitle titles the detail pane while the reviews filter is open.
const filterTitle = "Filter"

// facetKind is which of the four facets a choice narrows by. Choices in one
// facet widen the queue, and the facets narrow it together.
//
// Trade-off TRADE-23: these facets and the rules below are written again in
// web/src/features/reviewqueue/reviewFacets.ts.
type facetKind int

const (
	facetRepository facetKind = iota
	facetCI
	facetDraft
	facetAuthor
)

// facet is one value a request can hold in one facet: its repository, how its
// CI stands, whether it is a draft, or who asks.
type facet struct {
	kind  facetKind
	value string
}

// facetChoice is a facet value the filter offers, with how many queued
// requests hold it.
type facetChoice struct {
	facet facet
	count int
}

// facetsOf is the value a request holds in each facet.
func facetsOf(request forge.ReviewRequest) []facet {
	readiness := "ready"
	if request.Draft {
		readiness = "draft"
	}

	return []facet{
		{kind: facetRepository, value: request.Repository},
		{kind: facetCI, value: ciWord(request.CI)},
		{kind: facetDraft, value: readiness},
		{kind: facetAuthor, value: request.Author},
	}
}

// The CI states as the filter, the Review pane and the API word them.
const (
	ciWordNone    = "none"
	ciWordRunning = "running"
	ciWordPassed  = "passed"
	ciWordFailed  = "failed"
)

// ciWord is a CI state in words.
func ciWord(state forge.CIState) string {
	return [...]string{
		forge.CINone: ciWordNone, forge.CIRunning: ciWordRunning, forge.CIPassed: ciWordPassed,
		forge.CIFailed: ciWordFailed,
	}[state]
}

// label is how the filter, and the line above the queue, name a facet value.
func (f facet) label() string {
	switch f.kind {
	case facetRepository:
		if f.value == "" {
			return "no repository"
		}

		return f.value
	case facetCI:
		return "CI " + f.value
	case facetAuthor:
		return "by " + f.value
	case facetDraft:
		return f.value
	}

	return f.value
}

// admitsReview reports whether a request holds a picked value in every facet
// something is picked in.
func admitsReview(picked []facet, request forge.ReviewRequest) bool {
	for _, held := range facetsOf(request) {
		constrained := slices.ContainsFunc(picked, func(chosen facet) bool { return chosen.kind == held.kind })
		if constrained && !slices.Contains(picked, held) {
			return false
		}
	}

	return true
}

// filteredReviews is the requests the picked facets admit.
func filteredReviews(picked []facet, requests []forge.ReviewRequest) []forge.ReviewRequest {
	return slices.DeleteFunc(slices.Clone(requests), func(request forge.ReviewRequest) bool {
		return !admitsReview(picked, request)
	})
}

// facetChoices is every value the requests hold, with how many hold each —
// repositories by name, the CI states and draft or ready in a fixed order,
// then authors by name — and every picked value none holds, at zero, so it can
// still be unpicked.
func facetChoices(requests []forge.ReviewRequest, picked []facet) []facetChoice {
	counts := map[facet]int{}

	for _, request := range requests {
		for _, held := range facetsOf(request) {
			counts[held]++
		}
	}

	var choices []facetChoice

	for _, offered := range offeredFacets(counts, picked) {
		if counts[offered] > 0 || slices.Contains(picked, offered) {
			choices = append(choices, facetChoice{facet: offered, count: counts[offered]})
		}
	}

	return choices
}

// offeredFacets is every value the filter could offer, in the order it lists
// them.
func offeredFacets(counts map[facet]int, picked []facet) []facet {
	named := func(kind facetKind) []facet {
		var values []string

		for held := range counts {
			if held.kind == kind {
				values = append(values, held.value)
			}
		}

		for _, chosen := range picked {
			if chosen.kind == kind {
				values = append(values, chosen.value)
			}
		}

		slices.Sort(values)

		return facetsNamed(kind, slices.Compact(values))
	}

	return slices.Concat(
		named(facetRepository),
		facetsNamed(facetCI, []string{ciWordFailed, ciWordPassed, ciWordRunning, ciWordNone}),
		facetsNamed(facetDraft, []string{"draft", "ready"}),
		named(facetAuthor),
	)
}

// facetsNamed is a facet of kind for each of values.
func facetsNamed(kind facetKind, values []string) []facet {
	facets := make([]facet, 0, len(values))

	for _, value := range values {
		facets = append(facets, facet{kind: kind, value: value})
	}

	return facets
}

// facetsLine names the picked facets, for the line above the queue.
func facetsLine(picked []facet) string {
	labels := make([]string, 0, len(picked))

	for _, chosen := range picked {
		labels = append(labels, chosen.label())
	}

	return "filters: " + strings.Join(labels, ", ")
}

// facetPicker is the checklist of facet values to narrow the queue to.
type facetPicker struct {
	marks   glyphs
	choices pickList[facetChoice]
	chosen  []facet
}

var (
	_ overlay   = facetPicker{}
	_ clickable = facetPicker{}
	_ steppable = facetPicker{}
)

// openFacetPicker opens the checklist on the values the queue holds, with
// those already picked checked.
func (m Model) openFacetPicker() (Model, tea.Cmd) {
	choices := facetChoices(m.reviewQueue.all, m.reviewQueue.facets)
	m.overlay = facetPicker{
		marks: m.marks, choices: pickList[facetChoice]{items: choices}, chosen: slices.Clone(m.reviewQueue.facets),
	}

	return m, nil
}

// view draws the checklist in as many rows as fit.
func (p facetPicker) view(_, rows int) (string, string) {
	if len(p.choices.items) == 0 {
		return filterTitle, "no review request to narrow"
	}

	return filterTitle, strings.Join(p.choices.rows(p.marks, rows, p.choiceRow), "\n")
}

// choiceRow is a facet value, checked when it is picked, with how many
// requests hold it.
func (p facetPicker) choiceRow(choice facetChoice) string {
	return p.marks.checkbox(slices.Contains(p.chosen, choice.facet)) + choice.facet.label() + "  " +
		strconv.Itoa(choice.count)
}

// footer offers moving, checking a value, applying and canceling.
func (facetPicker) footer(keys keyMap) []key.Binding {
	return []key.Binding{
		keys.up, keys.down, keys.toggleOption,
		relabel(keys.confirm, "apply"), relabel(keys.closeOverlay, "cancel"),
	}
}

// handleKey answers a key while the checklist has the keyboard.
func (p facetPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return p.apply(m), nil
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

// toggled is the checklist with the value under the cursor picked, or
// unpicked when it was.
func (p facetPicker) toggled() facetPicker {
	choice, ok := p.choices.chosen()
	if !ok {
		return p
	}

	if index := slices.Index(p.chosen, choice.facet); index >= 0 {
		p.chosen = slices.Delete(slices.Clone(p.chosen), index, index+1)

		return p
	}

	p.chosen = append(slices.Clone(p.chosen), choice.facet)

	return p
}

// apply narrows the queue to the checked values and closes the checklist,
// keeping the selection on the request it was on while that is still listed.
func (p facetPicker) apply(m Model) Model {
	previous, _ := m.reviewQueue.current()
	m.reviewQueue.facets = p.chosen
	m.reviewQueue = m.reviewQueue.listed(previous, m.detailRows())

	return m.closeOverlay()
}

// step moves the cursor by delta.
func (p facetPicker) step(m Model, delta int) Model {
	p.choices = p.choices.moved(delta)
	m.overlay = p

	return m
}

// click moves the cursor to the clicked value.
func (p facetPicker) click(m Model, line int) (Model, tea.Cmd) {
	p.choices = p.choices.clicked(line, m.detailRows())
	m.overlay = p

	return m, nil
}
