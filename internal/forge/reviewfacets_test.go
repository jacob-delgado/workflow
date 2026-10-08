// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The repositories and authors the facet cases queue requests in and by, and
// the labels more than one case names.
const (
	exampleRepo   = "example/repo"
	exampleOther  = "example/other"
	authorKwan    = "kwan"
	authorMira    = "mira"
	noRepository  = "no repository"
	ciFailedLabel = "CI failed"
)

// facetQueue is four requests that differ in every facet, oldest first: #5, a
// draft by kwan in example/repo with CI running; #12 by kwan in example/other,
// passed; #3 by mira in no repository, with no CI; and #7 by mira in
// example/repo, failed.
func facetQueue() []forge.ReviewRequest {
	return []forge.ReviewRequest{
		{Number: 5, Author: authorKwan, Repository: exampleRepo, Draft: true, CI: forge.CIRunning},
		{Number: 12, Author: authorKwan, Repository: exampleOther, CI: forge.CIPassed},
		{Number: 3, Author: authorMira, CI: forge.CINone},
		{Number: 7, Author: authorMira, Repository: exampleRepo, CI: forge.CIFailed},
	}
}

// facet is a facet of kind holding value.
func facet(kind forge.ReviewFacetKind, value string) forge.ReviewFacet {
	return forge.ReviewFacet{Kind: kind, Value: value}
}

// labels is each facet's label, in order.
func labels(facets []forge.ReviewFacet) []string {
	named := make([]string, 0, len(facets))
	for _, each := range facets {
		named = append(named, each.Label())
	}

	return named
}

func TestAReviewRequestHoldsAValueInEachFacet(t *testing.T) {
	t.Parallel()

	// Arrange
	request := facetQueue()[0]

	// Act
	held := request.Facets()

	// Assert
	want := []forge.ReviewFacet{
		facet(forge.FacetRepository, exampleRepo), facet(forge.FacetCI, "running"),
		facet(forge.FacetDraft, forge.ReviewDraft), facet(forge.FacetAuthor, authorKwan),
	}
	if !slices.Equal(held, want) {
		t.Errorf("Facets() = %v, want %v", held, want)
	}
}

func TestAReviewFacetIsLabeledAsEverySurfaceNamesIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		facet forge.ReviewFacet
		want  string
	}{
		"a repository by its name":     {facet: facet(forge.FacetRepository, exampleRepo), want: exampleRepo},
		"no repository":                {facet: facet(forge.FacetRepository, ""), want: noRepository},
		"a CI state":                   {facet: facet(forge.FacetCI, "failed"), want: ciFailedLabel},
		"draft or ready as it is":      {facet: facet(forge.FacetDraft, forge.ReviewReady), want: forge.ReviewReady},
		"an author as who asks for it": {facet: facet(forge.FacetAuthor, authorMira), want: "by mira"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.facet.Label()

			// Assert
			if got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTheFilterOffersRepositoriesThenCIThenDraftThenAuthors(t *testing.T) {
	t.Parallel()

	// Act
	offered := forge.OfferedReviewFacets(facetQueue())

	// Assert
	want := []string{
		noRepository, exampleOther, exampleRepo, ciFailedLabel, "CI passed", "CI running", "CI none",
		forge.ReviewDraft, forge.ReviewReady, "by kwan", "by mira",
	}
	if got := labels(offered); !slices.Equal(got, want) {
		t.Errorf("offered %q, want %q", got, want)
	}
}

func TestTheFilterOffersEveryCIStateAndBothReadinessesWhateverTheQueueHolds(t *testing.T) {
	t.Parallel()

	// Act
	offered := forge.OfferedReviewFacets(nil)

	// Assert
	want := []string{ciFailedLabel, "CI passed", "CI running", "CI none", forge.ReviewDraft, forge.ReviewReady}
	if got := labels(offered); !slices.Equal(got, want) {
		t.Errorf("offered %q for an empty queue, want %q", got, want)
	}
}

// Names are sorted by their UTF-8 bytes, which is code point order: a
// character past the Basic Multilingual Plane sorts after one inside it,
// where JavaScript's own sort, by UTF-16 units, would put it first.
func TestTheFilterSortsNamesByCodePoint(t *testing.T) {
	t.Parallel()

	// Arrange
	requests := []forge.ReviewRequest{
		{Number: 1, Author: "\U0001F600-bot", Repository: exampleRepo},
		{Number: 2, Author: "Ａna", Repository: exampleRepo},
	}

	// Act
	offered := forge.OfferedReviewFacets(requests)

	// Assert
	authors := slices.DeleteFunc(slices.Clone(offered), func(each forge.ReviewFacet) bool {
		return each.Kind != forge.FacetAuthor
	})
	if want := []string{"by Ａna", "by \U0001F600-bot"}; !slices.Equal(labels(authors), want) {
		t.Errorf("authors offered %q, want %q", labels(authors), want)
	}
}

// choiceLines is each choice as the filter lists it: its label and count.
func choiceLines(choices []forge.ReviewFacetChoice) []string {
	lines := make([]string, 0, len(choices))
	for _, choice := range choices {
		lines = append(lines, fmt.Sprintf("%s  %d", choice.Facet.Label(), choice.Count))
	}

	return lines
}

func TestReviewChoicesCountWhatTheQueueHolds(t *testing.T) {
	t.Parallel()

	// Act
	choices := forge.ReviewChoices(facetQueue(), nil)

	// Assert
	want := []string{
		"no repository  1", "example/other  1", "example/repo  2", "CI failed  1", "CI passed  1",
		"CI running  1", "CI none  1", "draft  1", "ready  3", "by kwan  2", "by mira  2",
	}
	if got := choiceLines(choices); !slices.Equal(got, want) {
		t.Errorf("choices %q, want %q", got, want)
	}
}

func TestReviewChoicesKeepAPickedValueNoRequestHoldsLastAtZero(t *testing.T) {
	t.Parallel()

	// Arrange
	picked := []forge.ReviewFacet{facet(forge.FacetAuthor, "ana"), facet(forge.FacetCI, "running")}

	// Act
	choices := forge.ReviewChoices(facetQueue()[1:], picked)

	// Assert
	want := []string{
		"no repository  1", "example/other  1", "example/repo  1", "CI failed  1", "CI passed  1",
		"CI running  0", "CI none  1", "ready  3", "by kwan  1", "by mira  2", "by ana  0",
	}
	if got := choiceLines(choices); !slices.Equal(got, want) {
		t.Errorf("choices %q, want %q", got, want)
	}
}

func TestPickedFacetsWidenWithinAndNarrowTogether(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		picked []forge.ReviewFacet
		want   []int
	}{
		"nothing picked lists all": {want: []int{5, 12, 3, 7}},
		"two repositories list either": {
			picked: []forge.ReviewFacet{
				facet(forge.FacetRepository, exampleOther), facet(forge.FacetRepository, exampleRepo),
			},
			want: []int{5, 12, 7},
		},
		"a repository and a CI state list both": {
			picked: []forge.ReviewFacet{facet(forge.FacetRepository, exampleRepo), facet(forge.FacetCI, "failed")},
			want:   []int{7},
		},
		"two CI states list either": {
			picked: []forge.ReviewFacet{facet(forge.FacetCI, "failed"), facet(forge.FacetCI, "none")},
			want:   []int{3, 7},
		},
		"draft and an author list both": {
			picked: []forge.ReviewFacet{facet(forge.FacetDraft, forge.ReviewDraft), facet(forge.FacetAuthor, authorKwan)},
			want:   []int{5},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			admitted := slices.DeleteFunc(facetQueue(), func(request forge.ReviewRequest) bool {
				return !forge.AdmitsReview(tt.picked, request)
			})

			// Assert
			numbers := make([]int, 0, len(admitted))
			for _, request := range admitted {
				numbers = append(numbers, request.Number)
			}

			if !slices.Equal(numbers, tt.want) {
				t.Errorf("admitted %v, want %v", numbers, tt.want)
			}
		})
	}
}
