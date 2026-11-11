// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

// statesOf is every state people lists for owner, in the order it lists them.
func statesOf(people api.People, owner string) []api.OwnerTagState {
	var states []api.OwnerTagState

	for _, listed := range people.Owners {
		if listed.Owner == owner {
			states = append(states, listed.State)
		}
	}

	return states
}

func TestRelinkingAnOwnerInAnotherCaseReplacesWhatWasDecided(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodPut, "/api/people", `{"owner":"Sam.Ortiz","not_on_slack":true}`)

	// Assert
	// Both forges read an owner's name without regard to case, as the real
	// store keeps it: Sam.Ortiz is sam.ortiz.
	people := read[api.People](demo, "/api/people")
	if states := statesOf(people, sam); !slices.Equal(states, []api.OwnerTagState{api.OwnerTagStateNotOnSlack}) {
		t.Errorf("sam.ortiz is %v after Sam.Ortiz was relinked, among %+v; want only not on Slack",
			states, people.Owners)
	}
}

func TestForgettingAnOwnerInAnotherCaseAsksAboutThemAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodDelete, "/api/people?owner=SAM.ORTIZ", "")

	// Assert
	people := read[api.People](demo, "/api/people")
	if states := statesOf(people, sam); len(states) != 0 {
		t.Errorf("sam.ortiz is %v after SAM.ORTIZ was forgotten, want them forgotten", states)
	}
}
