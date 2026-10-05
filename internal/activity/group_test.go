// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
)

// at is an instant, written as a test reads it.
func at(t *testing.T, text string) time.Time {
	t.Helper()

	instant, err := time.Parse(time.RFC3339, text)
	if err != nil {
		t.Fatalf("parsing %s: %v", text, err)
	}

	return instant
}

// hourLabels are the hours a grouping holds, in order, by label.
func hourLabels(years []activity.Year) []string {
	var labels []string

	for _, year := range years {
		for _, month := range year.Months {
			for _, day := range month.Days {
				for _, hour := range day.Hours {
					labels = append(labels, day.Date.String()+" "+hour.Label)
				}
			}
		}
	}

	return labels
}

func TestGroupingNestsYearMonthDayAndHourOldestFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	items := []activity.Item{
		{At: at(t, "2027-01-01T09:30:00Z"), Kind: activity.Committed, Title: "new year"},
		{At: at(t, "2026-12-31T09:05:00Z"), Kind: activity.Committed, Title: "first"},
		{At: at(t, "2026-12-31T09:55:00Z"), Kind: activity.TaskCompleted, Title: "same hour"},
		{At: at(t, "2026-12-31T14:00:00Z"), Kind: activity.IssueMoved, Title: "afternoon"},
	}

	// Act
	years := activity.Group(items, time.UTC)

	// Assert
	want := []string{"2026-12-31 09:00", "2026-12-31 14:00", "2027-01-01 09:00"}
	if got := hourLabels(years); !equal(got, want) {
		t.Errorf("hours = %v, want %v", got, want)
	}

	if len(years) != 2 || years[0].Months[0].Days[0].Hours[0].Items[1].Title != "same hour" {
		t.Errorf("Group = %+v, want two years, with 09:00 holding both of its items in order", years)
	}
}

func TestTheHourTheClocksRepeatIsTwoHoursNamedApart(t *testing.T) {
	t.Parallel()

	// Arrange
	// On 2026-11-01 New York's 01:00 comes twice: first in EDT, then in EST.
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}

	items := []activity.Item{
		{At: at(t, "2026-11-01T05:30:00Z"), Kind: activity.Committed, Title: "first 01:30"},
		{At: at(t, "2026-11-01T06:30:00Z"), Kind: activity.Committed, Title: "second 01:30"},
	}

	// Act
	years := activity.Group(items, newYork)

	// Assert
	want := []string{"2026-11-01 01:00 EDT", "2026-11-01 01:00 EST"}
	if got := hourLabels(years); !equal(got, want) {
		t.Errorf("hours = %v, want %v", got, want)
	}
}

func TestMergingOrdersByTimeAndKeepsACommitSeenTwiceOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit on two branches, or read from two repositories that share it,
	// is one piece of work.
	commit := activity.Item{At: at(t, "2026-10-02T10:00:00Z"), Kind: activity.Committed, Ref: "abc1234", Title: "fix"}
	task := activity.Item{At: at(t, "2026-10-02T09:00:00Z"), Kind: activity.TaskCompleted, Ref: "12", Title: "done"}

	// Act
	merged := activity.Merge([]activity.Item{commit}, []activity.Item{task, commit})

	// Assert
	if len(merged) != 2 || merged[0] != task || merged[1] != commit {
		t.Errorf("Merge = %+v, want the task then the commit, once", merged)
	}
}

// equal reports two lists of labels the same.
func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}

	return true
}

func TestEachKindIsDoneInOneSource(t *testing.T) {
	t.Parallel()

	cases := map[activity.Kind]activity.Source{
		activity.Committed: activity.SourceGit, activity.TaskAdded: activity.SourceTasks,
		activity.TaskStarted: activity.SourceTasks, activity.TaskAnnotated: activity.SourceTasks,
		activity.TaskCompleted: activity.SourceTasks, activity.IssueCreated: activity.SourceJira,
		activity.IssueMoved: activity.SourceJira, activity.IssueWorked: activity.SourceJira,
		activity.IssueCommented: activity.SourceJira, activity.PullOpened: activity.SourceForge,
		activity.PullMerged: activity.SourceForge, activity.PullReviewed: activity.SourceForge,
	}

	for kind, want := range cases {
		t.Run(kind.Verb(), func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := kind.Source(); got != want {
				t.Errorf("%s.Source() = %s, want %s", kind.Verb(), got.Name(), want.Name())
			}
		})
	}
}
