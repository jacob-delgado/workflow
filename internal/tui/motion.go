// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"math"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
)

// framesPerSecond is how often a moving screen is drawn again.
const framesPerSecond = 12

// The spring a rule fills by: critically damped, so it never overshoots a
// full rule, and quick enough to look done in about 350 ms.
const (
	springFrequency = 13
	springDamping   = 1
	// springRest is how near its target, and how slow, a spring may be and
	// still count as come to rest.
	springRest = 0.005
)

// openingGrace is how long the first screen has to fill in from what the
// services answer before a change to it moves; a key pressed sooner ends it.
const openingGrace = 2 * time.Second

// spring is a share shown on screen — of a rule, or of a bar — on its way to
// the share it stands for.
type spring struct {
	shown, velocity, target float64
}

// motionState is what moves on the screen: the frame the in-flight mark has
// turned to, the track's rules on their way to how far each stage has got,
// the Review pane's bar on its way to the share of CI's checks finished,
// whether the next frame is already asked for, so one chain of frames runs at
// a time, when the session began and whether its first screen is done
// filling in, before which nothing moves, and whether ui.motion keeps
// everything still.
type motionState struct {
	turn   spinner.Spinner
	frame  int
	rules  []spring
	checks spring
	ease   harmonica.Spring
	asked  bool
	began  time.Time
	opened bool
	still  bool
}

// newMotion is a screen at rest from began, turning the in-flight mark through
// turn's frames when it moves at all.
func newMotion(turn spinner.Spinner, began time.Time, still bool) motionState {
	return motionState{
		turn: turn, began: began, still: still,
		ease: harmonica.NewSpring(harmonica.FPS(framesPerSecond), springFrequency, springDamping),
	}
}

// looked is the motion once the user has done something on the first screen,
// which ends its filling in.
func (s motionState) looked() motionState {
	s.opened = true

	return s
}

// aimed points each of the track's rules at targets, how far its stage has
// got, and the Review bar at checks, the share of CI's checks finished.
func (s motionState) aimed(targets []float64, checks float64) motionState {
	rules := make([]spring, len(targets))

	for index, target := range targets {
		rules[index] = spring{shown: target, target: target}
		if index < len(s.rules) {
			rules[index] = s.aim(s.rules[index], target)
		}
	}

	s.rules, s.checks = rules, s.aim(s.checks, checks)

	return s
}

// aim points from at target. Until the first screen is done filling in, and
// under reduced motion, it stands at its target, so the opening screen and a
// still one never move.
func (s motionState) aim(from spring, target float64) spring {
	if !s.opened || s.still {
		return spring{shown: target, target: target}
	}

	from.target = target

	return from
}

// stepped is every rule one frame further toward its target.
func (s motionState) stepped() motionState {
	rules := make([]spring, len(s.rules))

	for index, rule := range s.rules {
		rules[index] = rule.stepped(s.ease)
	}

	s.rules, s.checks = rules, s.checks.stepped(s.ease)

	return s
}

// stepped is the spring one frame on, at its target once it has come to rest.
func (r spring) stepped(ease harmonica.Spring) spring {
	r.shown, r.velocity = ease.Update(r.shown, r.velocity, r.target)
	if r.resting() {
		r.shown, r.velocity = r.target, 0
	}

	return r
}

// resting reports whether the spring has come to rest at its target.
func (r spring) resting() bool {
	return math.Abs(r.target-r.shown) < springRest && math.Abs(r.velocity) < springRest
}

// filling reports whether any rule, or the Review bar, is still on its way.
func (s motionState) filling() bool {
	for _, rule := range s.rules {
		if rule.shown != rule.target {
			return true
		}
	}

	return s.checks.shown != s.checks.target
}

// shown is how far each rule is drawn filled.
func (s motionState) shown() []float64 {
	shares := make([]float64, len(s.rules))
	for index, rule := range s.rules {
		shares[index] = rule.shown
	}

	return shares
}

// motionTick is a frame arriving.
type motionTick struct{}

// apply moves the screen one frame on, asking for the next: each rule toward
// its stage, and the in-flight mark one turn while a read is out, back to its
// first once none is, so the next read starts there.
func (motionTick) apply(m Model) (Model, tea.Cmd) {
	m.motion.asked = false
	m.motion = m.motion.stepped()
	turned := m.motion.frame
	m.motion.frame = 0

	if m.turning() {
		m.motion.frame = (turned + 1) % len(m.motion.turn.Frames)
	}

	return m.nextFrame(nil)
}

// reading reports whether any pane's read is out.
func (m Model) reading() bool {
	for each := range pane(paneCount) {
		if behaviorOf(each).loading(m) {
			return true
		}
	}

	return false
}

// turning reports whether the in-flight mark turns: a pane's read is out, the
// mark has more than one frame to turn through, and ui.motion does not keep
// it still.
func (m Model) turning() bool {
	return !m.motion.still && len(m.motion.turn.Frames) > 1 && m.reading()
}

// moving reports whether anything on screen moves: the in-flight mark, or a
// rule on its way to its stage.
func (m Model) moving() bool {
	return m.turning() || m.motion.filling()
}

// nextFrame points the track's rules at how far each stage has got, and asks
// the timer for the next frame alongside cmd while something moves and no
// frame is asked for yet. Only while: a screen at rest asks for nothing, so
// the program sleeps until a key or an answer wakes it.
func (m Model) nextFrame(cmd tea.Cmd) (Model, tea.Cmd) {
	m.motion = m.motion.aimed(m.spineView().targets(), checksFinished(m.review.ci))
	m.motion.opened = m.motion.opened || !m.reading() && m.deps.now().Sub(m.motion.began) >= openingGrace

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
