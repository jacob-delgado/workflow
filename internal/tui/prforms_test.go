// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// heldOwners is deps whose read of the changed paths waits on held, which
// keeps the code owners from answering.
func heldOwners(deps *tui.Deps, held *hold) {
	changed := deps.Git.ChangedPaths
	deps.Git.ChangedPaths = func(ctx context.Context, base string) ([]string, error) {
		held.wait()

		return changed(ctx, base)
	}
}

func TestTheComposerAsksWhereThePullRequestGoesFirst(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withoutPull().live(t, 120, 40), "4", "n").View().Content

	// Assert
	requireScreen(t, view, "Base", "> main")
	refuseScreen(t, view, "Reviewers")
}

func TestTheComposerAsksWhatThePullRequestSaysOnceItsBaseIsChosen(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withoutPull().live(t, 120, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Title", "> "+pullTitle, "Draft", "Reviewers", "Assignees", "Labels")
}

func TestTheComposerOpensItsFormOnceItsReadsHaveAnswered(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHold(t)
	started := heldLive(t, held, ownedBy("ana"), heldOwners)
	held.armed.Store(true)

	// Act
	reading := holding(t, held, started, "4", "n")

	// Assert
	held.requireReached(t)

	view := reading.View().Content
	requireScreen(t, view, "reading what the pull request starts from")
	refuseScreen(t, view, "> main")
}

func TestTheComposerKeepsEscInANarrowFooter(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"asking where it goes": {"4", "n"},
		"asking what it says":  {"4", "n", keyEnter},
	}

	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, withoutPull().live(t, 80, 24), keys...).View().Content

			// Assert
			if footer := footerLine(view); !strings.Contains(footer, escCloses) {
				t.Errorf("the footer = %q, want %q", footer, escCloses)
			}
		})
	}
}

func TestTheComposerHandsEachFormToThePromptsWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each prompt is answered with an empty line, which keeps what it
	// proposed.
	prompts := &answering{typed: strings.Repeat("\n", 5)}
	opening := withoutPull()
	deps := opening.deps()
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(opening.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	// Act
	typing(t, drain(t, model, model.Init()), "4", "n")

	// Assert
	if prompts.asked.Load() != 2 {
		t.Errorf("handed the prompts %d forms, want where it goes and what it says", prompts.asked.Load())
	}

	if calls := opening.asked("open "); len(calls) != 1 || !strings.HasPrefix(calls[0], "open "+pullTitle) {
		t.Errorf("open calls = %q, want the proposed pull request opened", calls)
	}
}

func TestTheComposerSaysWhyItsPromptsStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	deps := opening.deps()
	deps.Forms.Ask = func(_ *huh.Form, done func(error) tea.Msg) tea.Cmd {
		return func() tea.Msg { return done(errPromptsStopped) }
	}
	model := sized(t, tui.New(opening.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	// Act
	stopped := typing(t, drain(t, model, model.Init()), "4", "n")

	// Assert
	requireScreen(t, stopped.View().Content, "the prompts stopped", "Base")

	if calls := opening.asked("open "); len(calls) != 0 {
		t.Errorf("opened %q though the prompts stopped", calls)
	}
}

func TestTheDraftKeyTurnsThePullRequestIntoADraftAsItIsWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()

	// Act
	typing(t, opening.live(t, 120, 40), "4", "n", keyEnter, "ctrl+r", keyEnter)

	// Assert
	if calls := opening.asked("open "); len(calls) != 1 || !strings.Contains(calls[0], "draft=yes") {
		t.Errorf("open calls = %q, want one opened as a draft", calls)
	}
}

func TestTheTemplateKeyMovesTheBodyOnAsThePullRequestIsWritten(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, twoTemplates().live(t, 120, 40), "4", "n", keyEnter, "ctrl+t").View().Content

	// Assert
	requireScreen(t, view, "template regression (2 of 2)", "## What broke", "Title")
}

func TestReviewersTypedWhereNoOwnersAreReadAreSent(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.noCodeOwners = true

	// Act
	typing(t, opening.live(t, 120, 40), "4", "n", keyEnter, keyTab, keyTab, pasted("ana"), keyEnter)

	// Assert
	if calls := opening.asked("open "); len(calls) != 1 || !strings.Contains(calls[0], "reviewers=ana") {
		t.Errorf("open calls = %q, want ana as a reviewer", calls)
	}
}

func TestTabTakesTheBasesCompletionAsItMovesOnToTheTemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := twoTemplates()
	opening.remoteBranches = []string{"develop"}

	// Act
	view := typing(t, opening.live(t, 120, 40), append(baseKeys("dev"), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "→ develop", "Title")
}

func TestTheComposersFormFollowsTheTerminalWhenItIsResized(t *testing.T) {
	t.Parallel()

	// Arrange
	long := "a title long enough to run past a narrowed pane's edge, and some way further on to here"
	saying := typing(t, withoutPull().live(t, 160, 40), "4", "n", keyEnter, keyClearLine, pasted(long))

	// Act
	narrowed, cmd := saying.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	narrow := drain(t, concrete(t, narrowed), cmd)

	// Assert
	// The title scrolls along its one line to the cursor at its end, rather
	// than running past the pane.
	for line := range strings.SplitSeq(plain(narrow.View().Content), "\n") {
		if strings.Contains(line, "┃ ┃ >") && !strings.Contains(line, "to here") {
			t.Errorf("the title's line does not end where it is typed to:\n%s", plain(narrow.View().Content))
		}
	}
}

func TestAResizeWhileTheComposerReadsChangesNothingInIt(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHold(t)
	started := heldLive(t, held, ownedBy("ana"), heldOwners)
	held.armed.Store(true)
	reading := holding(t, held, started, "4", "n")

	// Act
	resized, cmd := reading.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Assert
	if cmd != nil {
		t.Errorf("a resize while the composer reads asked for something")
	}

	requireScreen(t, concrete(t, resized).View().Content, "reading what the pull request starts from")
}
