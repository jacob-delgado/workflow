// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// quickActionHint is the line the composer adds on GitLab, which runs a
// comment's slash lines.
const quickActionHint = "GitLab runs a line starting with / as a quick action"

// commentingOnTheForgeIssue opens the composer on the forge issue and writes
// text in it, back in normal mode.
func commentingOnTheForgeIssue(text string) []string {
	return append(append([]string{upAction, "c", "i"}, letters(text)...), keyEsc)
}

func TestAForgeIssuesCommentIsPostedAsWrittenWhateverJirasSetting(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge renders Markdown itself, so converting it to Jira's markup
	// would post asterisks and braces.
	world := withForgeIssue()
	world.cfg.Jira.MarkdownComments = true
	previewed := typing(t, world.live(t, 120, 40),
		append(commentingOnTheForgeIssue("**on it**"), keyEnter)...)

	// Act
	posted := typing(t, previewed, keyEnter).View().Content

	// Assert
	if got := world.asked("comment "); len(got) != 1 || got[0] != "comment "+string(forgeIssue)+" **on it**" {
		t.Errorf("posted %q, want the Markdown as written", got)
	}

	requireScreen(t, posted, "commented on #"+string(forgeIssue))
}

func TestTheComposerNamesAForgeIssueByItsNumberAndSaysTheForgeRendersIt(t *testing.T) {
	t.Parallel()

	// Arrange
	world := withForgeIssue()

	// Act
	view := typing(t, world.live(t, 120, 40), upAction, "c").View().Content

	// Assert
	requireScreen(t, view, "Comment on #"+string(forgeIssue), "the forge renders it")
}

func TestOnGitLabTheComposerSaysASlashLineIsAQuickAction(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind  forge.Kind
		issue jira.Key
		shown bool
	}{
		"a GitLab issue":       {kind: forge.KindGitLab, issue: forgeIssue, shown: true},
		"a GitHub issue":       {kind: forge.KindGitHub, issue: forgeIssue, shown: false},
		"a Jira issue, GitLab": {kind: forge.KindGitLab, issue: issueKey, shown: false},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			world := withForgeIssue()
			world.forgeKind = testCase.kind

			keys := []string{"c"}
			if testCase.issue == forgeIssue {
				keys = []string{upAction, "c"}
			}

			// Act
			view := typing(t, world.live(t, 120, 40), keys...).View().Content

			// Assert
			if got := strings.Contains(plain(view), quickActionHint); got != testCase.shown {
				t.Errorf("the quick-action hint shown = %v, want %v", got, testCase.shown)
			}
		})
	}
}

func TestTheThreadNeutralizesTerminalControlsInAuthorAndBody(t *testing.T) {
	t.Parallel()

	// Arrange
	// Anyone who can comment on the repository writes a forge thread; the
	// clients sanitize what they read, and the screen does again.
	world := newWorld()
	world.detail.Comments = []jira.Comment{
		{Author: "mallory\x1b]0;owned\x07", Body: "fine\x1b[2Jgone", Created: testNow()},
	}

	// Act
	view := world.live(t, 120, 40).View().Content

	// Assert
	// The raw screen, since refuseScreen strips escapes before it compares.
	for _, control := range []string{"\x1b]0;owned", "\x1b[2J"} {
		if strings.Contains(view, control) {
			t.Errorf("the screen carries the raw control %q", control)
		}
	}
}
