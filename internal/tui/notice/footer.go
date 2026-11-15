// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package notice holds the footer's one-line report of what just happened,
// and the reports that wait their turn for it, what no pane repeats and a
// failure a search hid: which report takes the row, which is dropped, and
// which waits until it can be read.
package notice

import "slices"

// Stage is what the footer's notice row reports.
type Stage int

const (
	// StageNone reports nothing. It is the zero stage.
	StageNone Stage = iota
	// StagePlain reports a result or guidance a pane can show again, so it
	// gives way to any later report.
	StagePlain
	// StageFailure reports something that broke, the one report that asks
	// the user to act, so no plain report takes its row.
	StageFailure
	// StageUnrepeated reports what no pane repeats, such as what the store
	// lost, so it is never dropped unread: a report that takes its row puts
	// it first in line to show again.
	StageUnrepeated
)

// Footer is the notice row: the report it shows, and the reports waiting for
// it, oldest first. The zero Footer reports nothing. Every transition returns
// a new Footer and leaves the one it came from as it was, the waiting reports
// among them.
type Footer struct {
	stage   Stage
	text    string
	waiting []report
}

// report is one report waiting for the row: a failure or what no pane
// repeats.
type report struct {
	stage Stage
	text  string
}

// Stage is what the row reports.
func (f Footer) Stage() Stage {
	return f.stage
}

// Text is the report the row shows; empty at StageNone.
func (f Footer) Text() string {
	return f.text
}

// Waiting is how many reports wait for the row.
func (f Footer) Waiting() int {
	return len(f.waiting)
}

// Reported shows text as a plain report. A failure keeps the row, and text is
// dropped: what it says can be seen again in its pane. A report no pane
// repeats gives the row up and waits first in line.
func (f Footer) Reported(text string) Footer {
	if f.stage == StageFailure {
		return f
	}

	return f.showing(StagePlain, text)
}

// Failed shows text as a failure, which takes the row from any report, a
// failure before it among them. A report no pane repeats gives the row up and
// waits first in line. Covered, the row given over to a search whose keys
// would clear it unseen, the failure waits instead, and Next gives it the row
// once the search ends, as Failed would have. Either way a failure already
// waiting is dropped, as it would be replaced on the row.
func (f Footer) Failed(text string, covered bool) Footer {
	f.waiting = slices.DeleteFunc(slices.Clone(f.waiting), report.failure)

	if covered {
		return f.waitingFor(report{stage: StageFailure, text: text})
	}

	return f.showing(StageFailure, text)
}

// Unrepeated shows text, which no pane repeats, unless it cannot be read now:
// a failure or another such report holds the row unread, others wait before
// it, or covered, the row given over to a search whose keys would clear it
// unseen. Then it waits, last in line.
func (f Footer) Unrepeated(text string, covered bool) Footer {
	if f.stage == StageFailure || f.stage == StageUnrepeated || len(f.waiting) > 0 || covered {
		return f.waitingFor(report{stage: StageUnrepeated, text: text})
	}

	f.stage, f.text = StageUnrepeated, text

	return f
}

// Cleared drops the report shown, as a key that starts the next action does;
// the reports waiting stay in line.
func (f Footer) Cleared() Footer {
	f.stage, f.text = StageNone, ""

	return f
}

// Next gives the row, once it is not covered, to a failure waiting for it,
// which takes the row as Failed does on a row no search covers, ahead of
// every other report waiting; otherwise, once the row reports nothing, to the
// oldest report waiting. A row covered, a row with nothing waiting and a row
// that reports something with no failure waiting are unchanged.
func (f Footer) Next(covered bool) Footer {
	if covered || len(f.waiting) == 0 {
		return f
	}

	if at := slices.IndexFunc(f.waiting, report.failure); at >= 0 {
		failure := f.waiting[at]
		f.waiting = slices.Delete(slices.Clone(f.waiting), at, at+1)

		return f.showing(StageFailure, failure.text)
	}

	if f.stage != StageNone {
		return f
	}

	f.stage, f.text, f.waiting = f.waiting[0].stage, f.waiting[0].text, f.waiting[1:]

	return f
}

// failure reports whether the report waiting is a failure.
func (waiting report) failure() bool {
	return waiting.stage == StageFailure
}

// waitingFor puts waiting last in line.
func (f Footer) waitingFor(waiting report) Footer {
	// Clipped, so the append never writes into an array the Footer it came
	// from still holds.
	f.waiting = append(slices.Clip(f.waiting), waiting)

	return f
}

// showing is the row reporting text at stage, the report no pane repeats it
// held put first in line.
func (f Footer) showing(stage Stage, text string) Footer {
	if f.stage == StageUnrepeated {
		f.waiting = slices.Concat([]report{{stage: StageUnrepeated, text: f.text}}, f.waiting)
	}

	f.stage, f.text = stage, text

	return f
}
