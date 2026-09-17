// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
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
			parts = append(parts, m.paintGlyph(each))
		} else {
			parts = append(parts, m.paintGlyph(each)+each.hue.Render(" "+each.name))
		}
	}

	joined := " " + strings.Join(parts, m.marks.rule)
	if shape.CompactSpine() {
		joined = " [" + strings.Join(parts, "") + "]"
	}

	if m.dryRun {
		joined = " " + m.styles.strong.Render("DRY RUN") + m.marks.separator + strings.TrimLeft(joined, " ")
	}

	return ansi.Truncate(joined, shape.Spine.Width, "")
}

// paintGlyph colors a stage's glyph: red where it failed, so red reads the same
// on the spine as everywhere else, and the system's own hue otherwise.
func (m Model) paintGlyph(s stage) string {
	if s.glyph == m.marks.failed {
		return m.styles.failure.Render(s.glyph)
	}

	return s.hue.Render(s.glyph)
}

// stages works out how far along the loop the work is, from what the panes
// know: nothing is stored, so it is all derived.
func (m Model) stages() []stage {
	return []stage{
		{name: "Issue", glyph: m.issueStage(), hue: m.styles.jira},
		{name: "Branch", glyph: m.branchStage(), hue: m.styles.git},
		{name: "Commits", glyph: m.commitStage(), hue: m.styles.git},
		{name: "Review", glyph: m.reviewStage(), hue: m.styles.forge},
		{name: "Slack", glyph: m.slackStage(), hue: m.styles.slack},
	}
}

// issueStage is done once the branch names an issue, and in flight while one is
// selected.
func (m Model) issueStage() string {
	_, named := convention.IssueKey(m.branch.branch.Name)
	_, selected := m.issues.current()

	switch {
	case named:
		return m.marks.done
	case selected:
		return m.marks.inFlight
	default:
		return m.marks.notStarted
	}
}

// branchStage is done on a feature branch.
func (m Model) branchStage() string {
	if m.branch.onFeatureBranch() {
		return m.marks.done
	}

	return m.marks.notStarted
}

// commitStage is done once the branch has commits, and in flight while there
// are changes to commit.
func (m Model) commitStage() string {
	switch {
	case m.branch.onFeatureBranch() && len(m.branch.branch.Commits) > 0:
		return m.marks.done
	case len(m.changes.changes) > 0:
		return m.marks.inFlight
	default:
		return m.marks.notStarted
	}
}

// reviewStage follows the pull request and its CI.
func (m Model) reviewStage() string {
	switch {
	case !m.review.found:
		return m.marks.notStarted
	case m.review.ci.State == forge.CIFailed:
		return m.marks.failed
	case m.review.ci.State == forge.CIPassed:
		return m.marks.done
	default:
		return m.marks.inFlight
	}
}

// slackStage is done once this pull request is posted, and in flight while a
// post waits for CI.
func (m Model) slackStage() string {
	switch {
	case m.announced():
		return m.marks.done
	case m.slack.pending.waiting():
		return m.marks.inFlight
	default:
		return m.marks.notStarted
	}
}
