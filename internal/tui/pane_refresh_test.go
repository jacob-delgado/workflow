// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// stale is longer ago than a pane's load stays fresh.
const stale = 31 * time.Second

func TestSwitchingToAStalePaneReloadsIt(t *testing.T) {
	t.Parallel()

	for name, via := range map[string][]string{"its number": {"6"}, keyTab: {"5", keyTab}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reading := newWorld()
			model := reading.live(t, 120, 40)
			reading.goStale()
			before := len(reading.asked("reviews"))

			// Act
			typing(t, model, via...)

			// Assert
			if reads := len(reading.asked("reviews")); reads != before+1 {
				t.Errorf("read the review queue %d times, want once more than the %d before", reads, before)
			}
		})
	}
}

func TestSwitchingToAPaneLoadedMomentsAgoLeavesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := newWorld()
	model := reading.live(t, 120, 40)
	before := len(reading.asked("reviews"))

	// Act
	typing(t, model, "6")

	// Assert
	if reads := len(reading.asked("reviews")); reads != before {
		t.Errorf("read the review queue %d times, want the %d of the start alone", reads, before)
	}
}

func TestRReloadsAPaneLoadedMomentsAgo(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := newWorld()
	model := typing(t, reading.live(t, 120, 40), "6")
	before := len(reading.asked("reviews"))

	// Act
	typing(t, model, "r")

	// Assert
	if reads := len(reading.asked("reviews")); reads != before+1 {
		t.Errorf("read the review queue %d times, want once more than the %d before r", reads, before)
	}
}

func TestSwitchingBackToIssuesWithMorePagesLoadedKeepsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	paged := newWorld()
	paged.pageSize = 1
	model := typing(t, paged.live(t, 120, 40), "ctrl+n", "2")
	paged.goStale()
	before := len(paged.asked("search"))

	// Act
	typing(t, model, "1")

	// Assert
	if searches := len(paged.asked("search")); searches != before {
		t.Errorf("searched %d times, want the %d before: a reload would drop the second page", searches, before)
	}
}

func TestSwitchingBackToIssuesOnTheFirstPageReloadsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := newWorld()
	model := typing(t, reading.live(t, 120, 40), "2")
	reading.goStale()
	reading.issues = append(reading.issues, jira.Issue{Key: "PROJ-77", Summary: "Arrived since", Status: "Selected"})

	// Act
	view := typing(t, model, "1").View().Content

	// Assert
	requireScreen(t, view, "PROJ-77")
}

func TestRInTheMessagingPaneReadsWhatWasAnnounced(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := newWorld()
	model := typing(t, reading.live(t, 120, 40), "5")
	before := len(reading.asked("history"))

	// Act
	typing(t, model, "r")

	// Assert
	if reads := len(reading.asked("history")); reads != before+1 {
		t.Errorf("read the announcements %d times, want once more than the %d before r", reads, before)
	}
}

// Branch, Commits, Review and Messaging each reload the branch, which goes on
// to find its pull request and read CI: one reload serves all four.
func TestTabbingThroughThePanesTheBranchFeedsLooksForThePullRequestOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := newWorld()
	model := reading.live(t, 120, 40)
	reading.goStale()
	before := len(reading.asked("find "))

	// Act
	typing(t, model, keyTab, keyTab, keyTab, keyTab)

	// Assert
	if finds := len(reading.asked("find ")); finds != before+1 {
		t.Errorf("looked for the pull request %d times, want once more than the %d before", finds, before)
	}
}
