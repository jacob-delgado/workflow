// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// framesPerSecond is how often a moving screen is drawn again.
const framesPerSecond = 8

// motionState is what moves on the screen: the frame the in-flight mark has
// turned to, whether the next frame is already asked for, so one chain of
// frames runs at a time, and whether ui.motion keeps everything still.
type motionState struct {
	turn  spinner.Spinner
	frame int
	asked bool
	still bool
}

// newMotion is a screen at rest, turning the in-flight mark through turn's
// frames when it moves at all.
func newMotion(turn spinner.Spinner, still bool) motionState {
	return motionState{turn: turn, still: still}
}

// motionTick is a frame arriving.
type motionTick struct{}

// apply turns the in-flight mark one frame on, asking for the next, while
// something still moves, and back to its first once nothing does, so the next
// read starts there.
func (motionTick) apply(m Model) (Model, tea.Cmd) {
	m.motion.asked = false
	if !m.moving() {
		m.motion.frame = 0

		return m, nil
	}

	m.motion.frame = (m.motion.frame + 1) % len(m.motion.turn.Frames)

	return m.nextFrame(nil)
}

// moving reports whether anything on screen moves: a pane's read is out, the
// in-flight mark has more than one frame to turn through, and ui.motion does
// not keep it still.
func (m Model) moving() bool {
	if m.motion.still || len(m.motion.turn.Frames) < 2 {
		return false
	}

	for each := range pane(paneCount) {
		if behaviorOf(each).loading(m) {
			return true
		}
	}

	return false
}

// nextFrame asks the timer for the next frame alongside cmd while something
// moves and no frame is asked for yet. Only while: a screen at rest asks for
// nothing, so the program sleeps until a key or an answer wakes it.
func (m Model) nextFrame(cmd tea.Cmd) (Model, tea.Cmd) {
	if m.motion.asked || !m.moving() {
		return m, cmd
	}

	m.motion.asked = true

	return m, tea.Batch(cmd, m.deps.after(m.motion.turn.FPS, func(time.Time) tea.Msg { return motionTick{} }))
}

// turning is the in-flight mark at the frame it has turned to.
func (s motionState) turning() string {
	return s.turn.Frames[s.frame]
}
