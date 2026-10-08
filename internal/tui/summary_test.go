// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// summaryKey jumps to the Summary pane, whose list is in its detail, so its
// focus shows on the detail's title, summaryTitle.
const (
	summaryKey   = "8"
	summaryTitle = "Summary"
)

var (
	errJiraDown     = errors.New("jira is down")
	errTokenCommand = errors.New("the token command exited 1")
)

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

func TestASourceThatIsNotSetUpSaysHowToSetItUpRatherThanFailing(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.done.jiraErr = fmt.Errorf("%w: %w", jira.ErrNoCredential, errTokenCommand)

	// Act
	view := typing(t, busy.live(t, 160, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, view, "○ Jira is not set up, so it was left out", "jira.token", "committed abc1234")

	if strings.Contains(view, "could not be read") || strings.Contains(view, "✗ Jira") {
		t.Errorf("a Jira that is not set up reads as a failure:\n%s", view)
	}
}

func TestARepositoryThatCannotBeReadIsNamedBesideTheOthersCommits(t *testing.T) {
	t.Parallel()

	// Arrange
	// A timeout is worded the TUI's own way, which must not lose which
	// repository timed out.
	busy := summaryWorld()
	busy.done.repositories = []loop.RepositoryCommits{
		{Repository: "acme/api", Commits: busy.done.commits},
		{Repository: "acme/web", Failed: proc.ErrTimedOut},
	}

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, view, "Git could not be read in acme/web", "committed acme/api@abc1234")
}

func TestAFailureWrappingTwoCausesIsOneNote(t *testing.T) {
	t.Parallel()

	// Arrange
	// fmt.Errorf with two %w is one failure, though errors can unwrap it in two.
	busy := summaryWorld()
	busy.done.jiraErr = fmt.Errorf("%w: %w", errJiraDown, errTokenCommand)

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey).View().Content

	// Assert
	if notes := strings.Count(view, "Jira could not be read"); notes != 1 {
		t.Errorf("Jira's failure is noted %d times, want once:\n%s", notes, view)
	}
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

func TestASourceThatFailedIsReadAgainOnComingBack(t *testing.T) {
	t.Parallel()

	// Arrange
	// A period that has ended is kept only once every source has said what it
	// did there; one that failed has not, so coming back asks it again.
	busy := summaryWorld()
	busy.done.jiraErr = errJiraDown
	opened := typing(t, busy.live(t, 120, 40), summaryKey)
	busy.goStale()

	// Act
	typing(t, opened, "1", summaryKey)

	// Assert
	if reads := busy.asked("summary jira "); len(reads) != 2 {
		t.Errorf("Jira read %d times, want twice: it failed the first time", len(reads))
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

func TestLaterMovesOnButNeverPastToday(t *testing.T) {
	t.Parallel()

	// Arrange
	// Tuesday moves on to today, Wednesday; Thursday has not happened.
	opened := typing(t, summaryWorld().live(t, 120, 40), summaryKey)

	// Act
	view := typing(t, opened, "]", "]").View().Content

	// Assert
	requireScreen(t, view, "2026-09-16")
	refuseScreen(t, view, "2026-09-17")
}

func TestTodayIsReadAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	typing(t, opened, "t")

	// Assert
	if reads := busy.asked(summaryRead("git", time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))); len(reads) != 1 {
		t.Errorf("today read %d times, want once, without waiting for the keys to rest", len(reads))
	}
}

func TestRefreshReadsAPeriodThatHasEndedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	typing(t, opened, "r")

	// Assert
	if reads := busy.asked("summary git "); len(reads) != 2 {
		t.Errorf("git read %d times, want twice: r reads even a period that has ended", len(reads))
	}
}

func TestYCopiesTheSelectedItemsLink(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	typing(t, opened, "j", "j", "y")

	// Assert
	if copied := busy.asked("copy "); len(copied) != 1 || copied[0] != "copy "+pullURL {
		t.Errorf("copied %q, want the pull request's link", copied)
	}
}

func TestASourceWithMoreThanItGaveSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.done.jira.Truncated = true

	// Act
	view := typing(t, busy.live(t, 120, 40), summaryKey).View().Content

	// Assert
	requireScreen(t, view, "Jira had more than this shows.")
}

func TestTheRailCountsOneThingDoneInTheSingular(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	busy.done.jira, busy.done.forge = jira.Activity{}, forge.Activity{}
	opened := typing(t, busy.live(t, 120, 40), summaryKey)

	// Act
	view := typing(t, opened, "1").View().Content

	// Assert
	requireScreen(t, view, "2026-09-15: 1 thing done")
}

func TestAnAnswerForAPeriodSinceLeftIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	// r reads the period again; its answers are held back while [ moves to the
	// day before and reads that, and git has since moved on to another commit.
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)
	rereading, staleReads := pressed(t, opened, "r")
	stepped := typing(t, rereading, "[")
	busy.done.commits = []gitrepo.DatedCommit{{Short: "def5678", Subject: "Read for a period left", Authored: tuesday(10)}}

	// Act
	view := drain(t, stepped, staleReads).View().Content

	// Assert
	requireScreen(t, view, "2026-09-14", "Fix the token leak")
	refuseScreen(t, view, "Read for a period left")
}

func TestAKeysRestForAPeriodSinceLeftReadsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first [ comes to rest only after the second has moved on and read
	// the period it rests on.
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), summaryKey)
	passing, firstRest := pressed(t, opened, "[")
	rested := typing(t, passing, "[")

	// Act
	view := drain(t, rested, firstRest).View().Content

	// Assert
	requireScreen(t, view, "2026-09-13")

	if reads := busy.asked("summary git "); len(reads) != 2 {
		t.Errorf("git read for %q, want the first period and the one the keys rest on, nothing more", reads)
	}
}
