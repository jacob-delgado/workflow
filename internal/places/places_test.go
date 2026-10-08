// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package places_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/places"
)

// casesFile is the place rules' cases, which the web's copy of the rules
// answers to as well.
const casesFile = "../../testdata/twins/places.json"

type placeCase struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type choiceCase struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type issueCase struct {
	Key      string   `json:"key"`
	Tracker  string   `json:"tracker"`
	Status   string   `json:"status"`
	Category string   `json:"category"`
	Marks    []string `json:"marks"`
}

type corpus struct {
	About  string      `json:"about"`
	Issues []issueCase `json:"issues"`
	Marks  []struct {
		Name     string `json:"name"`
		Standing struct {
			InFlight bool   `json:"in_flight"`
			Task     string `json:"task"`
			Forge    bool   `json:"forge"`
		} `json:"standing"`
		Want []string `json:"want"`
	} `json:"marks"`
	Admits []struct {
		Name   string      `json:"name"`
		Picked []placeCase `json:"picked"`
		Want   []string    `json:"want"`
	} `json:"admits"`
	Choices []struct {
		Name   string       `json:"name"`
		Issues []string     `json:"issues"`
		Picked []placeCase  `json:"picked"`
		Want   []choiceCase `json:"want"`
	} `json:"choices"`
}

// readCorpus is the shared cases, refusing a field this side would ignore so a
// case cannot pin one copy and pass the other unread.
func readCorpus(t *testing.T) corpus {
	t.Helper()

	file, err := os.Open(casesFile)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var read corpus

	err = decoder.Decode(&read)
	if err != nil {
		t.Fatalf("%s: %v", casesFile, err)
	}

	return read
}

func picked(cases []placeCase) []places.Place {
	chosen := make([]places.Place, 0, len(cases))
	for _, place := range cases {
		chosen = append(chosen, places.Place{Kind: places.Kind(place.Kind), Name: place.Name})
	}

	return chosen
}

func issuesOf(cases []issueCase) []jira.Issue {
	issues := make([]jira.Issue, 0, len(cases))
	for _, issue := range cases {
		issues = append(issues, jira.Issue{
			Key: jira.Key(issue.Key), Status: issue.Status, StatusCategory: jira.StatusCategory(issue.Category),
		})
	}

	return issues
}

func marksIn(cases []issueCase) func(jira.Key) []string {
	return func(key jira.Key) []string {
		at := slices.IndexFunc(cases, func(issue issueCase) bool { return issue.Key == string(key) })

		return cases[at].Marks
	}
}

func chosenFrom(t *testing.T, world []issueCase, keys []string) []issueCase {
	t.Helper()

	chosen := make([]issueCase, 0, len(keys))

	for _, key := range keys {
		at := slices.IndexFunc(world, func(issue issueCase) bool { return issue.Key == key })
		if at < 0 {
			t.Fatalf("%s names %s, which its issues do not hold", casesFile, key)
		}

		chosen = append(chosen, world[at])
	}

	return chosen
}

func TestAStandingIsInTheMarksTheSharedCasesName(t *testing.T) {
	t.Parallel()

	for _, tt := range readCorpus(t).Marks {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			standing := places.Standing{InFlight: tt.Standing.InFlight, Task: tt.Standing.Task, Forge: tt.Standing.Forge}

			// Act
			got := standing.Marks()

			// Assert
			if !slices.Equal(got, tt.Want) {
				t.Errorf("Marks() = %q, want %q", got, tt.Want)
			}
		})
	}
}

func TestPickedPlacesAdmitTheIssuesTheSharedCasesName(t *testing.T) {
	t.Parallel()

	read := readCorpus(t)

	for _, tt := range read.Admits {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			chosen := picked(tt.Picked)

			// Act
			listed := []string{}

			for _, issue := range read.Issues {
				if places.Admits(chosen, issue.Status, issue.Marks) {
					listed = append(listed, issue.Key)
				}
			}

			// Assert
			if !slices.Equal(listed, tt.Want) {
				t.Errorf("Admits listed %q, want %q", listed, tt.Want)
			}
		})
	}
}

func TestThePlacesOfferedAreTheOnesTheSharedCasesName(t *testing.T) {
	t.Parallel()

	read := readCorpus(t)

	for _, tt := range read.Choices {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := chosenFrom(t, read.Issues, tt.Issues)

			// Act
			choices := places.Choices(issuesOf(world), marksIn(world), picked(tt.Picked))

			// Assert
			got := make([]choiceCase, 0, len(choices))
			for _, choice := range choices {
				got = append(got, choiceCase{Kind: string(choice.Place.Kind), Name: choice.Place.Name, Count: choice.Count})
			}

			if !slices.Equal(got, tt.Want) {
				t.Errorf("Choices = %+v, want %+v", got, tt.Want)
			}
		})
	}
}
