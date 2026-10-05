// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
)

// leapDay is the leap day the cases reach.
const leapDay = "2028-02-29"

// day is a civil date, written as a test reads it.
func day(t *testing.T, text string) activity.Date {
	t.Helper()

	date, err := activity.ParseDate(text)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", text, err)
	}

	return date
}

func TestThePreviousWorkingDayReachesBackOverTheWeekend(t *testing.T) {
	t.Parallel()

	const (
		friday   = "2026-10-02"
		saturday = "2026-10-03"
		sunday   = "2026-10-04"
		monday   = "2026-10-05"
	)

	cases := map[string]struct {
		today    string
		from, to string
	}{
		"a Tuesday is Monday":                     {today: "2026-10-06", from: monday, to: monday},
		"a Monday is Friday through Sunday":       {today: monday, from: friday, to: sunday},
		"a Sunday is Friday through Saturday":     {today: sunday, from: friday, to: saturday},
		"a Saturday is Friday":                    {today: saturday, from: friday, to: friday},
		"the first of a year reaches the last":    {today: "2027-01-01", from: "2026-12-31", to: "2026-12-31"},
		"a Monday the first reaches into a month": {today: "2026-06-01", from: "2026-05-29", to: "2026-05-31"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			period := activity.PreviousWorkingDay(day(t, testCase.today))

			// Assert
			if period.From != day(t, testCase.from) || period.To != day(t, testCase.to) {
				t.Errorf("PreviousWorkingDay(%s) = %s, want %s to %s", testCase.today, period, testCase.from, testCase.to)
			}
		})
	}
}

func TestAPeriodRunsForwardAndNoLongerThanAYearAndADay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from, to string
		want     error
	}{
		"one day":           {from: "2026-09-30", to: "2026-09-30"},
		"a leap year whole": {from: "2028-01-01", to: "2028-12-31"},
		"backwards":         {from: "2026-10-02", to: "2026-10-01", want: activity.ErrBackwards},
		"too long":          {from: "2026-01-01", to: "2027-01-02", want: activity.ErrTooLong},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := activity.NewPeriod(day(t, testCase.from), day(t, testCase.to))

			// Assert
			if !errors.Is(err, testCase.want) {
				t.Errorf("NewPeriod(%s, %s) = %v, want %v", testCase.from, testCase.to, err, testCase.want)
			}
		})
	}
}

func TestAPeriodsBoundsAreItsDaysMidnightsWhereverTheClocksChange(t *testing.T) {
	t.Parallel()

	// Arrange
	// New York's clocks fall back on 2026-11-01, a 25-hour day.
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}

	period, err := activity.NewPeriod(day(t, "2026-11-01"), day(t, "2026-11-01"))
	if err != nil {
		t.Fatalf("NewPeriod: %v", err)
	}

	// Act
	start, end := period.Bounds(newYork)

	// Assert
	if end.Sub(start) != 25*time.Hour || start.Hour() != 0 || end.In(newYork).Hour() != 0 {
		t.Errorf("Bounds = %v to %v, want the 25 hours from one midnight to the next", start, end)
	}
}

func TestADaysStartIsTheFirstHourItsClocksShowWhenTheySkipMidnight(t *testing.T) {
	t.Parallel()

	// Arrange
	// Santiago's clocks jump from 23:59:59 on 2025-09-06 to 01:00 on the 7th,
	// and Go places the midnight that never happened an hour before, on the 6th.
	santiago, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}

	// Act
	start := day(t, "2025-09-07").Start(santiago)

	// Assert
	if got := start.In(santiago); got.Day() != 7 || got.Hour() != 1 {
		t.Errorf("Start = %v, want 01:00 on the 7th, the day's first hour", got)
	}
}

func TestADayTheClocksSkipStartsWhereTheNextDayDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	// Samoa crossed the date line by skipping 2011-12-30 entirely.
	apia, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}

	// Act
	start := day(t, "2011-12-30").Start(apia)

	// Assert
	if got := start.In(apia); got.Day() != 31 || got.Hour() != 0 {
		t.Errorf("Start = %v, want midnight on the 31st, the next day there was", got)
	}
}

func TestADateIsReadAndWrittenAsYearMonthDay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text string
		ok   bool
	}{
		"a date":        {text: "2026-02-28", ok: true},
		"no such day":   {text: "2026-02-30", ok: false},
		"not a date":    {text: "yesterday", ok: false},
		"with a time":   {text: "2026-02-28T10:00:00Z", ok: false},
		"leap day kept": {text: leapDay, ok: true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			date, err := activity.ParseDate(testCase.text)

			// Assert
			if (err == nil) != testCase.ok || (testCase.ok && date.String() != testCase.text) {
				t.Errorf("ParseDate(%q) = %v, %v; want ok %v and the same text back", testCase.text, date, err, testCase.ok)
			}
		})
	}
}

func TestMovingByMonthsKeepsTheDayWhereTheMonthHasIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from   string
		months int
		want   string
	}{
		"a day every month has":     {from: "2026-01-15", months: 1, want: "2026-02-15"},
		"the 31st into a short one": {from: "2026-01-31", months: 1, want: "2026-02-28"},
		"into a leap February":      {from: "2028-01-31", months: 1, want: leapDay},
		"back over a year's end":    {from: "2026-01-31", months: -2, want: "2025-11-30"},
		"a year on from a leap day": {from: leapDay, months: 12, want: "2029-02-28"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			moved := day(t, testCase.from).AddMonths(testCase.months)

			// Assert
			if moved != day(t, testCase.want) {
				t.Errorf("%s + %d months = %s, want %s", testCase.from, testCase.months, moved, testCase.want)
			}
		})
	}
}

func TestADatesMonthAndYearAreWholePeriods(t *testing.T) {
	t.Parallel()

	// Arrange
	date := day(t, "2028-02-10")

	// Act
	month, year := activity.MonthOf(date), activity.YearOf(date)

	// Assert
	if month.String() != "2028-02-01 to 2028-02-29" || year.String() != "2028-01-01 to 2028-12-31" {
		t.Errorf("MonthOf = %s, YearOf = %s; want the leap February and the whole year", month, year)
	}
}

func TestAPeriodStepsByItsOwnLengthAndAWholeMonthOrYearByItself(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from, to string
		steps    int
		want     string
	}{
		"days, earlier":          {from: "2025-06-11", to: "2025-06-13", steps: -1, want: "2025-06-08 to 2025-06-10"},
		"days, later":            {from: "2025-06-20", to: "2025-06-20", steps: 1, want: "2025-06-21"},
		"a whole month, earlier": {from: "2025-10-01", to: "2025-10-31", steps: -1, want: "2025-09-01 to 2025-09-30"},
		"a whole month, later":   {from: "2025-01-01", to: "2025-01-31", steps: 1, want: "2025-02-01 to 2025-02-28"},
		"a whole year, earlier":  {from: "2024-01-01", to: "2024-12-31", steps: -1, want: "2023-01-01 to 2023-12-31"},
	}

	for name, step := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			period, err := activity.NewPeriod(day(t, step.from), day(t, step.to))
			if err != nil {
				t.Fatalf("NewPeriod: %v", err)
			}

			// Act
			stepped := period.Step(step.steps)

			// Assert
			if got := stepped.String(); got != step.want {
				t.Errorf("Step(%d) = %s, want %s", step.steps, got, step.want)
			}
		})
	}
}
