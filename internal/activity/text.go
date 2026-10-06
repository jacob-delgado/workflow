// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package activity

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Summary is what every source read for a period.
type Summary struct {
	Period Period
	Reads  []Read
}

// Items are every source's items, merged.
func (s Summary) Items() []Item {
	lists := make([][]Item, 0, len(s.Reads))
	for _, read := range s.Reads {
		lists = append(lists, read.Items)
	}

	return Merge(lists...)
}

// Text is the summary as Markdown to copy: the period, then its items nested
// by year, month, day and hour in loc, then a line for each source that could
// not be read or had more than it gave.
func (s Summary) Text(loc *time.Location) string {
	lines := []string{"# " + s.Period.String(), ""}

	years := Group(s.Items(), loc)
	if len(years) == 0 {
		lines = append(lines, "Nothing was done in this period.", "")
	}

	for _, year := range years {
		lines = append(lines, yearLines(year)...)
	}

	lines = append(lines, s.notes()...)

	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

// yearLines are a year's headings and items, each heading a level below the
// one it is under.
func yearLines(year Year) []string {
	lines := []string{"## " + strconv.Itoa(year.Year), ""}

	for _, month := range year.Months {
		lines = append(lines, "### "+strconv.Itoa(year.Year)+"-"+twoDigits(int(month.Month))+" "+month.Month.String(), "")

		for _, day := range month.Days {
			lines = append(lines, "#### "+day.Date.String()+" "+day.Date.Weekday().String(), "")

			for _, hour := range day.Hours {
				lines = append(lines, "##### "+hour.Label, "")
				lines = append(lines, itemLines(hour.Items)...)
				lines = append(lines, "")
			}
		}
	}

	return lines
}

// itemLines are an hour's items as a list, each title's Markdown escaped: a
// title is anyone's to write — a Jira summary, a pull request's title, a
// commit's subject — and the text is Markdown that Teams and Discord render,
// where "[Click](https://evil)" unescaped would be a live link.
func itemLines(items []Item) []string {
	escape := markdownEscaper()

	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+strings.Join(nonEmpty(item.Kind.Verb(), item.Ref, escape.Replace(item.Title)), " "))
	}

	return lines
}

// markdownEscaper backslash-escapes every character Markdown reads as markup
// inside a line, as messaging escapes an announcement's values.
func markdownEscaper() *strings.Replacer {
	return strings.NewReplacer(
		`\`, `\\`, "`", "\\`", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "*", "\\*", "_", "\\_",
		"~", "\\~", "|", "\\|", "#", "\\#", ">", "\\>",
	)
}

// notes say which sources could not be read, and which had more than they
// gave, in the order they were read.
func (s Summary) notes() []string {
	var notes []string

	for _, read := range s.Reads {
		name := read.Source.Title()

		if read.Failed != nil {
			notes = append(notes, name+" could not be read.")
		}

		if read.Truncated {
			notes = append(notes, name+" had more than this shows.")
		}
	}

	return notes
}

// nonEmpty is words without the empty ones.
func nonEmpty(words ...string) []string {
	kept := words[:0]

	for _, word := range words {
		if word != "" {
			kept = append(kept, word)
		}
	}

	return kept
}

// twoDigits is a month's number written with two digits.
func twoDigits(number int) string {
	return fmt.Sprintf("%02d", number)
}
