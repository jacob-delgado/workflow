// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// filterTitle titles the detail pane while a filter checklist is open, on the
// Issues, Reviews or Tasks pane.
const filterTitle = "Filter"

// filteredReviews is the requests the picked facets admit.
func filteredReviews(picked []forge.ReviewFacet, requests []forge.ReviewRequest) []forge.ReviewRequest {
	return slices.DeleteFunc(slices.Clone(requests), func(request forge.ReviewRequest) bool {
		return !forge.AdmitsReview(picked, request)
	})
}

// facetChoices is the filter's choices over the requests, with those picked
// still offered: forge.ReviewChoices, as the checklist lists them.
func facetChoices(requests []forge.ReviewRequest, picked []forge.ReviewFacet) []offered[forge.ReviewFacet] {
	choices := forge.ReviewChoices(requests, picked)
	offers := make([]offered[forge.ReviewFacet], 0, len(choices))

	for _, choice := range choices {
		offers = append(offers, offered[forge.ReviewFacet]{value: choice.Facet, count: choice.Count})
	}

	return offers
}

// facetsLine names the picked facets, for the line above the queue.
func facetsLine(picked []forge.ReviewFacet) string {
	labels := make([]string, 0, len(picked))

	for _, chosen := range picked {
		labels = append(labels, chosen.Label())
	}

	return "filters: " + strings.Join(labels, ", ")
}

var (
	_ overlay   = checklist[forge.ReviewFacet]{}
	_ clickable = checklist[forge.ReviewFacet]{}
	_ steppable = checklist[forge.ReviewFacet]{}
)

// openFacetPicker opens the checklist on the values the queue holds, with
// those already picked checked. Applying it keeps the selection on the request
// it was on while that is still listed.
func (m Model) openFacetPicker() (Model, tea.Cmd) {
	m.overlay = checklist[forge.ReviewFacet]{
		title: filterTitle, none: "no request to filter",
		choices: pickList[offered[forge.ReviewFacet]]{items: facetChoices(m.reviewQueue.all, m.reviewQueue.facets)},
		chosen:  slices.Clone(m.reviewQueue.facets),
		label:   forge.ReviewFacet.Label,
		apply: func(m Model, chosen []forge.ReviewFacet) (Model, tea.Cmd) {
			previous, _ := m.reviewQueue.current()
			m.reviewQueue.facets = chosen
			m.reviewQueue = m.reviewQueue.listed(previous, m.detailRows())

			return m, nil
		},
	}

	return m, nil
}
