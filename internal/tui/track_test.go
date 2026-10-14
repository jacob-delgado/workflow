// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The rules of the loop track: filled after a stage that is done, open after
// one still to come.
const (
	filledRule = "━"
	openRule   = "─"
)

// ruleRun is a run of either rule, however long the room made it.
var ruleRun = regexp.MustCompile(`━+|─+|=+|-{2,}`)

// track is the spine's plain text with every run of a rule cut to one
// character, so a test reads the stages' order and states whatever width the
// terminal gave the rules.
func track(view string) string {
	return ruleRun.ReplaceAllStringFunc(ansi.Strip(spineLine(view)), func(run string) string {
		return string([]rune(run)[0])
	})
}

// opening is the escape a style writes before its text, learned from the style
// rather than written out.
func opening(style lipgloss.Style) string {
	open, _, _ := strings.Cut(style.Render(filledRule), filledRule)

	return open
}

func TestADoneStagesRuleIsFilledInItsSystemsHue(t *testing.T) {
	t.Parallel()

	// Act
	spine := spineLine(newWorld().live(t, 120, 40).View().Content)

	// Assert
	blue := opening(lipgloss.NewStyle().Foreground(lipgloss.Color("4")))
	if !strings.Contains(spine, blue+filledRule) {
		t.Errorf("the done Issue stage's rule is not a filled rule in Jira's blue:\n%q", spine)
	}
}

func TestStagesStillToComeAreFaintAndOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	started := newWorld()
	started.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	started.changes, started.pullFound = nil, false

	// Act
	spine := spineLine(started.live(t, 120, 40).View().Content)

	// Assert
	faint := opening(lipgloss.NewStyle().Faint(true))
	if !strings.Contains(spine, faint+"○") || !strings.Contains(spine, faint+openRule) ||
		strings.Contains(spine, filledRule) {
		t.Errorf("the stages still to come are not faint, open rules:\n%q", spine)
	}
}

func TestTheTrackGivesWayAsRoomRunsOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cols, rows int
		ascii      bool
		want       string
	}{
		"an eighty-column terminal names every stage": {
			cols: 80, rows: 24,
			want: "● Issue ━ ● Branch ━ ● Commits ━ ● Review ━ ○ Slack",
		},
		"a terminal under sixty columns names the stages by initial": {
			cols: 59, rows: 40,
			want: " I● ━ B● ━ C● ━ R● ━ S○",
		},
		"an ASCII terminal draws the rules in plain characters": {
			cols: 120, rows: 40, ascii: true,
			want: "# Issue = # Branch = # Commits = # Review = o Slack",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			model := newWorld().live(t, tt.cols, tt.rows)
			if tt.ascii {
				model = asciiInterface(t, newWorld(), tt.cols, tt.rows)
			}

			view := model.View().Content

			// Assert
			if width := ansi.StringWidth(spineLine(view)); width > tt.cols {
				t.Errorf("the track is %d columns wide on a %d-column terminal:\n%s", width, tt.cols, ansi.Strip(spineLine(view)))
			}

			requireScreen(t, track(view), tt.want)
		})
	}
}

func TestReviewsRuleFillsAsItsChecksFinish(t *testing.T) {
	t.Parallel()

	// Arrange
	checking := newWorld()
	checking.ci = []forge.CI{{State: forge.CIRunning, Total: 4, Done: 2}}

	// Act
	spine := ansi.Strip(spineLine(checking.live(t, 120, 40).View().Content))

	// Assert
	_, after, _ := strings.Cut(spine, "◐ Review ")
	rule, _, _ := strings.Cut(after, " ")

	filled, open := strings.Count(rule, filledRule), strings.Count(rule, openRule)
	if filled == 0 || filled != open {
		t.Errorf("Review's rule = %q, want half filled with two of four checks finished", rule)
	}
}
