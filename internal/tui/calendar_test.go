// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// calendarOpen are the keys that open the calendar on the Summary pane, on
// Tuesday, the day it shows.
func calendarOpen() []string { return []string{summaryKey, "c"} }

func TestTheCalendarOpensOnTheDayShownInItsDayColumn(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, summaryWorld().live(t, 120, 40), summaryKey, "c").View().Content

	// Assert
	// The cursor's value is drawn ‹so›, the others' choices [so], so the
	// column with the cursor reads apart without color.
	requireScreen(t, view, "Calendar", "Year", "Month", "Day", "[2026]", "[09]", "‹15›")

	// September 2026 starts on a Tuesday, under Tu.
	weekdays, firstWeek := lineHolding(view, " Mo  Tu "), lineHolding(view, " 01  02 ")
	if column(weekdays, "Tu") != column(firstWeek, "01") {
		t.Errorf("the first falls under %q, want Tu:\n%s\n%s", weekdays, weekdays, firstWeek)
	}
}

// column is where text starts on a line, counted in characters.
func column(line, text string) int {
	return utf8.RuneCountInString(line[:max(0, strings.Index(line, text))])
}

// lineHolding is the screen's first line, plain, that holds text.
func lineHolding(view, text string) string {
	for line := range strings.Lines(plain(view)) {
		if strings.Contains(line, text) {
			return line
		}
	}

	return ""
}

func TestEnterShowsWhatTheColumnWithTheCursorNames(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"a day":       {keys: []string{"k", keyEnter}, want: "2026-09-14"},
		"its month":   {keys: []string{keyShiftTab, keyEnter}, want: "2026-09-01 to 2026-09-30"},
		"its year":    {keys: []string{keyShiftTab, keyShiftTab, keyEnter}, want: "2026-01-01 to 2026-12-31"},
		"a range":     {keys: []string{keySpace, "k", "k", "k", keyEnter}, want: "2026-09-12 to 2026-09-15"},
		"a range too": {keys: []string{"k", keySpace, "j", "j", keyEnter}, want: "2026-09-14 to 2026-09-16"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			busy := summaryWorld()
			opened := typing(t, busy.live(t, 120, 40), calendarOpen()...)

			// Act
			view := typing(t, opened, testCase.keys...).View().Content

			// Assert
			requireScreen(t, view, "8 Summary", testCase.want)
			refuseScreen(t, view, "Calendar")
		})
	}
}

func TestEscClosesTheCalendarAndKeepsThePeriod(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), calendarOpen()...)

	// Act
	view := typing(t, opened, "k", keyEsc).View().Content

	// Assert
	requireScreen(t, view, "2026-09-15")
	refuseScreen(t, view, "Calendar", "2026-09-14")
}

func TestTheCalendarRefusesADayThatHasNotHappened(t *testing.T) {
	t.Parallel()

	// Arrange
	// Today is Wednesday the 16th; nothing has been done on the 17th yet, and
	// ] refuses to move there too.
	busy := summaryWorld()
	opened := typing(t, busy.live(t, 120, 40), calendarOpen()...)

	// Act
	view := typing(t, opened, "j", "j", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Calendar", "after today")

	if reads := busy.asked(summaryRead("git", time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC))); len(reads) != 0 {
		t.Errorf("read %v, want nothing read for a day to come", reads)
	}
}

func TestTabMovesToTheYearColumnWhereAStepIsAYear(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, summaryWorld().live(t, 120, 40), calendarOpen()...)

	// Act
	view := typing(t, opened, "tab", "k", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "2025-01-01 to 2025-12-31")
}

func TestSpaceAgainUnmarksTheRange(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, summaryWorld().live(t, 120, 40), calendarOpen()...)

	// Act
	view := typing(t, opened, keySpace, "k", keySpace, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "8 Summary", "2026-09-14")
	refuseScreen(t, view, "2026-09-14 to")
}
