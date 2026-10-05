// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

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
	if f.kind == facetRepository && f.value == "" {
		return "no repository"
	}

	return map[facetKind]string{facetRepository: "", facetCI: "CI ", facetDraft: "", facetAuthor: "by "}[f.kind] +
		f.value
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
func facetChoices(requests []forge.ReviewRequest, picked []facet) []offered[facet] {
	counts := map[facet]int{}

	for _, request := range requests {
		for _, held := range facetsOf(request) {
			counts[held]++
		}
	}

	var choices []offered[facet]

	for _, offering := range offeredFacets(counts, picked) {
		if counts[offering] > 0 || slices.Contains(picked, offering) {
			choices = append(choices, offered[facet]{value: offering, count: counts[offering]})
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

var (
	_ overlay   = checklist[facet]{}
	_ clickable = checklist[facet]{}
	_ steppable = checklist[facet]{}
)

// openFacetPicker opens the checklist on the values the queue holds, with
// those already picked checked. Applying it keeps the selection on the request
// it was on while that is still listed.
func (m Model) openFacetPicker() (Model, tea.Cmd) {
	m.overlay = checklist[facet]{
		marks: m.marks, title: filterTitle, none: "no request to narrow",
		choices: pickList[offered[facet]]{items: facetChoices(m.reviewQueue.all, m.reviewQueue.facets)},
		chosen:  slices.Clone(m.reviewQueue.facets),
		label:   facet.label,
		apply: func(m Model, chosen []facet) (Model, tea.Cmd) {
			previous, _ := m.reviewQueue.current()
			m.reviewQueue.facets = chosen
			m.reviewQueue = m.reviewQueue.listed(previous, m.detailRows())

			return m, nil
		},
	}

	return m, nil
}
