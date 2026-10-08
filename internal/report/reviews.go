// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// ReviewRequests maps the requests waiting on your review onto the wire, each
// with its facets, an empty list rather than null when none waits: the
// requests GET /api/reviews answers, and what `workflow reviews --json`
// prints.
func ReviewRequests(requests []forge.ReviewRequest) []api.ReviewRequest {
	queue := make([]api.ReviewRequest, 0, len(requests))
	for _, request := range requests {
		queue = append(queue, api.ReviewRequest{
			Number: request.Number, URL: request.URL, Title: request.Title, Author: request.Author,
			Repository: request.Repository, Draft: request.Draft, Ci: api.CIState(request.CI.Word()),
			OpenedAt: openedAt(request.OpenedAt), Facets: ReviewFacets(request.Facets()),
		})
	}

	return queue
}

// ReviewFacets maps review facets onto the wire, each with its label, an
// empty list rather than null: a request's own, and the values the review
// queue's filter offers. A map, so exhaustive keeps the kinds complete.
func ReviewFacets(facets []forge.ReviewFacet) []api.ReviewFacet {
	kinds := map[forge.ReviewFacetKind]api.ReviewFacetKind{
		forge.FacetRepository: api.ReviewFacetKindRepository, forge.FacetCI: api.ReviewFacetKindCi,
		forge.FacetDraft: api.ReviewFacetKindDraft, forge.FacetAuthor: api.ReviewFacetKindAuthor,
	}

	out := make([]api.ReviewFacet, 0, len(facets))
	for _, facet := range facets {
		out = append(out, api.ReviewFacet{Kind: kinds[facet.Kind], Value: facet.Value, Label: facet.Label()})
	}

	return out
}

// openedAt is when a request was opened, or nil where the forge did not say:
// the wire leaves such a time out, never sending the zero time.
func openedAt(moment time.Time) *time.Time {
	if moment.IsZero() {
		return nil
	}

	return &moment
}
