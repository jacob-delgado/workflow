// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// stage is one step of the loop the spine shows, and how much of the rule
// after it is filled: all of it once the stage is done, CI's share of its
// checks finished while Review is in flight, none otherwise.
type stage struct {
	name  string
	state progress.State
	glyph string
	hue   lipgloss.Style
	fill  float64
}

// spineView is what the spine draws: the glyphs and styles, whether the
// session holds its writes back, where you work, how far the work has got,
// what the messaging service is called, CI's share of checks finished, how
// far each rule is drawn filled on its way there, the active task and the
// clock it has run by.
type spineView struct {
	kit     renderKit
	dryRun  bool
	place   string
	work    progress.Work
	service string
	checked float64
	shown   []float64
	task    taskwarrior.Task
	active  bool
	now     time.Time
}

// spineView is the spine's view of the rest of the interface.
func (m Model) spineView() spineView {
	task, active := m.tasks.active()

	return spineView{
		kit: m.kit(), dryRun: m.dryRun, place: placeLabel(m.deps), work: m.work(),
		service: m.cfg.Messaging.Service(), checked: checksFinished(m.review.ci), shown: m.motion.shown(),
		task: task, active: active, now: m.deps.now(),
	}
}

// checksFinished is the share of CI's checks that have finished, zero where
// the forge reports none one by one.
func checksFinished(ci forge.CI) float64 {
	if ci.Total == 0 {
		return 0
	}

	return float64(ci.Done) / float64(ci.Total)
}

// The loop track's rules: at most ruleMost columns each, and at least
// ruleNamed while the stages keep their names. Below trackNarrowBelow columns
// the stages go by their initials.
const (
	ruleMost         = 8
	ruleNamed        = 3
	trackNarrowBelow = 60
	trackRules       = 4
)

// trackFit is one way the spine can be laid out: whether it leads with where
// you work, whether the active task keeps its description, whether the stages
// keep their names, and the fewest and most columns each rule may take.
type trackFit struct {
	place, described, named bool
	leastRule, mostRule     int
}

// fits are the layouts the spine tries, roomiest first: the place gives way
// first, then the task's description, then the stages' names, and last the
// rules themselves.
func (v spineView) fits(shape layout.Layout) []trackFit {
	initials := []trackFit{{leastRule: 1, mostRule: 1}, {}}
	if shape.CompactSpine() || shape.Spine.Width < trackNarrowBelow {
		return initials
	}

	return append([]trackFit{
		{place: v.place != "", described: true, named: true, leastRule: ruleNamed, mostRule: ruleMost},
		{described: true, named: true, leastRule: ruleNamed, mostRule: ruleMost},
		{named: true, leastRule: ruleNamed, mostRule: ruleMost},
	}, initials...)
}

// draw draws the loop's stages across the top, each in its system's color and
// with a glyph saying how far it has got, a rule after each filled as far as
// the stage has, and the active task at its end. It takes the first of fits
// with room for every rule.
func (v spineView) draw(shape layout.Layout) string {
	width, stages := shape.Spine.Width, v.stages()
	fits := v.fits(shape)
	chosen, rule := fits[len(fits)-1], 0

	for _, fit := range fits {
		if length, ok := v.ruleLength(fit, stages, width); ok {
			chosen, rule = fit, length

			break
		}
	}

	line := v.lead(chosen) + v.trackOf(stages, chosen.named, rule)

	return ansi.Truncate(line+v.activeTaskTail(width, line, chosen.described), width, "")
}

// ruleLength is how long each rule is under fit: an equal share of the room
// the rest leaves, at most the fit's most, and whether there is that room and
// the share reaches the fit's least.
func (v spineView) ruleLength(fit trackFit, stages []stage, width int) (int, bool) {
	free := width - v.ruleless(fit, stages) - v.leastTail(fit.described)

	return min(free/trackRules, fit.mostRule), free >= 0 && free/trackRules >= fit.leastRule
}

// ruleless is how wide the spine is under fit before its rules take a column:
// a rule brings a space of its own beside the one between two stages.
func (v spineView) ruleless(fit trackFit, stages []stage) int {
	if fit.mostRule == 0 {
		return ansi.StringWidth(v.lead(fit) + v.trackOf(stages, fit.named, 0))
	}

	return ansi.StringWidth(v.lead(fit)+v.trackOf(stages, fit.named, 1)) - trackRules
}

// leastTail is the fewest columns the active task needs at the spine's end,
// with its description or without; none when no task is active.
func (v spineView) leastTail(described bool) int {
	if !v.active {
		return 0
	}

	brief := ansi.StringWidth(" " + v.kit.marks.inFlight + " " + elapsed(v.task.Start, v.now))
	if !described {
		return brief
	}

	return brief + ansi.StringWidth(v.kit.marks.separator) +
		min(describedAtLeast, ansi.StringWidth(v.task.Description))
}

// lead is what comes before the stages: DRY RUN while writes are held back,
// and where you work when the fit has room for it.
func (v spineView) lead(fit trackFit) string {
	lead := " "
	if v.dryRun {
		lead += v.kit.styles.strong.Render("DRY RUN") + v.kit.marks.separator
	}

	if fit.place {
		lead += v.kit.styles.label.Render(v.place) + v.kit.marks.separator
	}

	return lead
}

// trackOf is the stages, by name or by initial, with a rule of length
// columns between each two; with no length for a rule, a space alone.
func (v spineView) trackOf(stages []stage, named bool, length int) string {
	parts := make([]string, len(stages))

	for index, each := range stages {
		parts[index] = v.label(each, named)
		if index < len(stages)-1 && length > 0 {
			parts[index] += " " + v.ruleAfter(each, length)
		}
	}

	return strings.Join(parts, " ")
}

// label is a stage's glyph and its name, or its initial then its glyph:
// faint while the stage is still to come, in its system's hue otherwise.
func (v spineView) label(step stage, named bool) string {
	hue := step.hue
	if step.state == progress.NotStarted {
		hue = v.kit.styles.label
	}

	glyph := v.paintGlyph(step, hue)
	if !named {
		return hue.Render(initial(step.name)) + glyph
	}

	return glyph + hue.Render(" "+step.name)
}

// ruleAfter is the rule after a stage: filled in its hue as far as the stage
// has got, open and faint for the rest.
func (v spineView) ruleAfter(step stage, length int) string {
	return v.kit.bar(step.fill, length, step.hue)
}

// bar is share of length columns filled in hue, the rest open and faint, so
// how far reads by shape as well as by color.
func (k renderKit) bar(share float64, length int, hue lipgloss.Style) string {
	filled := min(length, max(0, int(share*float64(length)+half)))

	bar := strings.Repeat(k.marks.trackFilled, filled)
	if filled > 0 {
		bar = hue.Render(bar)
	}

	if filled < length {
		bar += k.styles.label.Render(strings.Repeat(k.marks.trackOpen, length-filled))
	}

	return bar
}

// half rounds a share of a rule to the nearest column.
const half = 0.5

// placeLabel is where you work, briefly: the repository's name and the path
// within it, or the directory's own name outside one; nothing when the
// interface was not told.
func placeLabel(deps Deps) string {
	here := deps.Repositories.Here
	if here.Dir == "" {
		return ""
	}

	if here.Root == "" {
		return sanitize.Line(filepath.Base(here.Dir))
	}

	label := filepath.Base(here.Root)

	path, err := filepath.Rel(here.Root, here.Dir)
	if err == nil && path != "." {
		label += "/" + filepath.ToSlash(path)
	}

	return sanitize.Line(label)
}

// describedAtLeast is the fewest columns of the active task's description the
// spine shows: with less room, it shows only how long the task has run.
const describedAtLeast = 8

// activeTaskTail is the active task at the spine's right edge, in Taskwarrior's
// hue, in the room the track leaves it. Nothing when no task is active.
func (v spineView) activeTaskTail(width int, track string, described bool) string {
	if !v.active {
		return ""
	}

	room := width - ansi.StringWidth(track) - 1

	tail := v.fittedTaskTail(room, !described)
	if tail == "" {
		return ""
	}

	return strings.Repeat(" ", room+1-ansi.StringWidth(tail)) + v.kit.styles.tasks.Render(tail)
}

// fittedTaskTail is the active task in room columns: what it is, its
// description cut with the ellipsis where it must be, and how long it has run.
// Only how long when brief, or where fewer than describedAtLeast columns of
// the description would show; and nothing where not even that fits, since a
// time cut short reads as another.
func (v spineView) fittedTaskTail(room int, brief bool) string {
	marks, task := v.kit.marks, v.task
	since := elapsed(task.Start, v.now)
	short := marks.inFlight + " " + since
	described := room - ansi.StringWidth(short+marks.separator)

	switch {
	case ansi.StringWidth(short) > room:
		return ""
	case brief || described < min(describedAtLeast, ansi.StringWidth(task.Description)):
		return short
	default:
		return marks.inFlight + " " + ansi.Truncate(task.Description, described, marks.ellipsis) +
			marks.separator + since
	}
}

// elapsed is how long it is from one moment to a later one, as briefly as is
// still clear: minutes, then hours and minutes, then days and hours. A from
// after to is no time at all.
func elapsed(from, to time.Time) string {
	length := max(0, to.Sub(from))

	switch {
	case length < time.Hour:
		return strconv.Itoa(int(length.Minutes())) + "m"
	case length < day:
		return strconv.Itoa(int(length.Hours())) + "h" + strconv.Itoa(int((length % time.Hour).Minutes())) + "m"
	default:
		return strconv.Itoa(int(length/day)) + "d " + strconv.Itoa(int((length % day).Hours())) + "h"
	}
}

// initial is a stage name's first letter, the label the compact spine has room
// for so a stage is named by more than its position.
func initial(name string) string {
	return string([]rune(name)[0])
}

// paintGlyph colors a stage's glyph: red where it failed, so red reads the same
// on the spine as everywhere else, and hue otherwise.
func (v spineView) paintGlyph(s stage, hue lipgloss.Style) string {
	if s.state == progress.Failed {
		return v.kit.styles.failure.Render(s.glyph)
	}

	return hue.Render(s.glyph)
}

// stages works out how far along the loop the work is, in the hue of each
// stage's system, from the shared derivation both this spine and `workflow
// status` read.
func (v spineView) stages() []stage {
	sty := v.kit.styles
	// Trade-off TRADE-5: the hue tells the systems apart by color alone; the
	// stage's name, or its initial when compact, is what names the stage.
	hues := map[progress.System]lipgloss.Style{
		progress.Tracker: sty.jira, progress.Git: sty.git,
		progress.Forge: sty.forge, progress.Messaging: sty.messaging,
	}

	derived := progress.Stages(v.work, v.service)
	stages := make([]stage, len(derived))

	for index, each := range derived {
		stages[index] = stage{
			name: each.Name, state: each.State, glyph: glyphFor(v.kit.marks, each.State), hue: hues[each.System],
			fill: v.fillOf(each),
		}
		if index < len(v.shown) {
			stages[index].fill = v.shown[index]
		}
	}

	return stages
}

// targets are how far each stage's rule is filled once it comes to rest.
func (v spineView) targets() []float64 {
	derived := progress.Stages(v.work, v.service)
	fills := make([]float64, len(derived))

	for index, each := range derived {
		fills[index] = v.fillOf(each)
	}

	return fills
}

// fillOf is how much of the rule after a stage is filled: all of it once
// done, CI's share of checks finished while Review is in flight, and none
// otherwise.
func (v spineView) fillOf(each progress.Stage) float64 {
	// Trade-off TRADE-45: CI is no stage of its own; its progress fills the
	// rule after Review.
	switch {
	case each.State == progress.Done:
		return 1
	case each.State == progress.InFlight && each.System == progress.Forge:
		return v.checked
	default:
		return 0
	}
}

// work is what the panes know, gathered for the shared stage derivation.
func (m Model) work() progress.Work {
	_, named := m.branch.issue(m.cfg.Jira.Project)
	_, selected := m.issues.current()

	return progress.Work{
		OnFeatureBranch:    m.branch.onFeatureBranch(),
		IssueNamed:         named,
		IssueSelected:      selected,
		Commits:            len(m.branch.branch.Commits),
		UncommittedChanges: len(m.changes.changes),
		PullRequest:        m.review.pullState(),
		CI:                 m.review.ci.State,
		ChangesRequested:   m.review.pull.ChangesRequested,
		Announced:          m.messaging.announced(m.review),
		AnnouncePending:    m.messaging.pending.waiting() || m.messaging.send.sending,
	}
}

// pullState is where the branch's pull request stands, for the stages: a
// merged one, as the rail says, has finished its review.
func (s reviewState) pullState() progress.PullState {
	if !s.found {
		return progress.NoPullRequest
	}

	return progress.PullStateOf(s.pull.State)
}

// glyphFor is the mark for a stage's state, in this session's glyph set. A
// map, not a switch, so there is no last-case arm gobco can never see;
// exhaustive keeps it complete.
func glyphFor(marks glyphs, state progress.State) string {
	return map[progress.State]string{
		progress.Done:       marks.done,
		progress.InFlight:   marks.inFlight,
		progress.Failed:     marks.failed,
		progress.NotStarted: marks.notStarted,
	}[state]
}
