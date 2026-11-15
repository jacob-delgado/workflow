// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package notice holds the footer's one-line report of what just happened,
// and the reports no pane repeats that wait their turn for it: which report
// takes the row, which is dropped, and which waits until it can be read.
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

// Footer is the notice row: the report it shows, and the reports no pane
// repeats waiting for it, oldest first. The zero Footer reports nothing.
// Every transition returns a new Footer and leaves the one it came from as it
// was, the waiting reports among them.
type Footer struct {
	stage   Stage
	text    string
	waiting []string
}

// Stage is what the row reports.
func (f Footer) Stage() Stage {
	return f.stage
}

// Text is the report the row shows; empty at StageNone.
func (f Footer) Text() string {
	return f.text
}

// Waiting is how many reports no pane repeats wait for the row.
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
// waits first in line.
func (f Footer) Failed(text string) Footer {
	return f.showing(StageFailure, text)
}

// Unrepeated shows text, which no pane repeats, unless it cannot be read now:
// a failure or another such report holds the row unread, others wait before
// it, or covered, the row given over to a search whose keys would clear it
// unseen. Then it waits, last in line.
func (f Footer) Unrepeated(text string, covered bool) Footer {
	if f.stage == StageFailure || f.stage == StageUnrepeated || len(f.waiting) > 0 || covered {
		// Clipped, so the append never writes into an array the Footer it
		// came from still holds.
		f.waiting = append(slices.Clip(f.waiting), text)

		return f
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

// Next gives the row, once it reports nothing and is not covered, to the
// oldest report waiting. A row that reports something, a row covered and a
// row with nothing waiting are unchanged.
func (f Footer) Next(covered bool) Footer {
	if f.stage != StageNone || len(f.waiting) == 0 || covered {
		return f
	}

	f.stage, f.text, f.waiting = StageUnrepeated, f.waiting[0], f.waiting[1:]

	return f
}

// showing is the row reporting text at stage, the report no pane repeats it
// held put first in line.
func (f Footer) showing(stage Stage, text string) Footer {
	if f.stage == StageUnrepeated {
		f.waiting = slices.Concat([]string{f.text}, f.waiting)
	}

	f.stage, f.text = stage, text

	return f
}
