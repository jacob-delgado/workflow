// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package latest_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

func TestSettlingTheRoundAwaitedEndsItAndKeepsWhatIsShown(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		round nth
		want  view
	}{
		"a refresh over a shown answer": {
			steps: []step{begin(), answer(1, "a", nil), begin()}, round: 2,
			want: view{stage: latest.StageShown, value: "a", held: true},
		},
		"a refresh over a failure": {
			steps: []step{begin(), answer(1, "a", nil), begin(), answer(2, "", errRead), begin()}, round: 3,
			want: view{stage: latest.StageFailed, value: "a", held: true, err: errRead},
		},
		"a first read, which holds nothing and so was never read": {
			steps: []step{begin()}, round: 1,
			want: view{stage: latest.StageNeverRead},
		},
		"round 0 on a read never begun, told or cleared": {
			round: initRound,
			want:  view{stage: latest.StageNeverRead},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Settled(arranged.round(tt.round))

			// Assert
			if !taken || viewOf(got) != tt.want {
				t.Errorf("taken %t, %+v; want taken, %+v", taken, viewOf(got), tt.want)
			}
		})
	}
}

func TestSettlingARoundNotAwaitedChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		round nth
	}{
		"a superseded round":     {steps: []step{begin(), begin()}, round: 1},
		"a round answered":       {steps: []step{begin(), answer(1, "a", nil)}, round: 1},
		"a round told":           {steps: []step{tell()}, round: nowRound},
		"round 0 after a Begin":  {steps: []step{begin()}, round: initRound},
		"round 0 after a Settle": {steps: []step{settle(initRound)}, round: initRound},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			arranged := played(t, tt.steps...)

			// Act
			got, taken := arranged.read.Settled(arranged.round(tt.round))

			// Assert
			if taken || got != arranged.read {
				t.Errorf("taken %t, %+v; want not taken, %+v unchanged", taken, viewOf(got), viewOf(arranged.read))
			}
		})
	}
}

func TestBeginningAReadStartsANewRoundOverWhatIsHeld(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps []step
		want  view
	}{
		"never read": {
			want: view{stage: latest.StageReading, out: true},
		},
		"while reading": {
			steps: []step{begin()},
			want:  view{stage: latest.StageReading, out: true},
		},
		"over a shown answer": {
			steps: []step{begin(), answer(1, "a", nil)},
			want:  view{stage: latest.StageShown, value: "a", held: true, out: true, refreshing: true},
		},
		"over a first read that failed": {
			steps: []step{begin(), answer(1, "", errRead)},
			want:  view{stage: latest.StageFailed, err: errRead, out: true, refreshing: true},
		},
		"over a failure after a shown answer": {
			steps: []step{begin(), answer(1, "a", nil), begin(), answer(2, "", errRead)},
			want:  view{stage: latest.StageFailed, value: "a", held: true, err: errRead, out: true, refreshing: true},
		},
		"over a told value": {
			steps: []step{tell()},
			want:  view{stage: latest.StageShown, value: told, held: true, out: true, refreshing: true},
		},
		"after a Cleared": {
			steps: []step{begin(), answer(1, "a", nil), clearRead()},
			want:  view{stage: latest.StageReading, out: true},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			before := played(t, tt.steps...).read

			// Act
			got, round := before.Begin()

			// Assert
			if viewOf(got) != tt.want || round != got.Round() || round == before.Round() {
				t.Errorf("%+v in round %d (Round %d, was %d); want %+v in a new round",
					viewOf(got), round, got.Round(), before.Round(), tt.want)
			}
		})
	}
}

func TestTellingAValueShowsItInANewRound(t *testing.T) {
	t.Parallel()

	cases := map[string][]step{
		"over a read never begun":  nil,
		"over a read out":          {begin()},
		"over a failure":           {begin(), answer(1, "a", nil), begin(), answer(2, "", errRead)},
		"while refreshing a value": {begin(), answer(1, "a", nil), begin()},
	}

	for name, steps := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			before := played(t, steps...).read

			// Act
			got := before.Told(told)

			// Assert
			want := view{stage: latest.StageShown, value: told, held: true}
			if viewOf(got) != want || got.Round() == before.Round() {
				t.Errorf("%+v in round %d (was %d); want %+v in a new round", viewOf(got), got.Round(), before.Round(), want)
			}
		})
	}
}

func TestClearingAReadForgetsItInANewRound(t *testing.T) {
	t.Parallel()

	cases := map[string][]step{
		"a read never begun":            nil,
		"a read out":                    {begin()},
		"over a shown answer":           {begin(), answer(1, "a", nil)},
		"refreshing over a failure":     {begin(), answer(1, "a", nil), begin(), answer(2, "", errRead), begin()},
		"after round 0 was answered":    {answer(initRound, "a", nil)},
		"after a Cleared, never back":   {clearRead()},
		"over a told value, never back": {tell()},
	}

	for name, steps := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			before := played(t, steps...).read

			// Act
			got := before.Cleared()

			// Assert
			if viewOf(got) != (view{}) || got.Round() == before.Round() || got.Round() == 0 {
				t.Errorf("%+v in round %d (was %d); want never read, in a new round past 0",
					viewOf(got), got.Round(), before.Round())
			}
		})
	}
}

func TestAReadIsOutOnlyWhileARoundBegunAwaitsItsAnswer(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		steps           []step
		out, refreshing bool
	}{
		"never read, round 0 awaited": {},
		"reading":                     {steps: []step{begin()}, out: true},
		"shown":                       {steps: []step{begin(), answer(1, "a", nil)}},
		"refreshing a shown answer": {
			steps: []step{begin(), answer(1, "a", nil), begin()}, out: true, refreshing: true,
		},
		"failed": {steps: []step{begin(), answer(1, "", errRead)}},
		"refreshing a failure": {
			steps: []step{begin(), answer(1, "", errRead), begin()}, out: true, refreshing: true,
		},
		"a first read settled":           {steps: []step{begin(), settle(1)}},
		"round 0 answered":               {steps: []step{answer(initRound, "a", nil)}},
		"a value told":                   {steps: []step{tell()}},
		"cleared while reading":          {steps: []step{begin(), clearRead()}},
		"a refresh superseded by a Told": {steps: []step{begin(), answer(1, "a", nil), begin(), tell()}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			read := played(t, tt.steps...).read

			// Act & Assert
			if read.Out() != tt.out || read.Refreshing() != tt.refreshing {
				t.Errorf("out %t, refreshing %t; want %t, %t", read.Out(), read.Refreshing(), tt.out, tt.refreshing)
			}
		})
	}
}
