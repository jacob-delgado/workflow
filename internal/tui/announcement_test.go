// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// teamTemplate is a team's own shape for the ready-for-review message, naming
// every placeholder it offers.
const teamTemplate = "{author} :rocket: {noun} <{url}|{title}> for {key} {summary} {issue_url}"

// teamTemplated is the world's Slack bot, shaping its message with the team's
// template.
func teamTemplated() config.Messaging {
	templated := completeConfig().Messaging
	templated.Announcement = teamTemplate

	return templated
}

// loopAnnouncement is what loop composes for the world's branch over the seams
// the interface is handed: the text the command line and the web announce.
func loopAnnouncement(t *testing.T, w *world) string {
	t.Helper()

	deps := w.deps()
	seams := loop.AnnounceSeams{
		Branch: deps.Git.Branch, FindPull: deps.Forge.FindPullRequest, Author: deps.Forge.Author,
		Issue: deps.Jira.Issue, BrowseURL: deps.Jira.BrowseURL, CheckCI: deps.Forge.CheckStatus,
	}

	announcement, _, err := loop.ComposeAnnouncement(seams, w.cfg.Messaging, w.cfg.Jira.Project, w.forgeKind)
	if err != nil {
		t.Fatalf("loop composed no announcement: %v", err)
	}

	return announcement.Text()
}

// requireTextRows fails the test unless consecutive rows of the screen each end
// with the next line of text, followed by nothing but padding and the border:
// the whole text, line for line, with nothing added after any line.
func requireTextRows(t *testing.T, view, text string) {
	t.Helper()

	rows, lines := strings.Split(plain(view), "\n"), strings.Split(text, "\n")

	for start := range rows {
		if rowsEndWith(rows[start:], lines) {
			return
		}
	}

	t.Errorf("the screen does not show, row for row:\n%s\n\nscreen:\n%s", text, plain(view))
}

// rowsEndWith reports whether each of rows, in turn, holds the matching line
// with only padding and the border after it.
func rowsEndWith(rows, lines []string) bool {
	if len(rows) < len(lines) {
		return false
	}

	for index, line := range lines {
		at := strings.Index(rows[index], line)
		if at < 0 || strings.Trim(rows[index][at+len(line):], " ┃│") != "" {
			return false
		}
	}

	return true
}

func TestTheAnnouncementPreviewedIsTheOneLoopComposes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind      forge.Kind
		state     forge.PullState
		ci        forge.CIState
		messaging config.Messaging
	}{
		"ready for review":       {ci: forge.CIPassed, messaging: completeConfig().Messaging},
		"merged, on GitLab":      {kind: forge.KindGitLab, state: forge.StateMerged, messaging: completeConfig().Messaging},
		"its CI red, for Teams":  {ci: forge.CIFailed, messaging: teamsMessaging()},
		"in the team's template": {ci: forge.CIPassed, messaging: teamTemplated()},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The tracker answers with the summary the issue list holds, so the
			// interface and loop read the same issue.
			announcing := newWorld()
			announcing.detail.Issue.Summary = issueSummary
			announcing.forgeKind, announcing.pull.State = tt.kind, tt.state
			announcing.ci = []forge.CI{{State: tt.ci, Total: 1, Done: 1}}
			announcing.cfg.Messaging = tt.messaging
			want := loopAnnouncement(t, announcing)

			// Act
			view := typing(t, announcing.live(t, 240, 40), "5", "p").View().Content

			// Assert
			requireTextRows(t, view, want)
		})
	}
}
