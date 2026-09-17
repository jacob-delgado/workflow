// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// Errors git answers with.
var (
	errBranchExists = errors.New("fatal: a branch named 'fix/x' already exists")
	errPushDenied   = errors.New("exit status 128")
)

// dryInterface is the world's interface in dry-run mode.
func dryInterface(w *world) tui.Model {
	return tui.New(completeConfig(), nil, w.deps()).WithDryRun()
}

func TestTheBranchPaneSaysWhereTheBranchStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch gitrepo.Branch
		want   []string
	}{
		"pushed with nothing new": {
			branch: gitrepo.Branch{Name: featureName, Upstream: "origin/" + featureName, Base: baseRef},
			want:   []string{featureName, "pushed"},
		},
		"ahead of origin": {
			branch: gitrepo.Branch{Name: featureName, Upstream: "origin/" + featureName, Ahead: 2, Behind: 1, Base: baseRef},
			want:   []string{"↑2 ↓1 against origin/" + featureName},
		},
		"never pushed": {
			branch: gitrepo.Branch{Name: featureName, Base: baseRef},
			want:   []string{"not pushed yet"},
		},
		"detached": {
			branch: gitrepo.Branch{Detached: true},
			want:   []string{"detached HEAD", "press b to start one"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			standing := newWorld()
			standing.branch = tt.branch

			requireScreen(t, typing(t, standing.live(t, 120, 40), "2").View(), tt.want...)
		})
	}
}

func TestTheBranchDetailNamesItsIssue(t *testing.T) {
	t.Parallel()

	view := typing(t, newWorld().live(t, 120, 40), "2").View()
	requireScreen(t, view, "base      origin/main", "commits   1 not on the base", "issue     PROJ-412 "+issueSummary)

	unlisted := newWorld()
	unlisted.branch.Name = "fix/OTHER-9-thing"
	requireScreen(t, typing(t, unlisted.live(t, 120, 40), "2").View(), "(not among your open issues)")

	baseless := newWorld()
	baseless.branch.Base = ""
	requireScreen(t, typing(t, baseless.live(t, 120, 40), "2").View(), "base      (none found)")
}

func TestABranchThatCannotBeReadSaysSo(t *testing.T) {
	t.Parallel()

	deps := newWorld().deps()
	deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errNotVisible }

	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	requireScreen(t, typing(t, drain(t, model, model.Init()), "2").View(), "✗ not a git repository")

	// Before it loads, the pane says so.
	requireScreen(t, press(t, sized(t, tui.New(completeConfig(), nil, deps), 120, 40), "2").View(), "loading…")
}

func TestBOpensABranchNamedForTheSelectedIssue(t *testing.T) {
	t.Parallel()

	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName, Base: baseRef}

	creator := typing(t, branching.live(t, 120, 40), "b")
	requireScreen(t, creator.View(), "┏━ New branch", "for PROJ-412 "+issueSummary, "> fix/PROJ-412-fix-token-redaction",
		"from origin/main", "enter create")

	created := typing(t, creator, keyEnter)
	requireScreen(t, created.View(), "● switched to fix/PROJ-412-fix-token-redaction")

	if calls := branching.asked("create"); len(calls) != 1 || calls[0] != "create "+featureName+" from origin/main" {
		t.Errorf("create calls = %q", calls)
	}

	if reads := branching.asked("branch"); len(reads) < 2 {
		t.Error("the branch was not read again after it was created")
	}
}

func TestTheBranchNameIsCheckedAsItIsTyped(t *testing.T) {
	t.Parallel()

	branching := newWorld()

	spaced := typing(t, branching.live(t, 120, 40), append([]string{"b"}, letters(" x")...)...)
	requireScreen(t, spaced.View(), "✗ not a valid branch name: it contains a space")

	refused := typing(t, spaced, keyEnter)
	requireScreen(t, refused.View(), "┏━ New branch")

	if calls := branching.asked("create"); len(calls) != 0 {
		t.Errorf("created a branch git would refuse: %q", calls)
	}
}

func TestABranchWithoutABaseStartsFromHere(t *testing.T) {
	t.Parallel()

	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName}
	branching.issues = nil

	creator := typing(t, branching.live(t, 120, 40), "2", "b")
	requireScreen(t, creator.View(), "from the current commit (no default branch found)")
	refuseScreen(t, creator.View(), "for PROJ")

	created := typing(t, creator, append(letters("spike"), keyEnter)...)
	requireScreen(t, created.View(), "● switched to spike")

	if calls := branching.asked("create"); len(calls) != 1 || calls[0] != "create spike from " {
		t.Errorf("create calls = %q", calls)
	}
}

func TestARefusedBranchKeepsTheCreatorOpen(t *testing.T) {
	t.Parallel()

	branching := newWorld()
	branching.createErr = errBranchExists

	refused := typing(t, branching.live(t, 120, 40), "b", keyEnter)
	requireScreen(t, refused.View(), "┏━ New branch", "✗ fatal: a branch named 'fix/x' already exists")

	requireScreen(t, typing(t, refused, "esc").View(), focused("1 Issues"))
}

func TestADryRunBranchIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "b", keyEnter).View(),
		"dry run: would create fix/PROJ-412-fix-token-redaction from origin/main")

	if calls := dry.asked("create"); len(calls) != 0 {
		t.Errorf("a dry run created a branch: %q", calls)
	}
}

func TestPushIsOfferedOnlyWithSomethingToPush(t *testing.T) {
	t.Parallel()

	pushable := newWorld()
	pushable.branch.Ahead = 1

	view := typing(t, pushable.live(t, 120, 40), "2").View()
	requireScreen(t, footerLine(view), "P push", "b new branch")

	upToDate := newWorld()
	pushed := typing(t, upToDate.live(t, 120, 40), "2")
	refuseScreen(t, footerLine(pushed.View()), "P push")

	// A key not offered does nothing.
	typing(t, pushed, "P")

	if calls := upToDate.asked("push"); len(calls) != 0 {
		t.Errorf("P pushed a branch with nothing to push: %q", calls)
	}
}

func TestPushStreamsGitAndReportsTheResult(t *testing.T) {
	t.Parallel()

	pushing := newWorld()
	pushing.branch.Ahead = 1
	pushing.pushLines = []string{"To github.com:example/repo.git", "   1a2b3c4..5d6e7f8  " + featureName}

	done := typing(t, pushing.live(t, 120, 40), "2", "P")
	requireScreen(t, done.View(), "● pushed "+featureName)

	if calls := pushing.asked("push"); len(calls) != 1 {
		t.Errorf("push calls = %q", calls)
	}

	failing := newWorld()
	failing.branch.Ahead = 1
	failing.pushLines = []string{"remote: Permission to example/repo.git denied."}
	failing.pushErr = errPushDenied

	failed := typing(t, failing.live(t, 120, 40), "2", "P")
	requireScreen(t, failed.View(), "┏━ git push", "✗ exit status 128", "remote: Permission to example/repo.git denied.")
}

func TestADryRunPushIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.branch.Ahead = 1

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "2", "P").View(), "dry run: would push "+featureName)

	if calls := dry.asked("push"); len(calls) != 0 {
		t.Errorf("a dry run pushed: %q", calls)
	}
}
