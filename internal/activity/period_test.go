// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"encoding/json"
	"errors"
	"os"
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

// datesFile is the calendar's dates, the cases the web's calendar answers to
// as well.
const datesFile = "testdata/civil_dates.json"

type periodCase struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type datesCorpus struct {
	About string `json:"about"`
	Parse []struct {
		Name string `json:"name"`
		Text string `json:"text"`
		OK   bool   `json:"ok"`
	} `json:"parse"`
	AddDays []struct {
		Name string `json:"name"`
		From string `json:"from"`
		Days int    `json:"days"`
		Want string `json:"want"`
	} `json:"add_days"`
	AddMonths []struct {
		Name   string `json:"name"`
		From   string `json:"from"`
		Months int    `json:"months"`
		Want   string `json:"want"`
	} `json:"add_months"`
	Whole []struct {
		Name  string     `json:"name"`
		Date  string     `json:"date"`
		Month periodCase `json:"month"`
		Year  periodCase `json:"year"`
	} `json:"whole"`
	Step []struct {
		Name  string     `json:"name"`
		From  string     `json:"from"`
		To    string     `json:"to"`
		Steps int        `json:"steps"`
		Days  int        `json:"days"`
		Want  periodCase `json:"want"`
	} `json:"step"`
}

// readDates is the shared cases, refusing a field this side would ignore so a
// case cannot pin one copy and pass the other unread.
func readDates(t *testing.T) datesCorpus {
	t.Helper()

	file, err := os.Open(datesFile)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var read datesCorpus

	err = decoder.Decode(&read)
	if err != nil {
		t.Fatalf("%s: %v", datesFile, err)
	}

	return read
}

// shown is a period as the cases write it.
func shown(period activity.Period) periodCase {
	return periodCase{From: period.From.String(), To: period.To.String()}
}

func TestADateIsReadAndWrittenAsYearMonthDay(t *testing.T) {
	t.Parallel()

	for _, testCase := range readDates(t).Parse {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Act
			date, err := activity.ParseDate(testCase.Text)

			// Assert
			if (err == nil) != testCase.OK || (testCase.OK && date.String() != testCase.Text) {
				t.Errorf("ParseDate(%q) = %v, %v; want ok %v and the same text back", testCase.Text, date, err, testCase.OK)
			}
		})
	}
}

func TestMovingByDaysCrossesMonthsAndYears(t *testing.T) {
	t.Parallel()

	for _, testCase := range readDates(t).AddDays {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Act
			moved := day(t, testCase.From).AddDays(testCase.Days)

			// Assert
			if moved.String() != testCase.Want {
				t.Errorf("%s + %d days = %s, want %s", testCase.From, testCase.Days, moved, testCase.Want)
			}
		})
	}
}

func TestMovingByMonthsKeepsTheDayWhereTheMonthHasIt(t *testing.T) {
	t.Parallel()

	for _, testCase := range readDates(t).AddMonths {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Act
			moved := day(t, testCase.From).AddMonths(testCase.Months)

			// Assert
			if moved.String() != testCase.Want {
				t.Errorf("%s + %d months = %s, want %s", testCase.From, testCase.Months, moved, testCase.Want)
			}
		})
	}
}

func TestADatesMonthAndYearAreWholePeriods(t *testing.T) {
	t.Parallel()

	for _, testCase := range readDates(t).Whole {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			date := day(t, testCase.Date)

			// Act
			month, year := activity.MonthOf(date), activity.YearOf(date)

			// Assert
			if shown(month) != testCase.Month || shown(year) != testCase.Year {
				t.Errorf("MonthOf = %s, YearOf = %s; want %+v and %+v", month, year, testCase.Month, testCase.Year)
			}
		})
	}
}

func TestAPeriodStepsByItsOwnLengthAndAWholeMonthOrYearByItself(t *testing.T) {
	t.Parallel()

	for _, step := range readDates(t).Step {
		t.Run(step.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			period, err := activity.NewPeriod(day(t, step.From), day(t, step.To))
			if err != nil {
				t.Fatalf("NewPeriod: %v", err)
			}

			// Act
			stepped := period.Step(step.Steps)

			// Assert
			if shown(stepped) != step.Want || period.Days() != step.Days {
				t.Errorf("Step(%d) = %s of %d days, want %+v of %d", step.Steps, stepped, period.Days(), step.Want, step.Days)
			}
		})
	}
}

// askedOn is the Monday a period is asked on, and the others the days of the
// week before it that a period names.
const (
	askedOn   = "2026-10-05"
	monday    = "2026-09-28"
	wednesday = "2026-09-30"
	thursday  = "2026-10-01"
)

func TestThePeriodAskedIsFromToTheDayAloneOrThePreviousWorkingDay(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from, to string
		want     string
	}{
		"neither day":      {from: "", to: "", want: "2026-10-02 to 2026-10-04"},
		"from alone":       {from: wednesday, to: "", want: wednesday + " to " + wednesday},
		"to alone":         {from: "", to: "2026-09-29", want: "2026-09-29 to 2026-09-29"},
		"from through to":  {from: monday, to: thursday, want: monday + " to " + thursday},
		"backwards is not": {from: thursday, to: monday, want: "refused"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			period, err := activity.PeriodAsked(testCase.from, testCase.to, day(t, askedOn))

			// Assert
			got := period.From.String() + " to " + period.To.String()
			if err != nil {
				got = "refused"
			}

			if got != testCase.want {
				t.Errorf("PeriodAsked(%q, %q) = %s (%v), want %s", testCase.from, testCase.to, got, err, testCase.want)
			}
		})
	}
}

func TestAPeriodAskedWithADayThatIsNoDateIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ from, to string }{
		"from":  {from: "tomorrow", to: thursday},
		"to":    {from: thursday, to: "10/02"},
		"alone": {from: "", to: "yesterday"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := activity.PeriodAsked(testCase.from, testCase.to, day(t, askedOn))

			// Assert
			if !errors.Is(err, activity.ErrNotADate) {
				t.Errorf("PeriodAsked(%q, %q) = %v, want ErrNotADate", testCase.from, testCase.to, err)
			}
		})
	}
}
