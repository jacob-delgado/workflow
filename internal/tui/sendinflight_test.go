// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A form that has sent its write takes no second enter, and no paste, until
// the service answers: Jira and git do not deduplicate, so a second press
// would write twice.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldAssign sends the assign, and then holds Jira's answer.
func heldAssign(deps *tui.Deps, held *hold) {
	assign := deps.Jira.Assign
	deps.Jira.Assign = func(key jira.Key, assignee string) error {
		err := assign(key, assignee)

		held.wait()

		return err
	}
}

// heldWorklog sends the work logged, and then holds Jira's answer.
func heldWorklog(deps *tui.Deps, held *hold) {
	add := deps.Jira.AddWorklog
	deps.Jira.AddWorklog = func(key jira.Key, spent, comment string) (jira.Worklog, error) {
		logged, err := add(key, spent, comment)

		held.wait()

		return logged, err
	}
}

// heldPullRequestLink sends the pull request's link to Jira, and then holds
// Jira's answer.
func heldPullRequestLink(deps *tui.Deps, held *hold) {
	link := deps.Jira.LinkPullRequest
	deps.Jira.LinkPullRequest = func(key jira.Key, url, title string) error {
		err := link(key, url, title)

		held.wait()

		return err
	}
}

// heldBranchLink keeps the branch's link, and then holds git's answer.
func heldBranchLink(deps *tui.Deps, held *hold) {
	link := deps.Git.LinkIssue
	deps.Git.LinkIssue = func(branch, issueKey string) error {
		err := link(branch, issueKey)

		held.wait()

		return err
	}
}

// offConventionWithoutPull is the world on a branch named for no issue, with
// no pull request opened from it.
func offConventionWithoutPull() *world {
	return onOffConventionBranch(false)
}

func TestAFormTakesNoSecondSendWhileItsWriteIsOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world func() *world
		wrap  func(deps *tui.Deps, held *hold)
		keys  []string
		sent  string
		doing string
		// unwanted is what the screen would show had the paste been typed, or
		// the write been answered.
		unwanted string
	}{
		"assigning": {
			world: newWorld, wrap: heldAssign, keys: []string{"a", "f", "r", "e", "d", keyEnter},
			sent: "assign ", doing: "sending…", unwanted: "> fredx",
		},
		"logging work": {
			world: newWorld, wrap: heldWorklog, keys: []string{"w", "2", "h", keyEnter},
			sent: "worklog ", doing: "sending…", unwanted: "> 2hx",
		},
		"linking the pull request on the issue": {
			world: withoutPull, wrap: heldPullRequestLink, keys: []string{"4", "n", keyEnter, keyEnter},
			sent: "link ", doing: "linking…", unwanted: "● linked",
		},
		"linking the branch": {
			world: offConventionWithoutPull, wrap: heldBranchLink, keys: []string{"2", "i", keyEnter},
			sent: "link-issue ", doing: "linking…", unwanted: "> " + issueKey + "x",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := tt.world()
			held := newHold(t)
			started := heldLive(t, held, repo, tt.wrap)
			held.armed.Store(true)
			sending := holding(t, held, started, tt.keys...)

			// Act
			again := holding(t, held, sending, keyEnter)
			pasted, _ := held.update(t, again, tea.PasteMsg{Content: "x"})

			// Assert
			view := pasted.View().Content
			requireScreen(t, view, tt.doing)
			refuseScreen(t, view, tt.unwanted)
			refuseScreen(t, footerLine(view), "enter")

			if calls := repo.asked(tt.sent); len(calls) != 1 {
				t.Errorf("sent %q, want the one write already out", calls)
			}
		})
	}
}
