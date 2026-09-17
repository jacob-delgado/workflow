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
		prepare func(*world)
		keys    []string
		sending string
	}{
		"a branch":       {keys: []string{"b"}, sending: "creating…"},
		"a pull request": {prepare: func(w *world) { w.pullFound = false }, keys: []string{"4", "n"}, sending: "opening…"},
		"a slack post":   {keys: []string{"5", "p"}, sending: "posting…"},
		"a configuration": {
			prepare: func(w *world) { w.gitHooks = legacyHooks() }, keys: nil, sending: "writing…",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sending := newWorld()
			if tt.prepare != nil {
				tt.prepare(sending)
			}

			overlay := typing(t, sending.live(t, 120, 50), tt.keys...)
			inFlight, _ := pressed(t, overlay, keyEnter)

			requireScreen(t, inFlight.View(), tt.sending)
			requireScreen(t, footerLine(inFlight.View()), "ctrl+c quit")
			refuseScreen(t, footerLine(inFlight.View()), "esc")

			// Neither leaving nor sending again does anything until it answers.
			if _, again := pressed(t, inFlight, keyEnter); again != nil {
				t.Error("enter sent again while the first was on its way")
			}

			requireScreen(t, press(t, inFlight, "esc", "v", "e", "w").View(), tt.sending)
		})
	}
}

func TestOverlaysIgnoreKeysThatMeanNothingInThem(t *testing.T) {
	t.Parallel()

	offering := newWorld()
	offering.gitHooks = legacyHooks()
	requireScreen(t, typing(t, offering.live(t, 120, 50), "x").View(), "┏━ No lefthook configuration")

	commenting := newWorld()
	commenting.edited = greeting
	requireScreen(t, typing(t, commenting.live(t, 120, 40), "c", "x").View(), "┏━ Comment on")

	posting := newWorld()
	requireScreen(t, typing(t, posting.live(t, 120, 40), "5", "p", "x").View(), "┏━ Post to Slack")

	// Typing on the type field changes nothing; the type is chosen with arrows.
	typed := typing(t, newWorld().live(t, 120, 40), "3", "c", keyShiftTab, keyShiftTab, "x").View()
	requireScreen(t, typed, "‹feat›")
	refuseScreen(t, typed, "scope   > x", "subject > x")
}

func TestAStructuredConfigurationWithNoScriptsSaysNothingOfScripts(t *testing.T) {
	t.Parallel()

	plain := newWorld()
	plain.gitHooks = []hooks.GitHook{{Name: "pre-commit", Script: "#!/bin/sh\ngofmt -l .\n"}}

	view := plain.live(t, 120, 50).View()
	requireScreen(t, view, "Found 1 hook in .git/hooks")
	refuseScreen(t, view, "kept whole")
}

func TestADryRunCommentIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.edited = greeting

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "c", keyEnter).View(), "dry run: would comment on PROJ-412")

	if calls := dry.asked("comment"); len(calls) != 0 {
		t.Errorf("a dry run commented: %q", calls)
	}
}

func TestTheComposerKeepsItsDraftWhenLeft(t *testing.T) {
	t.Parallel()

	left := typing(t, newWorld().live(t, 120, 40), append(append([]string{"3", "c"}, letters("half done")...), "esc")...)
	requireScreen(t, typing(t, left, "c").View(), "subject > half done")

	// On a branch that names no issue there is no trailer.
	unnamed := newWorld()
	unnamed.branch.Name = "spike"
	refuseScreen(t, typing(t, unnamed.live(t, 120, 40), "3", "c").View(), "Refs:")
}

func TestABodyThatCannotBeEditedLeavesTheComposerAsItWas(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.editErr = errEditorFailed

	requireScreen(t, typing(t, failing.live(t, 120, 40), "3", "c", "ctrl+e").View(), "┏━ Commit",
		"✗ the editor exited with an error")

	opening := withoutPull()
	opening.editErr = errEditorFailed
	requireScreen(t, typing(t, opening.live(t, 120, 40), "4", "n", "ctrl+e").View(), "┏━ Open pull request",
		"✗ the editor exited with an error")

	noEditor := newWorld().deps()
	noEditor.Editor = tui.EditorDeps{}

	model := sized(t, tui.New(completeConfig(), nil, noEditor), 120, 40)
	model = drain(t, model, model.Init())
	requireScreen(t, typing(t, model, "3", "c", "ctrl+e").View(), "no body yet")
}

func TestThePullRequestComposerWorksWithoutTemplatesOrAnEditor(t *testing.T) {
	t.Parallel()

	bare := withoutPull()
	deps := bare.deps()
	deps.Forge.Templates, deps.Editor = nil, tui.EditorDeps{}

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	model = drain(t, model, model.Init())

	composer := typing(t, model, "4", "n", "ctrl+e", "ctrl+t", keyTab, keyTab)
	requireScreen(t, composer.View(), "no template in this repository", "▸ title  >")
}

func TestTheReviewPaneSaysWhatItIsWaitingFor(t *testing.T) {
	t.Parallel()

	unfound := newWorld().deps()
	unfound.Forge.FindPullRequest = nil

	model := sized(t, tui.New(completeConfig(), nil, unfound), 120, 40)
	requireScreen(t, typing(t, drain(t, model, model.Init()), "4").View(), "looking…")

	unchecked := newWorld().deps()
	unchecked.Forge.CheckStatus = nil

	model = sized(t, tui.New(completeConfig(), nil, unchecked), 120, 40)
	requireScreen(t, typing(t, drain(t, model, model.Init()), "4").View(), "CI     checking…")

	failing := newWorld()
	failing.ciErr = errUnreachable
	requireScreen(t, typing(t, failing.live(t, 120, 40), "4").View(), "CI     ✗ could not reach the forge")
}

func TestAnAnnouncementWithoutAnAuthorStillAnnounces(t *testing.T) {
	t.Parallel()

	anonymous := newWorld()
	anonymous.authorErr = errUnreachable

	requireScreen(t, typing(t, anonymous.live(t, 120, 40), "5").View(), "A pull request is ready for review:")
}

func TestAPostWaitingForCIThatNeverReportsKeepsWaiting(t *testing.T) {
	t.Parallel()

	unreported := newWorld()
	unreported.ciInterval = time.Millisecond
	unreported.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CINone}}

	waiting := typing(t, unreported.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, typing(t, waiting, "j").View(), "state  ◐ posts when CI passes")

	if calls := unreported.asked("post "); len(calls) != 0 {
		t.Errorf("posted with no CI reported: %q", calls)
	}
}

func TestAPostWaitingForCIThatFailsToPostSaysWhy(t *testing.T) {
	t.Parallel()

	refusing := newWorld()
	refusing.ciInterval = time.Millisecond
	refusing.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	refusing.postErr = errNotInChannel

	failed := typing(t, refusing.live(t, 120, 40), "5", "p", "w")
	requireScreen(t, failed.View(), "✗ the credential was not accepted: not_in_channel")
}

func TestACommitThatCannotStartSaysWhy(t *testing.T) {
	t.Parallel()

	unstartable := newWorld()
	unstartable.commitStartErr = errDiskFull

	view := typing(t, unstartable.live(t, 120, 40), commitKeys("x")...).View()
	requireScreen(t, view, "┏━ git commit", "✗ writing the commit message: disk full")
}

func TestARunsFailuresMoveBothWaysAndNeedAnEditor(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{
		"┃  lint ❯ ", "a.go:1:1: first", "b.go:2:1: second",
		"summary: (done in 1 seconds)", "🥊 lint (1 seconds)",
	}

	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)
	requireScreen(t, typing(t, failed, "j", "k").View(), "▸ a.go:1 first")

	// With jobs listed above them, the list starts a row lower.
	requireScreen(t, click(t, failed, 60, 6).View(), "▸ b.go:2 second")

	noEditor := failing.deps()
	noEditor.Editor.Open = nil

	model := sized(t, tui.New(completeConfig(), nil, noEditor), 120, 40)
	model = drain(t, model, model.Init())
	requireScreen(t, typing(t, model, commitKeys("x", keyEnter)...).View(),
		"▸ a.go:1 first")
}

func TestClicksThatLandOnNothingDoNothing(t *testing.T) {
	t.Parallel()

	screen := newWorld().live(t, 120, 40)

	// Below the issues in the rail, on the spine, on the Branch pane's detail.
	requireScreen(t, click(t, screen, 5, 15).View(), "▸ ◐ PROJ-412")
	requireScreen(t, click(t, screen, 50, 0).View(), focused("1 Issues"))
	requireScreen(t, click(t, typing(t, screen, "2"), 60, 5).View(), focused("2 Branch"))
}

func TestTheHelpScrollsBackUp(t *testing.T) {
	t.Parallel()

	help := typing(t, newWorld().live(t, 120, 20), "?", "j", "j", "k", "k")
	requireScreen(t, help.View(), "Moving around")
}

func TestAnInterfaceWithoutAClockUsesTheRealOne(t *testing.T) {
	t.Parallel()

	deps := newWorld().deps()
	deps.Clock = nil
	deps.Jira.Issue = func(string) (jira.IssueDetail, error) {
		return jira.IssueDetail{
			Issue: jira.Issue{Key: issueKey}, CommentTotal: 1,
			Comments: []jira.Comment{{Author: "Ana", Body: "hi", Created: time.Now().Add(-5 * time.Minute)}},
		}, nil
	}

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	requireScreen(t, drain(t, model, model.Init()).View(), "Ana · 5m ago")
}

func TestPickingAnOptionMovesBothWays(t *testing.T) {
	t.Parallel()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	requireScreen(t, typing(t, resolving.live(t, 120, 40), "t", keyEnter, "j", "k").View(), "▸ Fixed")
}

func TestRReadsTheChangesAgain(t *testing.T) {
	t.Parallel()

	refreshing := newWorld()
	refreshing.branch = gitrepo.Branch{Name: featureName, Base: baseRef}
	typing(t, refreshing.live(t, 120, 40), "3", "r")

	if reads := refreshing.asked("changes"); len(reads) < 2 {
		t.Errorf("read the changes %d times, want again on r", len(reads))
	}
}
