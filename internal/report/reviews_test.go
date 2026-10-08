// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/report"
)

const (
	requestAuthor     = "octo"
	requestRepository = "owner/repo"
)

func TestNoReviewRequestIsAnEmptyListNotNull(t *testing.T) {
	t.Parallel()

	// Act
	queue := report.ReviewRequests(nil)

	// Assert
	if queue == nil || len(queue) != 0 {
		t.Errorf("ReviewRequests(nil) = %#v, want an empty list", queue)
	}
}

func TestAReviewRequestCarriesItsCIAndWhenItOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := time.Date(2026, time.March, 3, 9, 0, 0, 0, time.UTC)
	request := forge.ReviewRequest{
		Number: 7, URL: "https://github.com/owner/repo/pull/7", Title: "Redact tokens", Author: requestAuthor,
		Repository: requestRepository, CI: forge.CIPassed, OpenedAt: opened,
	}

	// Act
	queue := report.ReviewRequests([]forge.ReviewRequest{request})

	// Assert
	got := queue[0]
	passed := api.CIState(forge.CIPassed.Word())

	if got.Number != 7 || got.Ci != passed || got.OpenedAt == nil || !got.OpenedAt.Equal(opened) {
		t.Errorf("the request = %+v, want #7, passed, opened when it was", got)
	}
}

func TestAReviewRequestTheForgeGaveNoOpeningForLeavesItOut(t *testing.T) {
	t.Parallel()

	// Act
	queue := report.ReviewRequests([]forge.ReviewRequest{{Number: 7}})

	// Assert
	if queue[0].OpenedAt != nil {
		t.Errorf("OpenedAt = %v, want it left out", queue[0].OpenedAt)
	}
}

func TestAReviewRequestCarriesEachFacetItHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	request := forge.ReviewRequest{
		Number: 7, Repository: requestRepository, Author: requestAuthor, CI: forge.CIPassed, Draft: true,
	}
	want := []api.ReviewFacet{
		{Kind: api.ReviewFacetKindRepository, Value: requestRepository, Label: requestRepository},
		{Kind: api.ReviewFacetKindCi, Value: "passed", Label: "CI passed"},
		{Kind: api.ReviewFacetKindDraft, Value: forge.ReviewDraft, Label: forge.ReviewDraft},
		{Kind: api.ReviewFacetKindAuthor, Value: requestAuthor, Label: "by " + requestAuthor},
	}

	// Act
	queue := report.ReviewRequests([]forge.ReviewRequest{request})

	// Assert
	if !slices.Equal(queue[0].Facets, want) {
		t.Errorf("Facets = %+v, want %+v", queue[0].Facets, want)
	}
}

func TestNoReviewFacetIsAnEmptyListNotNull(t *testing.T) {
	t.Parallel()

	// Act
	facets := report.ReviewFacets(nil)

	// Assert
	if facets == nil || len(facets) != 0 {
		t.Errorf("ReviewFacets(nil) = %#v, want an empty list", facets)
	}
}
