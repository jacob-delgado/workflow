// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"slices"
	"strings"
	"time"
)

// ReviewRequest is an open pull or merge request that asks for your review. It
// carries what a review queue is read for — who wants it, since when, and where
// CI stands — across whichever repositories on the forge requested you.
type ReviewRequest struct {
	Number int
	URL    string
	Title  string
	Author string
	// Repository is the owner/name (GitHub) or group/project (GitLab) the request
	// is in, since a review queue spans repositories.
	Repository string
	Draft      bool
	// CI is best effort: the forge's own summary where the listing carries one,
	// and CINone where it does not, since the queue is read without a follow-up
	// request per entry.
	CI       CIState
	OpenedAt time.Time
}

// OldestFirst is the review queue in the order it is worked through, the
// longest-waiting request first — the order every surface lists it in. Requests
// opened at the same moment keep the forge's order, and the forge's answer is
// left as it came.
func OldestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return left.OpenedAt.Compare(right.OpenedAt)
	})

	return queue
}

// NewestFirst is the requests ordered with the one opened last first, for a
// look at what just arrived. Requests opened at the same moment keep the
// forge's order.
func NewestFirst(requests []ReviewRequest) []ReviewRequest {
	queue := slices.Clone(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return right.OpenedAt.Compare(left.OpenedAt)
	})

	return queue
}

// ByRepository is the requests grouped by repository, in its name's order,
// and the longest-waiting first within each.
func ByRepository(requests []ReviewRequest) []ReviewRequest {
	queue := OldestFirst(requests)
	slices.SortStableFunc(queue, func(left, right ReviewRequest) int {
		return strings.Compare(left.Repository, right.Repository)
	})

	return queue
}

// ReviewFacetKind is one of the four facets the review queue is narrowed by:
// a request's repository, how its CI stands, whether it is a draft, and who
// asks. Values picked in one facet widen the queue, and the facets narrow it
// together. The rules are written here alone: the terminal's filter calls
// them, and the web server ships each request's facets and the order they
// are offered in.
type ReviewFacetKind int

// The facets, in the order the filter offers them.
const (
	FacetRepository ReviewFacetKind = iota
	FacetCI
	FacetDraft
	FacetAuthor
)

// The draft facet's two values.
const (
	ReviewDraft = "draft"
	ReviewReady = "ready"
)

// ReviewFacet is one value a request can hold in one facet: its repository
// ("" for none the forge named), how its CI stands, in CIState's words, draft
// or ready, or who asks.
type ReviewFacet struct {
	Kind  ReviewFacetKind
	Value string
}

// Label is how every surface names a facet value, in the filter and in the
// line that says what the queue is narrowed to.
func (f ReviewFacet) Label() string {
	if f.Kind == FacetRepository && f.Value == "" {
		return "no repository"
	}

	return map[ReviewFacetKind]string{FacetRepository: "", FacetCI: "CI ", FacetDraft: "", FacetAuthor: "by "}[f.Kind] +
		f.Value
}

// Facets is the value the request holds in each facet.
func (r ReviewRequest) Facets() []ReviewFacet {
	readiness := ReviewReady
	if r.Draft {
		readiness = ReviewDraft
	}

	return []ReviewFacet{
		{Kind: FacetRepository, Value: r.Repository},
		{Kind: FacetCI, Value: r.CI.Word()},
		{Kind: FacetDraft, Value: readiness},
		{Kind: FacetAuthor, Value: r.Author},
	}
}

// OfferedReviewFacets is every value the queue's filter offers, in the order
// it lists them: the repositories the requests are in, by name, every CI
// state and draft then ready in a fixed order, and who asks, by name. A name
// is ordered by its UTF-8 bytes, which is code point order.
func OfferedReviewFacets(requests []ReviewRequest) []ReviewFacet {
	named := func(kind ReviewFacetKind) []ReviewFacet {
		var values []string

		for _, request := range requests {
			for _, held := range request.Facets() {
				if held.Kind == kind {
					values = append(values, held.Value)
				}
			}
		}

		slices.Sort(values)

		return reviewFacetsOf(kind, slices.Compact(values))
	}

	return slices.Concat(
		named(FacetRepository),
		reviewFacetsOf(FacetCI, []string{CIFailed.Word(), CIPassed.Word(), CIRunning.Word(), CINone.Word()}),
		reviewFacetsOf(FacetDraft, []string{ReviewDraft, ReviewReady}),
		named(FacetAuthor),
	)
}

// reviewFacetsOf is a facet of kind for each of values.
func reviewFacetsOf(kind ReviewFacetKind, values []string) []ReviewFacet {
	facets := make([]ReviewFacet, 0, len(values))

	for _, value := range values {
		facets = append(facets, ReviewFacet{Kind: kind, Value: value})
	}

	return facets
}

// ReviewFacetChoice is a value the filter offers, with how many requests hold
// it.
type ReviewFacetChoice struct {
	Facet ReviewFacet
	Count int
}

// ReviewChoices is every value the requests hold, with how many hold each, in
// the order OfferedReviewFacets offers them, then every picked value none
// holds, at zero, in the order it was picked, so it can still be unpicked.
func ReviewChoices(requests []ReviewRequest, picked []ReviewFacet) []ReviewFacetChoice {
	counts := map[ReviewFacet]int{}

	for _, request := range requests {
		for _, held := range request.Facets() {
			counts[held]++
		}
	}

	var choices []ReviewFacetChoice

	for _, offering := range OfferedReviewFacets(requests) {
		if counts[offering] > 0 || slices.Contains(picked, offering) {
			choices = append(choices, ReviewFacetChoice{Facet: offering, Count: counts[offering]})
		}
	}

	for _, chosen := range picked {
		if counts[chosen] == 0 && !slices.ContainsFunc(choices, func(choice ReviewFacetChoice) bool {
			return choice.Facet == chosen
		}) {
			choices = append(choices, ReviewFacetChoice{Facet: chosen})
		}
	}

	return choices
}

// AdmitsReview reports whether a request holds a picked value in every facet
// something is picked in.
func AdmitsReview(picked []ReviewFacet, request ReviewRequest) bool {
	for _, held := range request.Facets() {
		constrained := slices.ContainsFunc(picked, func(chosen ReviewFacet) bool { return chosen.Kind == held.Kind })
		if constrained && !slices.Contains(picked, held) {
			return false
		}
	}

	return true
}
