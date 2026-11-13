// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// How the combined tracker pages the first view: a later page asked by its
// offset, and one asked after a cursor, which carries Jira's own offset.

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestALaterPageAsksJiraPastItsOwnRows(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	_, err := tracker.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("the first page: %v", err)
	}

	// Act
	// Two rows are listed, one of them the forge's, so Jira has given one.
	_, err = tracker.Search(t.Context(), jira.AssignedToMe, 2)

	// Assert
	asked := stand.requests()
	if err != nil || !strings.Contains(asked[len(asked)-1], "startAt=1") {
		t.Errorf("Search = %v; Jira was last asked %q, want from its second row", err, asked[len(asked)-1])
	}
}

func TestTheFirstPageNamesJirasLastRowAsWhereTheNextStarts(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})
	tracker := bothTrackers(t, &fakeJira{})

	// Act
	result, err := tracker.Search(t.Context(), jira.AssignedToMe, 0)

	// Assert
	if want := (jira.Cursor{After: jiraKey, At: 0}); err != nil || result.Next != want {
		t.Errorf("Search's Next = %+v, %v; want %+v, Jira's own row at Jira's own offset", result.Next, err, want)
	}
}

// The forge's issues lead only the first page, so a page after a cursor asks
// Jira from the cursor's own offset, whatever count of forge rows another
// first page has left since: a first page whose forge read failed counts none.
func TestAPageAfterACursorAsksJiraFromItsOwnOffsetAlone(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{search: theBugList})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	_, err := tracker.Search(t.Context(), jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("the first page: %v", err)
	}

	forgeAsked := len(ghStub.args())

	// Act
	result, err := tracker.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{After: "PROJ-0", At: 5})

	// Assert
	asked := stand.requests()
	if err != nil || !strings.Contains(asked[len(asked)-1], "startAt=5") {
		t.Errorf("SearchAfter = %v; Jira was last asked %q, want from the cursor's offset, 5", err, asked[len(asked)-1])
	}

	if len(ghStub.args()) != forgeAsked || slices.Contains(keysOf(result), "42") {
		t.Errorf("SearchAfter listed %v, gh asked %v; want Jira's rows alone past the first page",
			keysOf(result), ghStub.args())
	}
}

func TestAPageAfterNoCursorIsTheFirstPageWithTheForgesIssues(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})
	tracker := bothTrackers(t, &fakeJira{})

	// Act
	result, err := tracker.SearchAfter(t.Context(), jira.AssignedToMe, jira.Cursor{})

	// Assert
	if err != nil || !slices.Equal(keysOf(result), []jira.Key{"42", jiraKey}) || result.Total != 4 {
		t.Errorf("SearchAfter = %v of %d, %v; want the forge's 42 then Jira's PROJ-1, of 3 + 1",
			keysOf(result), result.Total, err)
	}
}
