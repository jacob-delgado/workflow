// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package activity is what was done over a period, read back from the places
// the work left a trace — git, Taskwarrior, Jira and the forge — and grouped
// by year, month, day and hour. It holds no clock and does no I/O: a caller
// says what today is, and hands it what each source read.
package activity

import (
	"errors"
	"fmt"
	"time"
)

// MaxDays is the longest period read at once: a year, leap day included.
const MaxDays = 366

// day is a calendar day's length away from any clock change, as the dates
// here are counted at noon in UTC.
const day = 24 * time.Hour

// dateLayout is how a date is read and written: year, month, day.
const dateLayout = time.DateOnly

var (
	// ErrBackwards is a period that ends before it starts.
	ErrBackwards = errors.New("the period ends before it starts")
	// ErrTooLong is a period longer than MaxDays.
	ErrTooLong = errors.New("the period is longer than a year and a day")
	// ErrNotADate is text that is not a date written year-month-day.
	ErrNotADate = errors.New("not a date written as YYYY-MM-DD")
)

// Trade-off TRADE-33: the web's calendar works these dates out again in
// civilDate.ts, so it moves with no request. Both copies answer to the one case
// file testdata/civil_dates.json.

// Date is a day on the calendar, wherever it is: no time and no zone, so a
// period means the same days whichever clock reads it.
type Date struct {
	year  int
	month time.Month
	day   int
}

// DateOf is the calendar day an instant falls on, in its own location.
func DateOf(instant time.Time) Date {
	year, month, day := instant.Date()

	return Date{year: year, month: month, day: day}
}

// ParseDate reads a date written year-month-day, refusing a day the month
// does not have.
func ParseDate(text string) (Date, error) {
	parsed, err := time.Parse(dateLayout, text)
	if err != nil {
		return Date{}, fmt.Errorf("%w: %q", ErrNotADate, text)
	}

	return DateOf(parsed), nil
}

// String is the date written year-month-day.
func (d Date) String() string { return d.noon().Format(dateLayout) }

// Year is the date's year.
func (d Date) Year() int { return d.year }

// Month is the date's month.
func (d Date) Month() time.Month { return d.month }

// Day is the date's day of the month.
func (d Date) Day() int { return d.day }

// Weekday is the day of the week the date falls on.
func (d Date) Weekday() time.Weekday { return d.noon().Weekday() }

// AddDays is the date days later, or earlier when days is negative.
func (d Date) AddDays(days int) Date { return DateOf(d.noon().AddDate(0, 0, days)) }

// AddMonths is the date months later, or earlier when months is negative, on
// the same day of the month, or the month's last where it has no such day.
func (d Date) AddMonths(months int) Date {
	first := time.Date(d.year, d.month, 1, 12, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	year, month, _ := first.Date()

	return Date{year: year, month: month, day: min(d.day, daysIn(year, month))}
}

// daysIn is how many days a month of a year has.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 12, 0, 0, 0, time.UTC).Day()
}

// Before reports d earlier on the calendar than other.
func (d Date) Before(other Date) bool { return d.noon().Before(other.noon()) }

// Start is the date's first instant in loc: its midnight, or the first hour
// the clocks show when they skip midnight. Go places a midnight that never
// happened an hour before it, on the day before, so it is stepped forward.
func (d Date) Start(loc *time.Location) time.Time {
	start := time.Date(d.year, d.month, d.day, 0, 0, 0, 0, loc)
	for DateOf(start).Before(d) {
		start = start.Add(time.Hour)
	}

	return start
}

// noon is the date at noon in UTC, an instant no clock change moves off it.
func (d Date) noon() time.Time { return time.Date(d.year, d.month, d.day, 12, 0, 0, 0, time.UTC) }

// Period is the days from From through To, both included.
type Period struct {
	From Date
	To   Date
}

// NewPeriod is the days from through to, refusing one that runs backwards or
// is longer than MaxDays.
func NewPeriod(from, to Date) (Period, error) {
	period := Period{From: from, To: to}

	switch {
	case to.Before(from):
		return Period{}, fmt.Errorf("%w: %s", ErrBackwards, period)
	case period.Days() > MaxDays:
		return Period{}, fmt.Errorf("%w: %s", ErrTooLong, period)
	}

	return period, nil
}

// PeriodAsked is the period first and last name, each a date written
// year-month-day or empty when it is not given: both days, one day alone, or
// the previous working day before today when neither is. Every surface reads
// a period asked for this way, so each defaults to the same days.
func PeriodAsked(first, last string, today Date) (Period, error) {
	switch {
	case first == "" && last == "":
		return PreviousWorkingDay(today), nil
	case first == "":
		return dayAlone(last)
	case last == "":
		return dayAlone(first)
	}

	from, err := ParseDate(first)
	if err != nil {
		return Period{}, err
	}

	through, err := ParseDate(last)
	if err != nil {
		return Period{}, err
	}

	return NewPeriod(from, through)
}

// dayAlone is the period of one day, written year-month-day.
func dayAlone(text string) (Period, error) {
	date, err := ParseDate(text)

	return Period{From: date, To: date}, err
}

// MonthOf is the whole month a date falls in.
func MonthOf(date Date) Period {
	return Period{
		From: Date{year: date.year, month: date.month, day: 1},
		To:   Date{year: date.year, month: date.month, day: daysIn(date.year, date.month)},
	}
}

// YearOf is the whole year a date falls in.
func YearOf(date Date) Period {
	return Period{
		From: Date{year: date.year, month: time.January, day: 1},
		To:   Date{year: date.year, month: time.December, day: daysIn(date.year, time.December)},
	}
}

// PreviousWorkingDay is the last working day before today through yesterday,
// so a Monday reads back Friday and the weekend after it.
func PreviousWorkingDay(today Date) Period {
	yesterday := today.AddDays(-1)

	from := yesterday
	for from.Weekday() == time.Saturday || from.Weekday() == time.Sunday {
		from = from.AddDays(-1)
	}

	return Period{From: from, To: yesterday}
}

// Days is how many days the period holds.
func (p Period) Days() int { return int(p.To.noon().Sub(p.From.noon())/day) + 1 }

// monthsInYear is how many months a whole year steps by.
const monthsInYear = 12

// Step is the period moved on by steps of its own length, or back when steps
// is negative. A whole year steps a year, and a whole month a month, so a month
// stays a month whatever its days.
func (p Period) Step(steps int) Period {
	switch p {
	case YearOf(p.From):
		return YearOf(p.From.AddMonths(steps * monthsInYear))
	case MonthOf(p.From):
		return MonthOf(p.From.AddMonths(steps))
	default:
		days := steps * p.Days()

		return Period{From: p.From.AddDays(days), To: p.To.AddDays(days)}
	}
}

// Bounds are the period's first instant in loc and the first instant after
// it, so an instant is in the period when it is at or after the one and
// before the other.
func (p Period) Bounds(loc *time.Location) (time.Time, time.Time) {
	return p.From.Start(loc), p.To.AddDays(1).Start(loc)
}

// String is the period named by its days: one, or its first and last.
func (p Period) String() string {
	if p.From == p.To {
		return p.From.String()
	}

	return p.From.String() + " to " + p.To.String()
}
