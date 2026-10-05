// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
)

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
		"leap day kept": {text: "2028-02-29", ok: true},
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
