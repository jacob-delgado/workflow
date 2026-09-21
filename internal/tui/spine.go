// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/progress"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// stage is one step of the loop the spine shows.
type stage struct {
	name  string
	glyph string
	hue   lipgloss.Style
}

// spine draws the loop's stages across the top, each in its system's color and
// with a glyph saying how far it has got. Short of rows it drops the names.
func (m Model) spine(shape layout.Layout) string {
	stages := m.stages()
	parts := make([]string, 0, len(stages))

	for _, each := range stages {
		if shape.CompactSpine() {
			parts = append(parts, each.hue.Render(initial(each.name))+m.paintGlyph(each))
		} else {
			parts = append(parts, m.paintGlyph(each)+each.hue.Render(" "+each.name))
		}
	}

	joined := " " + strings.Join(parts, m.marks.rule)
	if shape.CompactSpine() {
		joined = " " + strings.Join(parts, " ")
	}

	if m.dryRun {
		joined = " " + m.styles.strong.Render("DRY RUN") + m.marks.separator + strings.TrimLeft(joined, " ")
	}

	return ansi.Truncate(joined, shape.Spine.Width, "")
}

// initial is a stage name's first letter, the label the compact spine has room
// for so a stage is named by more than its position.
func initial(name string) string {
	return string([]rune(name)[0])
}

// paintGlyph colors a stage's glyph: red where it failed, so red reads the same
// on the spine as everywhere else, and the system's own hue otherwise.
func (m Model) paintGlyph(s stage) string {
	if s.glyph == m.marks.failed {
		return m.styles.failure.Render(s.glyph)
	}

	return s.hue.Render(s.glyph)
}

// stages works out how far along the loop the work is, in each stage's own hue,
// from the shared derivation both this spine and `workflow status` read.
func (m Model) stages() []stage {
	hues := []lipgloss.Style{m.styles.jira, m.styles.git, m.styles.git, m.styles.forge, m.styles.slack}

	derived := progress.Stages(m.work())
	stages := make([]stage, len(derived))

	for index, each := range derived {
		stages[index] = stage{name: each.Name, glyph: m.glyphFor(each.State), hue: hues[index]}
	}

	return stages
}

// work is what the panes know, gathered for the shared stage derivation.
func (m Model) work() progress.Work {
	_, named := m.branchIssue()
	_, selected := m.issues.current()

	return progress.Work{
		OnFeatureBranch:    m.branch.onFeatureBranch(),
		IssueNamed:         named,
		IssueSelected:      selected,
		Commits:            len(m.branch.branch.Commits),
		UncommittedChanges: len(m.changes.changes),
		PullRequestFound:   m.review.found,
		CI:                 m.review.ci.State,
		ChangesRequested:   m.review.pull.ChangesRequested,
		Announced:          m.announced(),
		PostPending:        m.slack.pending.waiting(),
	}
}

// glyphFor is the mark for a stage's state, in this session's glyph set.
func (m Model) glyphFor(state progress.State) string {
	switch state {
	case progress.Done:
		return m.marks.done
	case progress.InFlight:
		return m.marks.inFlight
	case progress.Failed:
		return m.marks.failed
	case progress.NotStarted:
		return m.marks.notStarted
	default:
		return m.marks.notStarted
	}
}
