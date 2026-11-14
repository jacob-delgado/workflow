// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package latest_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

func TestAReadTakesTheAnswerOfTheRoundItAwaits(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		round nth
		value string
		err   error
		want  view
	}{
		"a reply to the current round": {
			steps: []step{begin()}, round: 1, value: "a",
			want: view{stage: latest.StageShown, value: "a", held: true},
		},
		"a first read that fails holds nothing": {
			steps: []step{begin()}, round: 1, value: "a", err: errRead,
			want: view{stage: latest.StageFailed, err: errRead},
		},
		"a failure after a shown answer keeps the value and shows the error": {
			steps: []step{begin(), answer(1, "a", nil), begin()}, round: 2, value: "b", err: errRead,
			want: view{stage: latest.StageFailed, value: "a", held: true, err: errRead},
		},
		"a success clears an earlier error": {
			steps: []step{begin(), answer(1, "a", nil), begin(), answer(2, "", errRead), begin()}, round: 3, value: "b",
			want: view{stage: latest.StageShown, value: "b", held: true},
		},
		"a failure after a failure shows the later error": {
			steps: []step{begin(), answer(1, "", errRead), begin()}, round: 2, err: errAgain,
			want: view{stage: latest.StageFailed, err: errAgain},
		},
		"a failure after a told value keeps the value": {
			steps: []step{tell(), begin()}, round: 1, err: errRead,
			want: view{stage: latest.StageFailed, value: told, held: true, err: errRead},
		},
		"begin while reading: the later round's answer": {
			steps: []step{begin(), begin()}, round: 2, value: "b",
			want: view{stage: latest.StageShown, value: "b", held: true},
		},
		"round 0 on a read never begun, told or cleared": {
			round: initRound, value: "a",
			want: view{stage: latest.StageShown, value: "a", held: true},
		},
		"round 0 failing on a read never begun, told or cleared": {
			round: initRound, err: errRead,
			want: view{stage: latest.StageFailed, err: errRead},
		},
		"an answer replaces the seed standing in for it": {
			steps: []step{begin(), seed(1)}, round: 1, value: "a",
			want: view{stage: latest.StageShown, value: "a", held: true},
		},
		"a failure keeps the seed standing in for it": {
			steps: []step{begin(), seed(1)}, round: 1, err: errRead,
			want: view{stage: latest.StageFailed, value: cached, held: true, err: errRead},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Answered(arranged.round(tt.round), tt.value, tt.err)

			// Assert
			if !taken || viewOf(got) != tt.want {
				t.Errorf("taken %t, %+v; want taken, %+v", taken, viewOf(got), tt.want)
			}
		})
	}
}

func TestAReadDropsAnAnswerItNoLongerAwaits(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		round nth
		value string
		err   error
	}{
		"a reply to a superseded round": {steps: []step{begin(), begin()}, round: 1, value: "a"},
		"a superseded failure after a later success": {
			steps: []step{begin(), begin(), answer(2, "b", nil)}, round: 1, err: errRead,
		},
		"answers in reverse order: the earlier last": {
			steps: []step{begin(), begin(), answer(2, "b", nil)}, round: 1, value: "a",
		},
		"round 0 after a Begin":   {steps: []step{begin()}, round: initRound, value: "a"},
		"round 0 after a Told":    {steps: []step{tell()}, round: initRound, value: "a"},
		"round 0 after a Cleared": {steps: []step{clearRead()}, round: initRound, value: "a"},
		"round 0 a second time": {
			steps: []step{answer(initRound, "a", nil)}, round: initRound, value: "b",
		},
		"round 0 once settled":                     {steps: []step{settle(initRound)}, round: initRound, value: "a"},
		"a second answer to one round":             {steps: []step{begin(), answer(1, "a", nil)}, round: 1, value: "b"},
		"a second answer to one round, failing":    {steps: []step{begin(), answer(1, "a", nil)}, round: 1, err: errRead},
		"an answer to a round settled":             {steps: []step{begin(), settle(1)}, round: 1, value: "a"},
		"an answer with nothing out, after a Told": {steps: []step{tell()}, round: nowRound, value: "a"},
		"an answer with nothing out, after a Cleared": {
			steps: []step{begin(), answer(1, "a", nil), clearRead()}, round: nowRound, value: "b",
		},
		"a read out when a value is told":         {steps: []step{begin(), tell()}, round: 1, value: "a"},
		"a read out across a Cleared and a Begin": {steps: []step{begin(), clearRead(), begin()}, round: 1, value: "a"},
		"a round never begun":                     {steps: []step{begin()}, round: nextRound, value: "a"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Answered(arranged.round(tt.round), tt.value, tt.err)

			// Assert
			if taken || got != arranged.read {
				t.Errorf("taken %t, %+v; want dropped, %+v unchanged", taken, viewOf(got), viewOf(arranged.read))
			}
		})
	}
}
