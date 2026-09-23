// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// reviewStatus is the status an issue moves to once its pull request is open.
const reviewStatus = "In Review"

// offering answers with the moves Jira offers, as a Transitions seam.
func offering(moves ...jira.Transition) func(jira.Key) ([]jira.Transition, error) {
	return func(jira.Key) ([]jira.Transition, error) { return moves, nil }
}

func TestReviewTransitionFindsTheMoveToTheReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	transitions := offering(
		jira.Transition{ID: "11", ToStatus: "In Progress"},
		jira.Transition{ID: "21", ToStatus: "in review"},
	)

	// Act
	target, ok := loop.ReviewTransition(transitions, issueKey, reviewStatus)

	// Assert
	if !ok || target.ID != "21" {
		t.Errorf("ReviewTransition = %+v, %t; want transition 21 offered", target, ok)
	}
}

func TestReviewTransitionOffersNothingItCannotApply(t *testing.T) {
	t.Parallel()

	needsFields := jira.Transition{ID: "21", ToStatus: reviewStatus, Fields: []jira.Field{{ID: "resolution"}}}

	cases := map[string]struct {
		transitions func(jira.Key) ([]jira.Transition, error)
		key         jira.Key
		status      string
	}{
		"a branch that names no issue": {
			transitions: offering(jira.Transition{ToStatus: reviewStatus}), status: reviewStatus,
		},
		"no way to read the transitions": {key: issueKey, status: reviewStatus},
		"a read that fails": {
			transitions: func(jira.Key) ([]jira.Transition, error) { return nil, errSeam },
			key:         issueKey, status: reviewStatus,
		},
		"a status Jira does not offer": {
			transitions: offering(jira.Transition{ToStatus: "Done"}), key: issueKey, status: reviewStatus,
		},
		"a move that needs fields": {transitions: offering(needsFields), key: issueKey, status: reviewStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			target, ok := loop.ReviewTransition(tt.transitions, tt.key, tt.status)

			// Assert
			if ok {
				t.Errorf("ReviewTransition = %+v, true; want no offer", target)
			}
		})
	}
}

func TestReviewTransitionAsksNothingWithoutAReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no review status configured there is nothing to offer, so the tracker
	// is not asked at all; every `workflow pr` would otherwise pay a round trip.
	asked := false
	transitions := func(jira.Key) ([]jira.Transition, error) {
		asked = true

		return []jira.Transition{{ID: "21", ToStatus: reviewStatus}}, nil
	}

	// Act
	target, ok := loop.ReviewTransition(transitions, issueKey, "")

	// Assert
	if ok || asked {
		t.Errorf("ReviewTransition = %+v, %t, asked = %t; want no offer and the tracker not asked", target, ok, asked)
	}
}

func TestFindReviewTransitionSaysWhyThereIsNoMove(t *testing.T) {
	t.Parallel()

	needsFields := jira.Transition{ID: "21", ToStatus: reviewStatus, Fields: []jira.Field{{ID: "resolution"}}}

	cases := map[string]struct {
		transitions func(jira.Key) ([]jira.Transition, error)
		key         jira.Key
		status      string
		want        error
	}{
		"no review status configured": {
			transitions: offering(jira.Transition{ToStatus: reviewStatus}), key: issueKey, want: loop.ErrNoReviewStatus,
		},
		"a branch that names no issue": {
			transitions: offering(jira.Transition{ToStatus: reviewStatus}), status: reviewStatus,
			want: loop.ErrNoReviewTransition,
		},
		"no way to read the transitions": {key: issueKey, status: reviewStatus, want: loop.ErrNoReviewTransition},
		"a read that fails": {
			transitions: func(jira.Key) ([]jira.Transition, error) { return nil, errSeam },
			key:         issueKey, status: reviewStatus, want: errSeam,
		},
		"a status Jira does not offer": {
			transitions: offering(jira.Transition{ToStatus: "Done"}), key: issueKey, status: reviewStatus,
			want: loop.ErrNoReviewTransition,
		},
		"a move that needs fields": {
			transitions: offering(needsFields), key: issueKey, status: reviewStatus, want: loop.ErrReviewNeedsFields,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			target, err := loop.FindReviewTransition(tt.transitions, tt.key, tt.status)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("FindReviewTransition = %+v, %v; want %v", target, err, tt.want)
			}
		})
	}
}

func TestFindReviewTransitionFindsTheFieldlessMove(t *testing.T) {
	t.Parallel()

	// Arrange
	transitions := offering(jira.Transition{ID: "21", ToStatus: "In review"})

	// Act
	target, err := loop.FindReviewTransition(transitions, issueKey, reviewStatus)

	// Assert
	if err != nil || target.ID != "21" {
		t.Errorf("FindReviewTransition = %+v, %v; want transition 21", target, err)
	}
}
