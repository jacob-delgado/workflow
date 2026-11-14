// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package latest_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

func TestASeedStandsInForTheAnswerOfItsRound(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		want  view
	}{
		"seed while the first read is out": {
			steps: []step{begin()},
			want:  view{stage: latest.StageShown, value: cached, held: true, out: true, refreshing: true},
		},
		"seed after the round failed": {
			steps: []step{begin(), answer(1, "", errRead)},
			want:  view{stage: latest.StageFailed, value: cached, held: true, err: errRead},
		},
		"seed while a read is out after a failure": {
			steps: []step{begin(), answer(1, "", errRead), begin()},
			want:  view{stage: latest.StageFailed, value: cached, held: true, err: errRead, out: true, refreshing: true},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Seeded(arranged.read.Round(), cached)

			// Assert
			if !taken || viewOf(got) != tt.want {
				t.Errorf("taken %t, %+v; want taken, %+v", taken, viewOf(got), tt.want)
			}
		})
	}
}

func TestASeedForARoundNotAwaitedOrAReadHoldingAValueChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		round nth
	}{
		"seed for round 0 on a Read never begun":     {steps: nil, round: initRound},
		"seed for a superseded round":                {steps: []step{begin(), begin()}, round: 1},
		"seed with a value held":                     {steps: []step{begin(), answer(1, "a", nil), begin()}, round: 2},
		"seed once the round is answered":            {steps: []step{begin(), answer(1, "a", nil)}, round: 1},
		"seed a second time":                         {steps: []step{begin(), seed(1)}, round: 1},
		"seed after a Cleared":                       {steps: []step{begin(), clearRead()}, round: nowRound},
		"seed for the round of a first read settled": {steps: []step{begin(), settle(1)}, round: 1},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Seeded(arranged.round(tt.round), cached)

			// Assert
			if taken || got != arranged.read {
				t.Errorf("taken %t, %+v; want dropped, %+v unchanged", taken, viewOf(got), viewOf(arranged.read))
			}
		})
	}
}

func TestWithValueChangesTheValueHeldAndNothingElse(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		want  view
	}{
		"a read shown": {
			steps: []step{begin(), answer(1, "a", nil)},
			want:  view{stage: latest.StageShown, value: "b", held: true},
		},
		"a read failed over a value": {
			steps: []step{begin(), answer(1, "a", nil), begin(), answer(2, "", errRead)},
			want:  view{stage: latest.StageFailed, value: "b", held: true, err: errRead},
		},
		"a read refreshing": {
			steps: []step{begin(), answer(1, "a", nil), begin()},
			want:  view{stage: latest.StageShown, value: "b", held: true, out: true, refreshing: true},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got := arranged.read.WithValue("b")

			// Assert
			if viewOf(got) != tt.want || got.Round() != arranged.read.Round() {
				t.Errorf("%+v in round %d; want %+v in round %d",
					viewOf(got), got.Round(), tt.want, arranged.read.Round())
			}
		})
	}
}

func TestAnAnswerAfterWithValueIsStillTaken(t *testing.T) {
	t.Parallel()

	// Arrange
	arranged := played(t, begin(), answer(1, "a", nil), begin())
	changed := arranged.read.WithValue("b")

	// Act
	got, taken := changed.Answered(arranged.round(2), "c", nil)

	// Assert
	if want := (view{stage: latest.StageShown, value: "c", held: true}); !taken || viewOf(got) != want {
		t.Errorf("taken %t, %+v; want taken, %+v", taken, viewOf(got), want)
	}
}
