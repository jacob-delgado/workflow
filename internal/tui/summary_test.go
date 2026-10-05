// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// summaryKey jumps to the Summary pane, whose list is in its detail, so its
// focus shows on the detail's title, summaryTitle.
const (
	summaryKey   = "8"
	summaryTitle = "Summary"
)

var errJiraDown = errors.New("jira is down")

// tuesday is the day before the world's Wednesday, the period the Summary
// opens on.
func tuesday(hour int) time.Time { return time.Date(2026, 9, 15, hour, 0, 0, 0, time.UTC) }

// summaryWorld is a world that did something on Tuesday in every source.
func summaryWorld() *world {
	busy := newWorld()
	busy.done = &activityWorld{
		commits: []gitrepo.DatedCommit{{Short: "abc1234", Subject: "Fix the token leak", Authored: tuesday(9)}},
		jira: jira.Activity{Events: []jira.Event{
			{At: tuesday(14), Kind: jira.EventMoved, Key: issueKey, Summary: issueSummary, Detail: "In Review"},
		}},
		forge: forge.Activity{Events: []forge.Event{
			{At: tuesday(15), Kind: forge.EventOpened, Number: 42, Title: "Redact tokens", URL: pullURL, Repository: "o/r"},
		}},
	}

	return busy
}

func TestTheSummaryOpensOnThePreviousWorkingDayByTheHour(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, view, "8 Summary", "2026-09-15", "Tuesday", "09:00", "committed abc1234 Fix the token leak",
		"14:00", "moved "+issueKey, "15:00", "opened o/r#42 Redact tokens")
}

func TestTheSummaryIsNotReadUntilItIsLookedAt(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()

	// Act
	busy.live(t, 120, 40)

	// Assert
	if reads := busy.asked("summary "); len(reads) != 0 {
		t.Errorf("read before the pane was opened: %q", reads)
	}
}

func TestASourceThatCannotBeReadIsNamedAndTheRestStillShow(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.done.jiraErr = errJiraDown

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, view, "Jira could not be read", "committed abc1234")
}

func TestABracketMovesThePeriodAndReadsItOnceTheKeysRest(t *testing.T) {
	t.Parallel()

	// Arrange
	// Held, [ sends a key at a time; the first comes and goes before its
	// wait is up, so only the period the keys rest on is read. Each moves a
	// period back by its own length, a day.
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)
	passing, _ := pressed(t, opened, "[")

	// Act
	view := typing(t, passing, "[").View().Content

	// Assert
	requireScreen(t, view, "2026-09-13")

	reads := busy.asked("summary git ")
	if len(reads) != 2 || reads[1] != summaryRead("git", time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("git read for %q, want the first period and then only the one the keys rest on", reads)
	}
}

func TestAPeriodThatHasEndedIsNotReadAgainOnComingBack(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)
	busy.goStale()

	// Act
	typing(t, opened, "1", summaryKey)

	// Assert
	if reads := busy.asked("summary git "); len(reads) != 1 {
		t.Errorf("git read %d times, want once: Tuesday has ended", len(reads))
	}
}

func TestShiftYCopiesTheSummaryAsMarkdown(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	typing(t, opened, "Y")

	// Assert
	copied := busy.asked("copy ")
	if len(copied) != 1 || !strings.Contains(copied[0], "# 2026-09-15") ||
		!strings.Contains(copied[0], "- committed abc1234 Fix the token leak") {
		t.Errorf("copied %q, want the summary as Markdown", copied)
	}
}

func TestOOpensTheSelectedItemsLink(t *testing.T) {
	t.Parallel()

	// Arrange
	// The cursor starts on the first item; the pull request is the third.
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	typing(t, opened, "j", "j", "o")

	// Assert
	if browsed := busy.asked("browse "); len(browsed) != 1 || browsed[0] != "browse "+pullURL {
		t.Errorf("browsed %q, want the pull request", browsed)
	}
}
