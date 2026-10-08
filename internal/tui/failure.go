// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// sendState is an outbound request: whether it is in flight, and the error it
// came back with. Every overlay that posts, applies or writes carries one, and
// so does the Messaging pane's own post, so "in flight, then failed with this"
// is written and named one way rather than as a sending bool beside an err, an
// applyErr or a problem.
type sendState struct {
	sending bool
	err     error
}

// starting marks a request as in flight, clearing any earlier error.
func starting() sendState {
	return sendState{sending: true}
}

// failed records that the request came back with err, and is no longer in
// flight.
func (s sendState) failed(err error) sendState {
	return sendState{sending: false, err: err}
}

// noticedFailure reports a failure in the footer: the red mark and the error in
// full, its own words after the sentence, the whole row in the failure style.
func (m Model) noticedFailure(err error) Model {
	return m.noticedFailureLedBy("", err)
}

// noticedFailureLedBy is noticedFailure with what failed leading the row —
// "re-run failed: " — for a notice no open overlay names: the footer clips a
// long sentence, and the lead must survive it.
func (m Model) noticedFailureLedBy(lead string, err error) Model {
	words := []string{markOf(m.marks, err) + " " + lead + inFull(err)}
	if sentence, known := errorSentence(err); known && !sentence.whole {
		words = append(words, ownText(err))
	}

	m = m.noticed(strings.Join(words, "\n"))
	m.notice.failed = !loop.NotSetUp(err)

	return m
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
func voice(sty styles, marks glyphs, err error) (string, lipgloss.Style) {
	if loop.NotSetUp(err) {
		return marks.notStarted, lipgloss.NewStyle()
	}

	return marks.failed, sty.failure
}

// voicedMark is a cause's mark in its voice's style.
func voicedMark(sty styles, marks glyphs, err error) string {
	mark, style := voice(sty, marks, err)

	return style.Render(mark)
}

// failedGlyph is the failure mark in red, so red always means something broke —
// the free form lets a seam that carries only styles and glyphs redden it too.
func failedGlyph(sty styles, marks glyphs) string {
	return sty.failure.Render(marks.failed)
}

// failedGlyph is failedGlyph for a Model.
func (m Model) failedGlyph() string {
	return failedGlyph(m.styles, m.marks)
}

// failureSummary is a failure on a summary row — a rail, and the line a detail
// repeats from it: the red mark and the error in brief.
func (m Model) failureSummary(err error) string {
	return voicedMark(m.styles, m.marks, err) + " " + briefly(err)
}

// unreadRow is a rail's row for a load that did not answer, pointing at the
// detail that tells why: what failed, in the failure voice, or that it is not
// set up, as guidance.
func unreadRow(sty styles, marks glyphs, err error, failed string) string {
	if loop.NotSetUp(err) {
		failed = "not set up"
	}

	return voicedMark(sty, marks, err) + " " + failed + marks.separator + "see detail"
}

// failureLine is a failure on a row of its own with room to spare — an
// overlay's outcome, a field's problem, a run's headline: the red mark and the
// error in full, on the one row. The free form serves an overlay, which draws
// with styles and glyphs but no Model.
func failureLine(sty styles, marks glyphs, err error) string {
	return voicedMark(sty, marks, err) + " " + inFull(err)
}

// failureLine is failureLine for a Model.
func (m Model) failureLine(err error) string {
	return failureLine(m.styles, m.marks, err)
}

// failureBlock is a failure given room of its own — a pane, an overlay's pinned
// outcome: recognized as a sentence like failure, wrapped to a width and styled
// per row, so every row opens and closes its own color and none runs on into
// the border beside it.
func failureBlock(sty styles, marks glyphs, err error, width int) string {
	mark, style := voice(sty, marks, err)

	words, known := errorSentence(err)
	if !known {
		return style.Render(wrap(mark+" "+ownText(err), width))
	}

	if words.whole {
		return style.Render(wrap(mark+" "+words.full, width))
	}

	return style.Render(wrap(mark+" "+words.full, width)) + "\n" +
		sty.label.Render(wrap(ownText(err), width))
}

// failureBlock is failureBlock for a Model.
func (m Model) failureBlock(err error, width int) string {
	return failureBlock(m.styles, m.marks, err, width)
}

// pinnedProblem is what a form finds wrong with what was typed, drawn under its
// title as its pinned outcome is and wrapped the same way; nothing when
// nothing is.
func pinnedProblem(sty styles, marks glyphs, problem error, width int) []string {
	if problem == nil {
		return nil
	}

	return []string{failureBlock(sty, marks, problem, width), ""}
}

// pinnedOutcome is an overlay's outcome — the in-flight word or the refusal —
// drawn under the title rather than at the bottom, so a long reason is wrapped
// and seen instead of clipped below the fold. Empty when nothing has happened.
func pinnedOutcome(sty styles, marks glyphs, send sendState, doing string, width int) []string {
	switch {
	case send.sending:
		return []string{doing + marks.ellipsis, ""}
	case send.err != nil:
		return []string{failureBlock(sty, marks, send.err, width), ""}
	default:
		return nil
	}
}
