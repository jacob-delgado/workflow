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

	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// stage is one step of the loop the spine shows.
type stage struct {
	name  string
	glyph string
	hue   lipgloss.Style
}

// spineView is what the spine draws: the glyphs and styles, whether the
// session holds its writes back, where you work, how far the work has got,
// what the messaging service is called, the active task and the clock it has
// run by.
type spineView struct {
	kit     renderKit
	dryRun  bool
	place   string
	work    progress.Work
	service string
	task    taskwarrior.Task
	active  bool
	now     time.Time
}

// spineView is the spine's view of the rest of the interface.
func (m Model) spineView() spineView {
	task, active := m.tasks.active()

	return spineView{
		kit: m.kit(), dryRun: m.dryRun, place: placeLabel(m.deps), work: m.work(),
		service: m.cfg.Messaging.Service(), task: task, active: active, now: m.deps.now(),
	}
}

// draw draws the loop's stages across the top, each in its system's color and
// with a glyph saying how far it has got, and the active task at its end. Short
// of rows it drops the names.
func (v spineView) draw(shape layout.Layout) string {
	stages := v.stages()
	parts := make([]string, 0, len(stages))

	for _, each := range stages {
		if shape.CompactSpine() {
			parts = append(parts, each.hue.Render(initial(each.name))+v.paintGlyph(each))
		} else {
			parts = append(parts, v.paintGlyph(each)+each.hue.Render(" "+each.name))
		}
	}

	joined := " " + strings.Join(parts, v.kit.marks.rule)
	if shape.CompactSpine() {
		joined = " " + strings.Join(parts, " ")
	}

	joined = v.ledByPlace(joined, shape.Spine.Width)

	if v.dryRun {
		joined = " " + v.kit.styles.strong.Render("DRY RUN") + v.kit.marks.separator + strings.TrimLeft(joined, " ")
	}

	return ansi.Truncate(joined+v.activeTaskTail(shape, joined), shape.Spine.Width, "")
}

// ledByPlace leads the stages with where you work, faint, when there is room
// for both and for DRY RUN; the place is the first thing to give way, since
// the Repositories pane says it in full.
func (v spineView) ledByPlace(stages string, width int) string {
	if v.place == "" {
		return stages
	}

	led := " " + v.kit.styles.label.Render(v.place) + v.kit.marks.separator + strings.TrimLeft(stages, " ")

	room := width
	if v.dryRun {
		room -= ansi.StringWidth("DRY RUN" + v.kit.marks.separator)
	}

	if ansi.StringWidth(led) > room {
		return stages
	}

	return led
}

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
// hue, in the room the stages leave it. Nothing when no task is active.
func (v spineView) activeTaskTail(shape layout.Layout, stages string) string {
	if !v.active {
		return ""
	}

	room := shape.Spine.Width - ansi.StringWidth(stages) - 1

	tail := v.fittedTaskTail(room, shape.CompactSpine())
	if tail == "" {
		return ""
	}

	return strings.Repeat(" ", room+1-ansi.StringWidth(tail)) + v.kit.styles.tasks.Render(tail)
}

// fittedTaskTail is the active task in room columns: what it is, its
// description cut with the ellipsis where it must be, and how long it has run.
// Only how long on a compact spine, or where fewer than describedAtLeast
// columns of the description would show; and nothing where not even that fits,
// since a time cut short reads as another.
func (v spineView) fittedTaskTail(room int, compact bool) string {
	marks, task := v.kit.marks, v.task
	since := elapsed(task.Start, v.now)
	brief := marks.inFlight + " " + since
	described := room - ansi.StringWidth(brief+marks.separator)

	switch {
	case ansi.StringWidth(brief) > room:
		return ""
	case compact || described < min(describedAtLeast, ansi.StringWidth(task.Description)):
		return brief
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
// on the spine as everywhere else, and the system's own hue otherwise.
func (v spineView) paintGlyph(s stage) string {
	if s.glyph == v.kit.marks.failed {
		return v.kit.styles.failure.Render(s.glyph)
	}

	return s.hue.Render(s.glyph)
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
		stages[index] = stage{name: each.Name, glyph: glyphFor(v.kit.marks, each.State), hue: hues[each.System]}
	}

	return stages
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
		PostPending:        m.messaging.pending.waiting(),
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
