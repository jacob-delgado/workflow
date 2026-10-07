// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// narrowedTasks is a small list to narrow: two linked, one not, one waiting.
func narrowedTasks() []taskwarrior.Task {
	return []taskwarrior.Task{
		{
			UUID: "n1", ID: 4, Description: "Fix the token leak", Status: taskwarrior.Pending, Start: orderNow(),
			Priority: "H", Project: "api", Tags: []string{tagWeb}, IssueKey: "PROJ-1",
		},
		{
			UUID: "n2", ID: 7, Description: "Renew the cert", Status: taskwarrior.Pending,
			Project: "infra", Tags: []string{tagCI},
		},
		{UUID: "n3", ID: 9, Description: "Tune the cache", Status: taskwarrior.Pending, Priority: "L", IssueKey: "PROJ-2"},
		{UUID: "n4", Description: "Book the room", Status: taskwarrior.Waiting, Wait: orderNow().Add(time.Hour)},
	}
}

// admitted is the uuids of the tasks a narrowing lets through.
func admitted(narrowing taskwarrior.Narrowing, tasks []taskwarrior.Task) []string {
	var ids []string

	for _, task := range tasks {
		if narrowing.Matches(task, orderNow()) {
			ids = append(ids, task.UUID)
		}
	}

	return ids
}

func TestANarrowingLetsThroughTheTasksItPicks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		narrowing taskwarrior.Narrowing
		want      []string
	}{
		"nothing picked or typed lets every task through": {
			taskwarrior.Narrowing{}, []string{"n1", "n2", "n3", "n4"},
		},
		"values picked in one kind widen": {
			taskwarrior.Narrowing{Picked: []taskwarrior.Facet{
				{Kind: taskwarrior.FacetPriority, Value: "H"}, {Kind: taskwarrior.FacetPriority, Value: "L"},
			}},
			[]string{"n1", "n3"},
		},
		"values picked in two kinds narrow together": {
			taskwarrior.Narrowing{Picked: []taskwarrior.Facet{
				{Kind: taskwarrior.FacetIssue, Value: taskwarrior.WithIssue},
				{Kind: taskwarrior.FacetState, Value: "started"},
			}},
			[]string{"n1"},
		},
		"no tag is a value of its own": {
			taskwarrior.Narrowing{Picked: []taskwarrior.Facet{{Kind: taskwarrior.FacetTag}}},
			[]string{"n3", "n4"},
		},
		"typed text matches the description, ignoring case": {
			taskwarrior.Narrowing{Text: "CERT"}, []string{"n2"},
		},
		"typed text matches a tag written with its plus": {
			taskwarrior.Narrowing{Text: "+" + tagCI}, []string{"n2"},
		},
		"typed text matches an issue key or an id": {
			taskwarrior.Narrowing{Text: "#9"}, []string{"n3"},
		},
		"typed text and picks narrow together": {
			taskwarrior.Narrowing{Text: "the", Picked: []taskwarrior.Facet{{Kind: taskwarrior.FacetProject, Value: "infra"}}},
			[]string{"n2"},
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := admitted(testCase.narrowing, narrowedTasks())

			// Assert
			if !slices.Equal(got, testCase.want) {
				t.Errorf("admitted %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestTypedTextMatchesOneFieldAtATime(t *testing.T) {
	t.Parallel()

	// Arrange
	// "api fix" is in no single field, though "api" is the project and "fix"
	// the description: a match never spans two fields.
	narrowing := taskwarrior.Narrowing{Text: "api fix"}

	// Act
	got := admitted(narrowing, narrowedTasks())

	// Assert
	if len(got) != 0 {
		t.Errorf("admitted %v, want none", got)
	}
}

func TestChoicesOfferEachValueWithItsCount(t *testing.T) {
	t.Parallel()

	// Act
	choices := taskwarrior.Choices(narrowedTasks(), nil, orderNow())

	// Assert
	// Every count is over the listed tasks, waiting ones left out, but for the
	// waiting state, which counts the waiting tasks.
	got := make([]string, 0, len(choices))
	for _, choice := range choices {
		got = append(got, choice.Facet.Label()+" "+strconv.Itoa(choice.Count))
	}

	want := []string{
		"started 1", "pending 2", "waiting 1",
		"priority H 1", "priority L 1", "no priority 1",
		"project api 1", "project infra 1", "no project 1",
		"+ci 1", "+web 1", "no tag 1",
		"with issue 2", "no issue 1",
	}
	if !slices.Equal(got, want) {
		t.Errorf("choices = %q, want %q", got, want)
	}
}

func TestAPickedValueNoTaskHoldsIsStillOfferedAtZero(t *testing.T) {
	t.Parallel()

	// Arrange
	picked := []taskwarrior.Facet{{Kind: taskwarrior.FacetPriority, Value: "M"}}

	// Act
	choices := taskwarrior.Choices(narrowedTasks(), picked, orderNow())

	// Assert
	if !slices.Contains(choices, taskwarrior.FacetChoice{Facet: picked[0], Count: 0}) {
		t.Errorf("choices = %v, want priority M offered at 0 so it can be unpicked", choices)
	}
}

func TestOnlyPickingWaitingListsWaitingTasks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		picked []taskwarrior.Facet
		want   bool
	}{
		"nothing picked":  {nil, false},
		"pending picked":  {[]taskwarrior.Facet{{Kind: taskwarrior.FacetState, Value: "pending"}}, false},
		"waiting picked":  {[]taskwarrior.Facet{{Kind: taskwarrior.FacetState, Value: "waiting"}}, true},
		"a priority only": {[]taskwarrior.Facet{{Kind: taskwarrior.FacetPriority, Value: "H"}}, false},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := taskwarrior.Narrowing{Picked: testCase.picked}.ListsWaiting()

			// Assert
			if got != testCase.want {
				t.Errorf("ListsWaiting = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAFacetOfAKindTheListDoesNotKnowHasNoLabel(t *testing.T) {
	t.Parallel()

	// Arrange
	const unknownKind taskwarrior.FacetKind = 9

	// Act
	label := taskwarrior.Facet{Kind: unknownKind}.Label()

	// Assert
	if label != "" {
		t.Errorf("Label() = %q, want none for a kind the list does not know", label)
	}
}
