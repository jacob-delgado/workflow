// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/activity"
)

// errNotYet is a period that starts on a day still to come.
var errNotYet = errors.New("the period starts after today")

// calendarColumn is one of the calendar's columns, as it reads left to right.
type calendarColumn int

const (
	columnYear calendarColumn = iota
	columnMonth
	columnDay
)

// calendarColumns is how many columns the calendar has.
const calendarColumns = 3

// yearsAround is how many years the Year column lists either side of the
// cursor's.
const yearsAround = 2

// weekdays heads the Day column, Monday first.
const weekdays = "Mo Tu We Th Fr Sa Su"

// The widths the Year and Month columns are padded to: a year or a month in
// its brackets, and a gap before the next column.
const (
	yearWidth  = 8
	monthWidth = 7
)

// daysInWeek and monthsInYear are the calendar's own counts.
const (
	daysInWeek   = 7
	monthsInYear = 12
)

// calendar picks the period the Summary shows: a year, a month or a day by
// the column the cursor is in, or a range from a marked day to the cursor's.
// Which value the cursor is on is drawn ‹so›, the values the other columns
// stand on [so], and the days of a marked range in bold, so each reads apart
// without color.
type calendar struct {
	cursor activity.Date
	column calendarColumn
	mark   activity.Date
	marked bool
	err    error
}

var _ overlay = calendar{}

// openCalendar opens the calendar on the last day the Summary shows.
func (m Model) openCalendar() (Model, tea.Cmd) {
	m.overlay = calendar{cursor: m.summaryPeriod().To, column: columnDay}

	return m, nil
}

// view draws the three columns side by side, and the range marked.
func (c calendar) view(kit renderKit, _, _ int) (string, string) {
	years, months, days := c.yearColumn(kit), c.monthColumn(kit), c.dayColumn(kit)

	var lines []string
	for row := range max(len(years), len(months), len(days)) {
		lines = append(lines, strings.TrimRight(
			padded(cell(years, row), yearWidth)+padded(cell(months, row), monthWidth)+cell(days, row), " "))
	}

	if c.marked {
		lines = append(lines, "", "range from "+c.mark.String()+" to the cursor")
	}

	if c.err != nil {
		lines = append(lines, "", kit.failureLine(c.err))
	}

	return "Calendar", strings.Join(lines, "\n")
}

// cell is a column's row, or blank past its end.
func cell(column []string, row int) string {
	if row < len(column) {
		return column[row]
	}

	return ""
}

// padded is text widened with spaces to width.
func padded(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-len([]rune(text))))
}

// shown is a value as its column draws it: ‹so› under the cursor, [so] where
// another column's cursor stands, and plain otherwise.
func (c calendar) shown(kit renderKit, value string, column calendarColumn, on bool) string {
	switch {
	case on && c.column == column:
		return kit.marks.chosenOpen + value + kit.marks.chosenClose
	case on:
		return "[" + value + "]"
	default:
		return " " + value + " "
	}
}

// yearColumn lists the years around the cursor's.
func (c calendar) yearColumn(kit renderKit) []string {
	lines := []string{"Year"}
	for year := c.cursor.Year() - yearsAround; year <= c.cursor.Year()+yearsAround; year++ {
		lines = append(lines, c.shown(kit, strconv.Itoa(year), columnYear, year == c.cursor.Year()))
	}

	return lines
}

// monthColumn lists the months of the year, by number.
func (c calendar) monthColumn(kit renderKit) []string {
	lines := []string{"Month"}
	for month := time.January; month <= time.December; month++ {
		lines = append(lines, c.shown(kit, twoDigit(int(month)), columnMonth, month == c.cursor.Month()))
	}

	return lines
}

// dayColumn lists the cursor's month as weeks, Monday first, the days of a
// marked range in bold.
func (c calendar) dayColumn(kit renderKit) []string {
	month := activity.MonthOf(c.cursor)
	lines := []string{"Day", " " + strings.ReplaceAll(weekdays, " ", "  ")}
	week := strings.Repeat("    ", (int(month.From.Weekday())+daysInWeek-1)%daysInWeek)

	for date := month.From; !month.To.Before(date); date = date.AddDays(1) {
		day := c.shown(kit, twoDigit(date.Day()), columnDay, date == c.cursor)
		if c.inRange(date) {
			day = kit.styles.strong.Render(day)
		}

		week += day
		if date.Weekday() == time.Sunday || date == month.To {
			lines, week = append(lines, strings.TrimRight(week, " ")), ""
		}
	}

	return lines
}

// inRange reports date between the marked day and the cursor's, both in.
func (c calendar) inRange(date activity.Date) bool {
	if !c.marked {
		return false
	}

	from, to := c.mark, c.cursor
	if to.Before(from) {
		from, to = to, from
	}

	return !date.Before(from) && !to.Before(date)
}

// twoDigit is a day or month number written with two digits.
func twoDigit(number int) string {
	text := strconv.Itoa(number)
	if len(text) == 1 {
		return "0" + text
	}

	return text
}

// footer offers moving between and within the columns, marking a range, and
// showing what is chosen.
func (c calendar) footer(keys keyMap) []key.Binding {
	mark := relabel(keys.toggleOption, "range from here")
	if c.marked {
		mark = relabel(keys.toggleOption, "no range")
	}

	bindings := []key.Binding{relabel(keys.nextField, "next column"), relabel(keys.prevField, "previous column")}
	if c.column == columnDay {
		bindings = append(bindings, relabel(keys.cycleLeft, "day"))
	}

	return append(bindings, relabel(keys.up, c.verticalStep()), mark,
		relabel(keys.confirm, "show"), relabel(keys.closeOverlay, escClose))
}

// verticalStep names how far up and down move in the cursor's column.
func (c calendar) verticalStep() string {
	switch c.column {
	case columnYear:
		return "year"
	case columnMonth:
		return "month"
	case columnDay:
		return "week"
	}

	return ""
}

// handleKey moves the cursor, marks a range, or shows what is chosen.
func (c calendar) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return c.show(m)
	case key.Matches(msg, m.keys.nextField):
		c.column = around(c.column, calendarColumns).next()
	case key.Matches(msg, m.keys.prevField):
		c.column = around(c.column, calendarColumns).prev()
	case key.Matches(msg, m.keys.toggleOption):
		c.mark, c.marked = c.cursor, !c.marked
	default:
		c.cursor = c.stepped(m.keys, msg)
	}

	c.err = nil
	m.overlay = c

	return m, nil
}

// stepped is the cursor after a movement key: up and down a step in its
// column, and, in the day column, left and right a day.
func (c calendar) stepped(keys keyMap, msg tea.KeyPressMsg) activity.Date {
	switch {
	case key.Matches(msg, keys.up):
		return c.moved(-1)
	case key.Matches(msg, keys.down):
		return c.moved(1)
	case c.column == columnDay && key.Matches(msg, keys.cycleLeft):
		return c.cursor.AddDays(-1)
	case c.column == columnDay && key.Matches(msg, keys.cycleRight):
		return c.cursor.AddDays(1)
	}

	return c.cursor
}

// moved is the cursor a step on in its column: a year, a month, or — the day
// column being drawn as a month of weeks — a week.
func (c calendar) moved(step int) activity.Date {
	switch c.column {
	case columnYear:
		return c.cursor.AddMonths(monthsInYear * step)
	case columnMonth:
		return c.cursor.AddMonths(step)
	case columnDay:
		return c.cursor.AddDays(daysInWeek * step)
	}

	return c.cursor
}

// chosen is the period picked: the marked range, or what the cursor's column
// names.
func (c calendar) chosen() (activity.Period, error) {
	if c.marked {
		from, to := c.mark, c.cursor
		if to.Before(from) {
			from, to = to, from
		}

		return activity.NewPeriod(from, to)
	}

	switch c.column {
	case columnYear:
		return activity.YearOf(c.cursor), nil
	case columnMonth:
		return activity.MonthOf(c.cursor), nil
	case columnDay:
		return activity.Period{From: c.cursor, To: c.cursor}, nil
	}

	return activity.Period{From: c.cursor, To: c.cursor}, nil
}

// show closes the calendar on the period chosen and reads it, or keeps it open
// on why the period cannot be shown.
func (c calendar) show(m Model) (Model, tea.Cmd) {
	period, err := c.chosen()
	if err == nil && m.today().Before(period.From) {
		err = errNotYet
	}

	if err != nil {
		c.err = err
		m.overlay = c

		return m, nil
	}

	m = m.closeOverlay()
	m.summary.period, m.summary.chosen, m.summary.selected, m.summary.scroll = period, true, 0, 0

	return m.readSummary()
}
