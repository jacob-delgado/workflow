// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"fmt"
	"time"
)

// Year is a year's months that hold items. Year, Month, Day and Hour group
// items as the summary nests them, oldest first at every level, with no
// minutes.
type Year struct {
	Year   int
	Months []Month
}

// Month is a month's days that hold items.
type Month struct {
	Month time.Month
	Days  []Day
}

// Day is a day's hours that hold items.
type Day struct {
	Date  Date
	Hours []Hour
}

// Hour is an hour on the clock and its items. Label is the hour as the clock
// showed it; on a day the clocks are turned back, the hour shown twice is two
// hours, each labeled with its zone so they read apart.
type Hour struct {
	Label  string
	Items  []Item
	clock  int
	offset int
	zone   string
}

// Group nests items, oldest first, by the year, month, day and hour each
// falls on in loc.
func Group(items []Item, loc *time.Location) []Year {
	var years []Year

	for _, item := range Merge(items) {
		years = placed(years, item, item.At.In(loc))
	}

	for yearIndex := range years {
		for monthIndex := range years[yearIndex].Months {
			for dayIndex := range years[yearIndex].Months[monthIndex].Days {
				label(&years[yearIndex].Months[monthIndex].Days[dayIndex])
			}
		}
	}

	return years
}

// placed is years with item added under the hour local falls in. Items come
// oldest first, so an item's year, month, day and hour are either the last
// ones already there or new ones after them.
func placed(years []Year, item Item, local time.Time) []Year {
	if len(years) == 0 || years[len(years)-1].Year != local.Year() {
		years = append(years, Year{Year: local.Year()})
	}

	year := &years[len(years)-1]
	if len(year.Months) == 0 || year.Months[len(year.Months)-1].Month != local.Month() {
		year.Months = append(year.Months, Month{Month: local.Month()})
	}

	month := &year.Months[len(year.Months)-1]
	if date := DateOf(local); len(month.Days) == 0 || month.Days[len(month.Days)-1].Date != date {
		month.Days = append(month.Days, Day{Date: date})
	}

	day := &month.Days[len(month.Days)-1]
	zone, offset := local.Zone()

	if last := len(day.Hours) - 1; last < 0 || day.Hours[last].clock != local.Hour() || day.Hours[last].offset != offset {
		day.Hours = append(day.Hours, Hour{clock: local.Hour(), offset: offset, zone: zone})
	}

	hour := &day.Hours[len(day.Hours)-1]
	hour.Items = append(hour.Items, item)

	return years
}

// label names each of a day's hours as the clock showed it, adding the zone
// to an hour the clock showed twice.
func label(day *Day) {
	shown := map[int]int{}
	for _, hour := range day.Hours {
		shown[hour.clock]++
	}

	for index := range day.Hours {
		hour := &day.Hours[index]

		hour.Label = fmt.Sprintf("%02d:00", hour.clock)
		if shown[hour.clock] > 1 {
			hour.Label += " " + hour.zone
		}
	}
}
