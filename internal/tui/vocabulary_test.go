// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// An act keeps one verb from the key that starts it to the notice that says it
// is done: each case reaches the act, reads the label its key wears, then does
// it and reads the notice, which must say the same verb.
func TestAnActKeepsItsVerbFromItsKeyToItsNotice(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo         func() *world
		reach        []string
		act          string
		label, notes string
	}{
		"saving a pull request edit": {
			repo: newWorld, reach: []string{"4", "e"}, act: keyEnter,
			label: "enter save", notes: "saved #42",
		},
		"starting work in a new worktree": {
			repo: newWorld, reach: []string{"2", "b", keyWorktree}, act: keyEnter,
			label: "enter create worktree", notes: "created worktree for " + featureName,
		},
		"linking a branch and its description": {
			repo: func() *world { return onOffConventionBranch(true) }, reach: []string{"2", "i", keyEnter}, act: keyEnter,
			label: "enter link and update the description", notes: "linked my-thing to " + issueKey + " and updated #42",
		},
		"marking a task done": {
			repo: withTasks, reach: []string{tasksPane, downAction}, act: "d",
			label: offersDone, notes: "marked 3 done",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reached := typing(t, tt.repo().live(t, 160, 40), tt.reach...)

			// Act
			done := typing(t, reached, tt.act).View().Content

			// Assert
			requireScreen(t, footerLine(reached.View().Content), tt.label)
			requireScreen(t, done, tt.notes)
		})
	}
}

// A last look asks before it acts: its body is a question.
func TestEveryLastLookAsksAQuestion(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo  func() *world
		keys  []string
		title string
	}{
		"the push":   {repo: unpushedWorld, keys: []string{"2", "P"}, title: "Push branch"},
		"the rebase": {repo: newWorld, keys: []string{"2", "u"}, title: "Rebase branch"},
		"the re-run": {repo: failedChecks, keys: []string{"4", "R"}, title: "Re-run checks"},
		"the amend":  {repo: unpushedWorld, keys: []string{"3", "A"}, title: "Amend the last commit"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := plain(typing(t, tt.repo().live(t, 160, 40), tt.keys...).View().Content)

			// Assert
			body := lastLookBody(t, view, tt.title)
			if !strings.HasSuffix(body, "?") {
				t.Errorf("%s look says %q, want a question", name, body)
			}
		})
	}
}

// failedChecks is a pull request whose one check failed, so its checks can be
// run again.
func failedChecks() *world {
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}

	return repo
}

// lastLookBody is the first line of words under the look's title, inside the
// detail's border.
func lastLookBody(t *testing.T, view, title string) string {
	t.Helper()

	lines := strings.Split(view, "\n")
	for index, line := range lines {
		if !strings.Contains(line, title) {
			continue
		}

		for _, below := range lines[index+1:] {
			_, detail, _ := strings.Cut(below, "┃")
			if words := strings.Trim(detail, " ┃"); words != "" {
				return words
			}
		}
	}

	t.Fatalf("no look titled %q:\n%s", title, view)

	return ""
}
