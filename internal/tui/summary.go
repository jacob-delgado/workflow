// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// summaryRest is how long [ and ] wait for the keys to rest before reading
// the period they rest on, so a held key reads one period, not each it passes.
const summaryRest = 300 * time.Millisecond

// summaryIndent sets an hour's items in under it.
const summaryIndent = "  "

// Trade-off TRADE-32: what was done is read back from each source every time
// the pane shows a period, never kept.

// summaryState is the Summary pane: the period shown, what each source has
// answered for it and which are still to, and the item the cursor is on.
type summaryState struct {
	period activity.Period
	chosen bool
	reads  []activity.Read
	asking []activity.Source
	// reading counts the reads begun: an answer to any but the last is for a
	// period no longer shown, and is dropped.
	reading int
	// complete is a period every source has answered for.
	complete bool
	selected int
	scroll   int
}

// summaryAnswered is one source's answer for the read reading began.
type summaryAnswered struct {
	reading int
	read    activity.Read
}

var _ applier = summaryAnswered{}

// apply keeps an answer for the period shown, and drops one for a period
// since left.
func (msg summaryAnswered) apply(m Model) (Model, tea.Cmd) {
	if msg.reading != m.summary.reading {
		return m, nil
	}

	m.summary.reads = append(m.summary.reads, msg.read)
	m.summary.asking = remove(m.summary.asking, msg.read.Source)
	m.summary.complete = len(m.summary.asking) == 0
	m.summary.selected = min(m.summary.selected, max(0, len(m.summaryItems())-1))

	return m, nil
}

// summaryRested is [ or ] at rest on the period of the read reading.
type summaryRested struct {
	reading int
}

var _ applier = summaryRested{}

// apply reads the period the keys came to rest on, unless another key moved
// it on since.
func (msg summaryRested) apply(m Model) (Model, tea.Cmd) {
	if msg.reading != m.summary.reading {
		return m, nil
	}

	return m.readSummary()
}

// remove is sources without source.
func remove(sources []activity.Source, source activity.Source) []activity.Source {
	kept := make([]activity.Source, 0, len(sources))

	for _, each := range sources {
		if each != source {
			kept = append(kept, each)
		}
	}

	return kept
}

// today is the day the clock says it is, where it says it.
func (m Model) today() activity.Date { return activity.DateOf(m.deps.now()) }

// summaryPeriod is the period the pane shows: the one chosen, or the previous
// working day until one is.
func (m Model) summaryPeriod() activity.Period {
	if m.summary.chosen {
		return m.summary.period
	}

	return activity.PreviousWorkingDay(m.today())
}

// refreshSummary reads the period shown, unless it has ended and every source
// has read it in full: what was done then will not change.
func (m Model) refreshSummary() (Model, tea.Cmd) {
	if m.summaryReadInFull() && m.summary.period.To.Before(m.today()) {
		return m, nil
	}

	return m.readSummary()
}

// summaryLoading reports a read of the period shown that some source has not
// answered yet.
func (m Model) summaryLoading() bool {
	return m.summary.chosen && !m.summary.complete
}

// summaryReadInFull reports every source answered for the period shown, and
// none of them with a failure or left out as not set up, which may be set up
// by the next refresh.
func (m Model) summaryReadInFull() bool {
	return m.summary.complete && !slices.ContainsFunc(m.summary.reads, func(read activity.Read) bool {
		return read.Failed != nil || read.NotSetUp != nil
	})
}

// readSummary asks every source the deps reach for the period shown, each on
// its own, so each fills in or fails as it answers.
func (m Model) readSummary() (Model, tea.Cmd) {
	m.summary.period, m.summary.chosen = m.summaryPeriod(), true
	m.summary.reading++
	m.summary.reads, m.summary.complete = nil, false

	start, end := m.summary.period.Bounds(m.deps.now().Location())
	reads := m.summaryReads(start, end)
	reading := m.summary.reading

	m.summary.asking = make([]activity.Source, 0, len(reads))
	commands := make([]tea.Cmd, 0, len(reads))

	for _, source := range reads {
		m.summary.asking = append(m.summary.asking, source.Source)
		commands = append(commands, func() tea.Msg { return summaryAnswered{reading: reading, read: source.Read()} })
	}

	m.summary.complete = len(reads) == 0

	return m, tea.Batch(commands...)
}

// summaryReads are the reads of each source the deps reach, from start up to
// end.
func (m Model) summaryReads(start, end time.Time) []loop.SourceRead {
	deps := m.deps

	return loop.SummaryReads(loop.ActivitySeams{
		Commits: deps.Git.CommitsBetween, Touched: deps.Tasks.Touched,
		Jira: deps.Jira.Activity, BrowseURL: deps.Jira.BrowseURL,
		Forge: deps.Forge.Activity, ForgeKind: deps.Forge.Kind,
	}, start, end)
}

// shownSummary is what the sources have answered for the period shown.
func (m Model) shownSummary() activity.Summary {
	return activity.Summary{Period: m.summaryPeriod(), Reads: m.summary.reads}
}

// summaryItems are the items answered so far, oldest first, as the detail
// lists them.
func (m Model) summaryItems() []activity.Item {
	var items []activity.Item

	for _, year := range activity.Group(m.shownSummary().Items(), m.deps.now().Location()) {
		for _, month := range year.Months {
			for _, day := range month.Days {
				for _, hour := range day.Hours {
					items = append(items, hour.Items...)
				}
			}
		}
	}

	return items
}

// stepSummary shows the period steps of its own length later, or earlier when
// steps is negative, and reads it once the keys rest; never one that starts
// after today.
func (m Model) stepSummary(steps int) (Model, tea.Cmd) {
	moved := m.summaryPeriod().Step(steps)
	if m.today().Before(moved.From) {
		return m, nil
	}

	m.summary.period, m.summary.chosen = moved, true
	m.summary.reading++
	m.summary.reads, m.summary.asking, m.summary.complete = nil, nil, false
	m.summary.selected, m.summary.scroll = 0, 0

	reading := m.summary.reading

	return m, m.deps.after(summaryRest, func(time.Time) tea.Msg { return summaryRested{reading: reading} })
}

// showToday shows today and reads it at once.
func (m Model) showToday() (Model, tea.Cmd) {
	m.summary.period = activity.Period{From: m.today(), To: m.today()}
	m.summary.chosen, m.summary.selected, m.summary.scroll = true, 0, 0

	return m.readSummary()
}

// handleSummaryKey answers the Summary pane's keys: those that choose the
// period, and those that act on what it lists.
func (m Model) handleSummaryKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.earlier):
		return m.stepSummary(-1)
	case key.Matches(msg, m.keys.later):
		return m.stepSummary(1)
	case key.Matches(msg, m.keys.today):
		return m.showToday()
	case key.Matches(msg, m.keys.calendar):
		return m.openCalendar()
	case key.Matches(msg, m.keys.refresh):
		return m.readSummary()
	}

	return m.handleSummaryListKey(msg)
}

// handleSummaryListKey answers the keys that act on what the Summary lists:
// moving the cursor, copying it all, and the selected item's link.
func (m Model) handleSummaryListKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	selected := m.selectedSummaryItem()

	switch {
	case key.Matches(msg, m.keys.copySummary):
		return m.copySummary()
	case key.Matches(msg, m.keys.postSummary) && m.canPostSummary():
		return m.previewSummaryPost()
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveSummaryBy(m.keys.stepOf(msg)), nil
	case key.Matches(msg, m.keys.openLink):
		return m.openLink(selected.URL)
	case key.Matches(msg, m.keys.copyLink):
		return m.copyLink(selected.URL)
	}

	return m, nil
}

// moveSummaryBy moves the cursor delta items down, or up for a negative
// delta, stopping at either end.
func (m Model) moveSummaryBy(delta int) Model {
	m.summary.selected = max(0, min(m.summary.selected+delta, len(m.summaryItems())-1))

	return m
}

// selectedSummaryItem is the item the cursor is on, or none.
func (m Model) selectedSummaryItem() activity.Item {
	items := m.summaryItems()
	if m.summary.selected >= len(items) {
		return activity.Item{}
	}

	return items[m.summary.selected]
}

// copySummary copies the summary as Markdown and says so.
func (m Model) copySummary() (Model, tea.Cmd) {
	copyText := m.deps.Copy
	if copyText == nil {
		return m, nil
	}

	return m.noticed(m.marks.done + " copied the summary of " + m.summaryPeriod().String()),
		copyText(m.shownSummary().Text(m.deps.now().Location()))
}

// summaryKeys is what the pane offers: moving the period, today, copying,
// posting, and the selected item's link.
func (m Model) summaryKeys() []key.Binding {
	keys := []key.Binding{m.keys.earlier, m.keys.later, m.keys.today, m.keys.calendar}
	if m.deps.Copy != nil && len(m.summaryItems()) > 0 {
		keys = append(keys, m.keys.copySummary)
	}

	if m.canPostSummary() {
		keys = append(keys, m.keys.postSummary)
	}

	return append(append(keys, m.linkKeys(m.selectedSummaryItem().URL)...), m.keys.refresh)
}

// summaryRail is the period shown and how much was done in it, on its first
// line, which is all a pane given one row shows; or, before the pane is first
// looked at, that it is read then.
func (m Model) summaryRail(_ int) string {
	if !m.summary.chosen {
		return "what you did, read when opened"
	}

	period := m.summaryPeriod().String()

	switch items := len(m.summaryItems()); {
	case !m.summary.complete && items == 0:
		return period + ": " + m.marks.reading()
	case items == 0:
		return period + ": nothing done"
	case items == 1:
		return period + ": 1 thing done"
	default:
		return period + ": " + strconv.Itoa(items) + " things done"
	}
}

// summaryDetail is the period, then what each source could not say, then the
// items nested by year, month, day and hour, the cursor's marked.
func (m Model) summaryDetail(width int) string {
	lines := []string{m.styles.strong.Render(m.summaryPeriod().String()), ""}
	lines = append(lines, m.summaryNotes(width)...)

	years := activity.Group(m.shownSummary().Items(), m.deps.now().Location())
	if len(years) == 0 && m.summary.complete {
		lines = append(lines, "Nothing was done in this period.")
	}

	index := 0

	for _, year := range years {
		for _, month := range year.Months {
			lines = append(lines, m.styles.strong.Render(strconv.Itoa(year.Year)+" "+month.Month.String()))

			for _, day := range month.Days {
				lines = append(lines, "", m.styles.strong.Render(day.Date.Weekday().String()+" "+day.Date.String()))

				for _, hour := range day.Hours {
					lines = append(lines, m.styles.label.Render(hour.Label))

					for _, item := range hour.Items {
						lines = append(lines, wrap(summaryIndent+m.marks.marker(index == m.summary.selected)+itemLine(item), width))
						index++
					}
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

// summaryNotes say which sources are still being read, could not be read, are
// not set up, or had more than they gave, each within width.
func (m Model) summaryNotes(width int) []string {
	var notes []string

	for _, source := range m.summary.asking {
		notes = append(notes, m.styles.label.Render("reading "+source.Name()+m.marks.ellipsis))
	}

	for _, read := range m.summary.reads {
		name := read.Source.Title()

		for _, failure := range loop.Failures(read.Failed) {
			notes = append(notes, m.failedGlyph()+" "+failedSourceLine(name, failure))
		}

		if read.NotSetUp != nil {
			notes = append(notes, m.notSetUpNote(name, read.NotSetUp, width))
		}

		if read.Truncated {
			notes = append(notes, m.styles.label.Render(name+" had more than this shows."))
		}
	}

	if len(notes) > 0 {
		notes = append(notes, "")
	}

	return notes
}

// notSetUpNote says a source was left out because it is not set up, and how
// to set it up, as guidance — the not-started mark and the muted label — not
// as a failure: nothing was asked, so nothing refused.
func (m Model) notSetUpNote(name string, why error, width int) string {
	note := wrap(m.marks.notStarted+" "+name+" is not set up, so it was left out: "+inFull(why), width)

	return m.styles.label.Render(note)
}

// failedSourceLine says a source could not be read and why, naming the
// repository when it was one of several.
func failedSourceLine(name string, failure error) string {
	if repository, named := errors.AsType[loop.RepositoryError](failure); named {
		return name + " could not be read in " + sanitize.Line(repository.Repository) + ": " + briefly(repository.Err)
	}

	return name + " could not be read: " + briefly(failure)
}

// itemLine is what was done, to what, as one line, neutralized: the items are
// read from places anyone can write.
func itemLine(item activity.Item) string {
	words := []string{item.Kind.Verb()}
	for _, word := range []string{item.Ref, item.Title} {
		if word != "" {
			words = append(words, sanitize.Line(word))
		}
	}

	return strings.Join(words, " ")
}
