// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// greeting is a comment's whole text.
const greeting = "hello"

// Errors the fakes answer with.
var (
	errUnreachable = errors.New("could not reach the forge")
	errDiskFull    = errors.New("writing the commit message: disk full")
)

func TestNothingInterruptsAWriteBeingSent(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pullMissing bool
		gitHooks    []hooks.GitHook
		keys        []string
		sending     string
	}{
		"a branch":        {keys: []string{"b"}, sending: "creating…"},
		"a pull request":  {pullMissing: true, keys: []string{"4", "n"}, sending: "opening…"},
		"a slack post":    {keys: []string{"5", "p"}, sending: "posting…"},
		"a configuration": {gitHooks: legacyHooks(), keys: nil, sending: "writing…"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			sending := newWorld()
			sending.pullFound, sending.gitHooks = !tt.pullMissing, tt.gitHooks
			overlay := typing(t, sending.live(t, 120, 50), tt.keys...)

			// Act: send it
			inFlight, _ := pressed(t, overlay, keyEnter)

			// Assert: it is on its way, and only quitting is offered
			requireScreen(t, inFlight.View(), tt.sending)
			requireScreen(t, footerLine(inFlight.View()), "ctrl+c quit")
			refuseScreen(t, footerLine(inFlight.View()), keyEsc)

			// Act & Assert: neither leaving nor sending again does anything until it answers
			for _, key := range []string{keyEnter, keyEsc, "v", "e", "w"} {
				after, cmd := pressed(t, inFlight, key)
				if cmd != nil || after.View() != inFlight.View() {
					t.Errorf("%s did something while the write was on its way:\n%s", key, after.View())
				}
			}
		})
	}
}

func TestOverlaysIgnoreKeysThatMeanNothingInThem(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		gitHooks []hooks.GitHook
		edited   string
		keys     []string
	}{
		"the lefthook offer": {gitHooks: legacyHooks()},
		"a comment preview":  {edited: greeting, keys: []string{"c"}},
		"the post to Slack":  {keys: []string{"5", "p"}},
		// The type is chosen with arrows, so typing on it changes nothing.
		"the commit type field": {keys: []string{"3", "c", keyShiftTab, keyShiftTab}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			faked.gitHooks, faked.edited = tt.gitHooks, tt.edited
			overlay := typing(t, faked.live(t, 120, 50), tt.keys...)

			// Act
			after := typing(t, overlay, "x")

			// Assert
			if after.View() != overlay.View() {
				t.Errorf("x changed %s:\n%s", name, after.View())
			}
		})
	}
}

func TestAStructuredConfigurationWithNoScriptsSaysNothingOfScripts(t *testing.T) {
	t.Parallel()

	// Arrange
	plain := newWorld()
	plain.gitHooks = []hooks.GitHook{{Name: "pre-commit", Script: "#!/bin/sh\ngofmt -l .\n"}}

	// Act
	view := plain.live(t, 120, 50).View()

	// Assert
	requireScreen(t, view, "Found 1 hook in .git/hooks")
	refuseScreen(t, view, "kept whole")
}

func TestADryRunCommentIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.edited = greeting
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "c", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would comment on PROJ-412")

	if calls := dry.asked("comment"); len(calls) != 0 {
		t.Errorf("a dry run commented: %q", calls)
	}
}

func TestTheComposerKeepsItsDraftWhenLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	left := typing(t, newWorld().live(t, 120, 40), append(append([]string{"3", "c"}, letters("half done")...), keyEsc)...)

	// Act
	view := typing(t, left, "c").View()

	// Assert
	requireScreen(t, view, "subject > half done")
}

func TestAComposerOnABranchNamingNoIssueHasNoTrailer(t *testing.T) {
	t.Parallel()

	// Arrange
	unnamed := newWorld()
	unnamed.branch.Name = "spike"

	// Act
	view := typing(t, unnamed.live(t, 120, 40), "3", "c").View()

	// Assert
	requireScreen(t, view, "┏━ Commit")
	refuseScreen(t, view, "Refs:")
}

func TestABodyThatCannotBeEditedLeavesTheComposerAsItWas(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pullMissing bool
		keys        []string
		title       string
	}{
		"the commit composer":       {keys: []string{"3", "c"}, title: "┏━ Commit"},
		"the pull request composer": {pullMissing: true, keys: []string{"4", "n"}, title: "┏━ Open pull request"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failing := newWorld()
			failing.pullFound, failing.editErr = !tt.pullMissing, errEditorFailed
			composer := typing(t, failing.live(t, 120, 40), tt.keys...)

			// Act
			view := typing(t, composer, "ctrl+o").View()

			// Assert
			requireScreen(t, view, tt.title, "✗ the editor exited with an error")
		})
	}
}

func TestWithoutAnEditorTheBodyCannotBeEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	noEditor := newWorld().deps()
	noEditor.Editor = tui.EditorDeps{}
	model := sized(t, tui.New(completeConfig(), nil, noEditor), 120, 40)
	composer := typing(t, drain(t, model, model.Init()), "3", "c")

	// Act
	after, cmd := pressed(t, composer, "ctrl+o")

	// Assert
	if cmd != nil || after.View() != composer.View() {
		t.Errorf("ctrl+e did something with no editor:\n%s", after.View())
	}
}

func TestThePullRequestComposerWorksWithoutTemplatesOrAnEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := withoutPull().deps()
	deps.Forge.Templates, deps.Editor = nil, tui.EditorDeps{}
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	composer := typing(t, model, "4", "n", "ctrl+o", "ctrl+t", keyTab, keyTab)

	// Assert
	requireScreen(t, composer.View(), "no template in this repository", "▸ title  >")
}

func TestTheReviewPaneSaysItIsLookingForAPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	unfound := newWorld().deps()
	unfound.Forge.FindPullRequest = nil
	model := sized(t, tui.New(completeConfig(), nil, unfound), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "4").View()

	// Assert
	requireScreen(t, view, "looking…")
}

func TestTheReviewPaneSaysCIIsBeingChecked(t *testing.T) {
	t.Parallel()

	// Arrange
	unchecked := newWorld().deps()
	unchecked.Forge.CheckStatus = nil
	model := sized(t, tui.New(completeConfig(), nil, unchecked), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "4").View()

	// Assert
	requireScreen(t, view, "CI     checking…")
}

func TestTheReviewPaneSaysWhyCICouldNotBeChecked(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.ciErr = errUnreachable

	// Act
	view := typing(t, failing.live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "CI     ✗ could not reach the forge")
}

func TestAnAnnouncementWithoutAnAuthorStillAnnounces(t *testing.T) {
	t.Parallel()

	// Arrange
	anonymous := newWorld()
	anonymous.authorErr = errUnreachable

	// Act
	view := typing(t, anonymous.live(t, 120, 40), "5").View()

	// Assert
	requireScreen(t, view, "A pull request is ready for review:")
}

func TestAPostWaitingForCIThatNeverReportsKeepsWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	unreported := newWorld()
	unreported.ciInterval = time.Millisecond
	unreported.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CINone}}

	// Act
	waiting := typing(t, unreported.live(t, 120, 40), "5", "p", "w", "j")

	// Assert
	requireScreen(t, waiting.View(), "state  ◐ posts when CI passes")

	if calls := unreported.asked("post "); len(calls) != 0 {
		t.Errorf("posted with no CI reported: %q", calls)
	}
}

func TestAPostWaitingForCIThatFailsToPostSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := newWorld()
	refusing.ciInterval = time.Millisecond
	refusing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	refusing.postErr = errNotInChannel

	// Act
	failed := typing(t, refusing.live(t, 120, 40), "5", "p", "w")

	// Assert
	requireScreen(t, failed.View(), "✗ the credential was not accepted: not_in_channel")
}

func TestACommitThatCannotStartSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	unstartable := newWorld()
	unstartable.commitStartErr = errDiskFull

	// Act
	view := typing(t, unstartable.live(t, 120, 40), commitKeys("x")...).View()

	// Assert
	requireScreen(t, view, "┏━ git commit", "✗ writing the commit message: disk full")
}

func TestClicksThatLandOnNothingDoNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys        []string
		column, row int
	}{
		"below the issues in the rail": {column: 5, row: 15},
		"on the spine":                 {column: 50, row: 0},
		"on the Branch pane's detail":  {keys: []string{"2"}, column: 60, row: 5},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			screen := typing(t, newWorld().live(t, 120, 40), tt.keys...)

			// Act
			clicked := click(t, screen, tt.column, tt.row)

			// Assert
			if clicked.View() != screen.View() {
				t.Errorf("a click %s changed the screen:\n%s", name, clicked.View())
			}
		})
	}
}

func TestTheHelpScrollsBackUp(t *testing.T) {
	t.Parallel()

	// Arrange
	help := typing(t, newWorld().live(t, 120, 20), "?")

	// Act: scroll down
	down := typing(t, help, "j", "j")

	// Assert: the first group is out of sight
	refuseScreen(t, down.View(), "Moving around")

	// Act: scroll back up
	up := typing(t, down, "k", "k")

	// Assert: it is back
	requireScreen(t, up.View(), "Moving around")
}

func TestAnInterfaceWithoutAClockUsesTheRealOne(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newWorld().deps()
	deps.Clock = nil
	deps.Jira.Issue = func(string) (jira.IssueDetail, error) {
		return jira.IssueDetail{
			Issue: jira.Issue{Key: issueKey}, CommentTotal: 1,
			Comments: []jira.Comment{{Author: "Ana", Body: "hi", Created: time.Now().Add(-5 * time.Minute)}},
		}, nil
	}

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)

	// Act
	view := drain(t, model, model.Init()).View()

	// Assert
	requireScreen(t, view, "Ana · 5m ago")
}

func TestPickingAnOptionMovesBothWays(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	form := typing(t, resolving.live(t, 120, 40), "t", keyEnter)

	// Act: j
	down := typing(t, form, "j")

	// Assert: the second option is selected
	requireScreen(t, down.View(), "▸ Won't Fix")

	// Act: k
	up := typing(t, down, "k")

	// Assert: the first is selected again
	requireScreen(t, up.View(), "▸ Fixed")
}

func TestRReadsTheChangesAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	refreshing := newWorld()
	refreshing.branch = gitrepo.Branch{Name: featureName, Base: baseRef}
	pane := typing(t, refreshing.live(t, 120, 40), "3")
	before := len(refreshing.asked("changes"))

	// Act
	typing(t, pane, "r")

	// Assert
	if reads := len(refreshing.asked("changes")); reads != before+1 {
		t.Errorf("read the changes %d times, want once more than the %d before r", reads, before)
	}
}
