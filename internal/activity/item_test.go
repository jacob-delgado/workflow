// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/activity"
)

var (
	errRefused  = errors.New("refused")
	errNotThere = errors.New("nothing set up")
)

func TestAReadNobodyFailedIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	read := activity.Read{Source: activity.SourceGit}

	// Act & Assert
	if read.Outcome() != activity.OutcomeRead || read.Why() != nil {
		t.Errorf("a fresh read = %v, %v; want read, no reason", read.Outcome(), read.Why())
	}
}

func TestAReadsOutcomeAfterEachTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		turn    func(activity.Read) activity.Read
		outcome activity.Outcome
		why     error
	}{
		{
			name:    "a failing read fails",
			turn:    func(read activity.Read) activity.Read { return read.Failing(errRefused) },
			outcome: activity.OutcomeFailed, why: errRefused,
		},
		{
			name:    "a read left out is not set up",
			turn:    func(read activity.Read) activity.Read { return read.LeftOut(errNotThere) },
			outcome: activity.OutcomeNotSetUp, why: errNotThere,
		},
		{
			name: "a refusal outranks not set up",
			turn: func(read activity.Read) activity.Read {
				return read.LeftOut(errNotThere).Failing(errRefused)
			},
			outcome: activity.OutcomeFailed, why: errRefused,
		},
		{
			name: "a failed read left out stays failed",
			turn: func(read activity.Read) activity.Read {
				return read.Failing(errRefused).LeftOut(errNotThere)
			},
			outcome: activity.OutcomeFailed, why: errRefused,
		},
		{
			name:    "no failure leaves it as it was",
			turn:    func(read activity.Read) activity.Read { return read.Failing(nil) },
			outcome: activity.OutcomeRead, why: nil,
		},
		{
			name:    "no reason to leave it out leaves it as it was",
			turn:    func(read activity.Read) activity.Read { return read.LeftOut(nil) },
			outcome: activity.OutcomeRead, why: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			read := activity.Read{Source: activity.SourceJira}

			// Act
			got := tt.turn(read)

			// Assert
			if got.Outcome() != tt.outcome || !errors.Is(got.Why(), tt.why) {
				t.Errorf("outcome = %v, %v; want %v, %v", got.Outcome(), got.Why(), tt.outcome, tt.why)
			}

			if got.Source != activity.SourceJira {
				t.Errorf("source = %v, want Jira kept", got.Source)
			}
		})
	}
}
