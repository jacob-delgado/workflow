// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// noticedFailure reports a failure in the footer: the red mark and the error in
// full, its own words after the sentence, the whole row in the failure style.
// One that lands while a search covers the row waits for the search to end,
// since the keys typed into it would clear it unseen.
func (m Model) noticedFailure(err error) Model {
	return m.noticedFailureLedBy("", err)
}

// noticedFailureLedBy is noticedFailure with what failed leading the row —
// "re-run failed: " — for a notice no open overlay names: the footer clips a
// long sentence, and the lead must survive it.
func (m Model) noticedFailureLedBy(lead string, err error) Model {
	row := m.failureRow(lead, err)
	if loop.NotSetUp(err) {
		return m.noticed(row)
	}

	m.notice = m.notice.Failed(row, m.noticeCovered())

	return m
}

// failureRow is the footer's row for a failure led by lead: its mark, the
// error in full, and its own words after the sentence.
func (m Model) failureRow(lead string, err error) string {
	words := []string{markOf(m.marks, err) + " " + lead + inFull(err)}
	if sentence, known := errorSentence(err); known && !sentence.whole {
		words = append(words, ownText(err))
	}

	return m.oneRow(strings.Join(words, "\n"))
}

// noticedGuidance tells, plainly, why a key did nothing and what would. The
// refusal is not a failure, so it takes no mark and no red — red means
// something broke — but its words live where every failure's do: in
// errorSentence, or in the refusal's own text.
func (m Model) noticedGuidance(refusal error) Model {
	return m.noticed(inFull(refusal))
}

// markOf is the mark a cause opens its row with: the not-started mark for
// what was never set up, the failure mark for anything that broke. Shape, not
// color, tells them apart, so they read apart in monochrome as well.
func markOf(marks glyphs, err error) string {
	if loop.NotSetUp(err) {
		return marks.notStarted
	}

	return marks.failed
}

// voice is how a cause is drawn — its mark and the style its sentence takes —
// and the one place the interface decides between the two: what was never set
// up is guidance, plain, with the not-started mark, since nothing was asked and
// nothing refused; anything else broke, and is red with the failure mark.
func (kit renderKit) voice(err error) (string, lipgloss.Style) {
	if loop.NotSetUp(err) {
		return kit.marks.notStarted, lipgloss.NewStyle()
	}

	return kit.marks.failed, kit.styles.failure
}

// voicedMark is a cause's mark in its voice's style.
func (kit renderKit) voicedMark(err error) string {
	mark, style := kit.voice(err)

	return style.Render(mark)
}

// failedGlyph is the failure mark in red, so red always means something broke.
func (kit renderKit) failedGlyph() string {
	return kit.styles.failure.Render(kit.marks.failed)
}

// failureSummary is a failure on a summary row — a rail, and the line a detail
// repeats from it: the red mark and the error in brief.
func (kit renderKit) failureSummary(err error) string {
	return kit.voicedMark(err) + " " + briefly(err)
}

// drawnRead is how a pane draws its read, as Out: while no answer is taken,
// as shown, as failed with nothing held, and as failed over a value still
// held.
type drawnRead[Out any] struct {
	reading, shown, failed, failedOver func() Out
}

// drawRead is read as it stands: reading until a read answers, then as draw
// draws it. The map holds every stage, so exhaustive checks each is drawn.
func drawRead[T, Out any](read latest.Read[T], draw drawnRead[Out]) Out {
	failed := draw.failed
	if _, held := read.Value(); held {
		failed = draw.failedOver
	}

	return map[latest.Stage]func() Out{
		latest.StageNeverRead: draw.reading,
		latest.StageReading:   draw.reading,
		latest.StageShown:     draw.shown,
		latest.StageFailed:    failed,
	}[read.Stage()]()
}

// unreadRow is a rail's row for a load that did not answer, pointing at the
// detail that tells why: what failed, in the failure voice, or that it is not
// set up, as guidance.
func (kit renderKit) unreadRow(err error, failed string) string {
	if loop.NotSetUp(err) {
		failed = "not set up"
	}

	return kit.voicedMark(err) + " " + failed + kit.marks.separator + "see detail"
}

// failureLine is a failure on a row of its own with room to spare — an
// overlay's outcome, a field's problem, a run's headline: the red mark and the
// error in full, on the one row.
func (kit renderKit) failureLine(err error) string {
	return kit.voicedMark(err) + " " + inFull(err)
}

// refusalOrProblem is an overlay's outcome once nothing is out, below a
// blank row: the refusal its write met, wrapped to width rather than cut so
// the service's own words are all seen, or else the problem found before
// anything was sent, on its one row; nothing when there is neither.
func (kit renderKit) refusalOrProblem(refusal, problem error, width int) []string {
	if refusal != nil {
		return append([]string{""}, strings.Split(wrap(kit.failureLine(refusal), width), "\n")...)
	}

	if problem != nil {
		return []string{"", kit.failureLine(problem)}
	}

	return nil
}

// failureBlocks says each failure among errs that is not nil, beneath what
// comes before it, after a blank line; two reads that failed in the same
// words are said once.
func (kit renderKit) failureBlocks(width int, errs ...error) []string {
	var lines []string

	for _, err := range errs {
		if err == nil {
			continue
		}

		if block := kit.failureBlock(err, width); !slices.Contains(lines, block) {
			lines = append(lines, "", block)
		}
	}

	return lines
}

// failureBlock is a failure given room of its own — a pane, an overlay's pinned
// outcome: recognized as a sentence like failure, wrapped to a width and styled
// per row, so every row opens and closes its own color and none runs on into
// the border beside it.
func (kit renderKit) failureBlock(err error, width int) string {
	mark, style := kit.voice(err)

	words, known := errorSentence(err)
	if !known {
		return style.Render(wrap(mark+" "+ownText(err), width))
	}

	if words.whole {
		return style.Render(wrap(mark+" "+words.full, width))
	}

	return style.Render(wrap(mark+" "+words.full, width)) + "\n" +
		kit.styles.label.Render(wrap(ownText(err), width))
}

// pinnedProblem is what a form finds wrong with what was typed, drawn under its
// title as its pinned outcome is and wrapped the same way; nothing when
// nothing is.
func (kit renderKit) pinnedProblem(problem error, width int) []string {
	if problem == nil {
		return nil
	}

	return []string{kit.failureBlock(problem, width), ""}
}

// pinnedSend is an overlay's write, held in a send.State, drawn under the
// title rather than at the bottom, so a long reason is wrapped and seen
// instead of clipped below the fold: the in-flight word while it is out, the
// refusal once refused, and nothing while idle, which has no reason to draw.
func (kit renderKit) pinnedSend(state send.State, doing string, width int) []string {
	if state.InFlight() {
		return []string{doing + kit.marks.ellipsis, ""}
	}

	reason := state.Reason()
	if reason != nil {
		return []string{kit.failureBlock(reason, width), ""}
	}

	return nil
}
