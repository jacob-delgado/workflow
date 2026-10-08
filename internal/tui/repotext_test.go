// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A repository's own .workflow.json applies to anyone who opens workflow in
// its clone, and its templates are files anyone can name, so what they hold
// is drawn as text and never as a terminal sequence.

import (
	"regexp"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// clipboardWrite is an OSC 52 sequence, which asks the terminal to set the
// clipboard.
const clipboardWrite = "\x1b]52;c;AAAA\x07"

// terminalControl is an escape that opens anything but the SGR color
// sequences the interface draws itself, or a bell.
var terminalControl = regexp.MustCompile("\x1b[^\\[]|\x07")

// fromAHostileRepository is a world whose configuration and pull request
// template carry clipboardWrite in every name a repository writes. Its branch
// has a pull request open; hostileWithoutAPull opens none.
func fromAHostileRepository() *world {
	repo := summaryWorld()
	repo.issues = nil
	repo.cfg.Path = "/home/ana/src/api/" + clipboardWrite + "/.workflow.json"
	repo.cfg.Jira.Views = []config.JiraView{
		{Name: "mine" + clipboardWrite, JQL: "project = PROJ"}, {Name: "team", JQL: "project = TEAM"},
	}
	repo.cfg.Jira.ReviewStatus = "In Review" + clipboardWrite
	repo.cfg.Messaging.Channel = devChannel + clipboardWrite
	repo.cfg.Messaging.Channels = []string{teamChannel + clipboardWrite}
	repo.cfg.UI.Keys = map[string]string{refreshAction: reboundRefreshKey + clipboardWrite}
	repo.cfg.Commit.Types = []string{"fix" + clipboardWrite, "feat"}
	repo.templates = []forge.Template{{Name: "bugfix" + clipboardWrite, Body: "## Bug\n"}}

	return repo
}

// hostileWithoutAPull is fromAHostileRepository with no pull request opened from the
// branch, so one can be.
func hostileWithoutAPull() *world {
	repo := fromAHostileRepository()
	repo.pullFound = false

	return repo
}

func TestNamesARepositoryWritesReachTheTerminalAsText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo func() *world
		dry  bool
		keys []string
		// shows is the repository's name as it is drawn, the sequence taken out.
		shows string
	}{
		"the Issues view":    {repo: fromAHostileRepository, keys: []string{"1"}, shows: "1 Issues · mine"},
		"the Messaging pane": {repo: fromAHostileRepository, keys: []string{"5"}, shows: "to     " + devChannel},
		"the announcement":   {repo: fromAHostileRepository, keys: []string{"5", "p"}, shows: "to  " + devChannel},
		"the Summary's post": {
			repo: fromAHostileRepository, keys: []string{summaryKey, postSummaryKey}, shows: "to  " + devChannel + ",",
		},
		"the pull request opened": {repo: hostileWithoutAPull, keys: []string{"4", "n"}, shows: "template bugfix (1 of 1)"},
		"the commit composed":     {repo: fromAHostileRepository, keys: []string{"3", "c"}, shows: "  fix: "},
		"the key list":            {repo: fromAHostileRepository, keys: []string{"?"}, shows: reboundRefreshKey},
		"the follow-ups a dry run names": {
			repo: hostileWithoutAPull, dry: true, keys: []string{"4", "n", keyEnter}, shows: "to move PROJ-412 to In Review",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := tt.repo()

			model := tui.New(repo.cfg, nil, repo.deps())
			if tt.dry {
				model = model.WithDryRun()
			}

			started := sized(t, model, 240, 70)
			started = drain(t, started, started.Init())

			// Act
			view := typing(t, started, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.shows)

			if found := terminalControl.FindString(view); found != "" {
				t.Errorf("the screen carries the terminal control %q:\n%s", found, plain(view))
			}
		})
	}
}
