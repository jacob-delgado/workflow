// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// everySource is a seam for each source the Summary reads, each answering one
// thing done at start.
func everySource(start time.Time) loop.ActivitySeams {
	return loop.ActivitySeams{
		Commits: func(time.Time, time.Time) []loop.RepositoryCommits { return nil },
		Touched: func(time.Time) ([]taskwarrior.Task, error) { return nil, nil },
		Jira: func(time.Time, time.Time) (jira.Activity, error) {
			return jira.Activity{Events: []jira.Event{{At: start, Kind: jira.EventCreated, Key: "PROJ-1"}}}, nil
		},
		Forge: func(time.Time, time.Time) (forge.Activity, error) {
			return forge.Activity{Events: []forge.Event{{At: start, Kind: forge.EventOpened, Number: 7, Repository: "o/r"}}}, nil
		},
		ForgeKind: forge.KindGitLab,
	}
}

// sourcesOf is the source of each read, in order.
func sourcesOf(reads []loop.SourceRead) []activity.Source {
	sources := make([]activity.Source, 0, len(reads))
	for _, read := range reads {
		sources = append(sources, read.Source)
	}

	return sources
}

func TestTheSummaryReadsEverySourceItReachesInOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()

	// Act
	reads := loop.SummaryReads(everySource(start), start, start.Add(24*time.Hour))

	// Assert
	want := []activity.Source{activity.SourceGit, activity.SourceTasks, activity.SourceJira, activity.SourceForge}
	if got := sourcesOf(reads); !slices.Equal(got, want) {
		t.Errorf("SummaryReads read %v, want %v", got, want)
	}
}

func TestTheSummaryDoesNotAskASourceItDoesNotReach(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	seams := everySource(start)
	seams.Commits, seams.Touched = nil, nil

	// Act
	reads := loop.SummaryReads(seams, start, start.Add(24*time.Hour))

	// Assert
	want := []activity.Source{activity.SourceJira, activity.SourceForge}
	if got := sourcesOf(reads); !slices.Equal(got, want) {
		t.Errorf("SummaryReads read %v, want %v", got, want)
	}
}

func TestReadAllMakesEachReadWithItsSeams(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	seams := everySource(start)
	seams.BrowseURL = func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) }
	seams.Commits, seams.Touched = nil, nil

	// Act
	reads := loop.ReadAll(loop.SummaryReads(seams, start, start.Add(24*time.Hour)))

	// Assert
	if len(reads) != 2 || reads[0].Items[0].URL != "https://jira.example.com/browse/PROJ-1" ||
		reads[1].Items[0].Ref != "o/r!7" {
		t.Errorf("ReadAll = %+v, want Jira's issue linked and the forge's merge request named the GitLab way", reads)
	}
}

func TestAJiraReadWithNoWayToBrowseLinksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	start := summaryStart()
	seams := loop.ActivitySeams{Jira: everySource(start).Jira}

	// Act
	reads := loop.ReadAll(loop.SummaryReads(seams, start, start.Add(24*time.Hour)))

	// Assert
	if len(reads) != 1 || reads[0].Items[0].URL != "" {
		t.Errorf("ReadAll = %+v, want Jira's one issue with no link", reads)
	}
}
