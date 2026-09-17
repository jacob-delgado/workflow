// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
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

func TestOutsideARepositoryEachRepoPaneSaysSoAndOffersNoRepoKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pane     string
		unwanted string
	}{
		"Branch":  {pane: "2", unwanted: "b new branch"},
		"Commits": {pane: "3", unwanted: "h run pre-commit"},
		"Review":  {pane: "4", unwanted: "n open pull request"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := newWorld().deps()
			deps.Git.Branch = func() (gitrepo.Branch, error) {
				return gitrepo.Branch{}, fmt.Errorf("%w: /home/example", gitrepo.ErrNotARepository)
			}
			deps.Git.Changes = func() ([]gitrepo.Change, error) {
				return nil, fmt.Errorf("reading the status: %w", gitrepo.ErrNotARepository)
			}
			model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, tt.pane).View()

			// Assert
			requireScreen(t, view, "Not inside a git repository")
			refuseScreen(t, footerLine(view), tt.unwanted)
		})
	}
}

func TestTheBranchPaneSaysWhereTheBranchStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch gitrepo.Branch
		want   []string
	}{
		"pushed with nothing new": {
			branch: gitrepo.Branch{Name: featureName, Upstream: "origin/" + featureName, Base: baseRef},
			want:   []string{featureName, "upstream  pushed"},
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

			// Arrange
			standing := newWorld()
			standing.branch = tt.branch

			// Act
			view := typing(t, standing.live(t, 120, 40), "2").View()

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}

func TestTheBranchDetailNamesItsIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		base string
		want []string
	}{
		"an issue among yours": {
			name: featureName, base: baseRef,
			want: []string{"base      origin/main", "commits   1 not on the base", "issue     PROJ-412 " + issueSummary},
		},
		"an issue not among yours": {
			name: "fix/OTHER-9-thing", base: baseRef, want: []string{"(not among your open issues)"},
		},
		"no base": {name: featureName, base: "", want: []string{"base      (none found)"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			naming := newWorld()
			naming.branch.Name, naming.branch.Base = tt.name, tt.base

			// Act
			view := typing(t, naming.live(t, 120, 40), "2").View()

			// Assert
			requireScreen(t, view, tt.want...)
		})
	}
}

func TestABranchThatCannotBeReadSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newWorld().deps()
	deps.Git.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errNotVisible }
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "2").View()

	// Assert
	// The rail says that it failed and the detail says why, in git's own words:
	// a repository that cannot be read is one reason among several. And nothing
	// offers a branch, which could not be made either.
	requireScreen(t, view, "✗ could not read the branch · see detail", "✗ "+errNotVisible.Error())
	refuseScreen(t, view, "not a git repository", "press b to start one")
}

func TestTheBranchPaneSaysSoBeforeTheBranchLoads(t *testing.T) {
	t.Parallel()

	// Arrange
	model := sized(t, tui.New(completeConfig(), nil, newWorld().deps()), 120, 40)

	// Act
	view := press(t, model, "2").View()

	// Assert
	// Other panes are loading too; the heavy border is the Branch pane's own.
	requireScreen(t, view, focused("2 Branch"), "┃ loading…")
}

func TestBOpensABranchNamedForTheSelectedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	model := branching.live(t, 120, 40)

	// Act: open the creator
	creator := typing(t, model, "b")

	// Assert: it proposes a name for the issue, from the base
	requireScreen(t, creator.View(), "┏━ New branch", "for PROJ-412 "+issueSummary, "> fix/PROJ-412-fix-token-redaction",
		"from origin/main", "enter create")

	// Act: create it
	created := typing(t, creator, keyEnter)

	// Assert: git created it, and the branch was read again
	requireScreen(t, created.View(), "● created and switched to fix/PROJ-412-fix-token-redaction")

	if calls := branching.asked("create"); len(calls) != 1 || calls[0] != "create "+featureName+" from origin/main" {
		t.Errorf("create calls = %q", calls)
	}

	if reads := branching.asked("branch"); len(reads) != 2 {
		t.Errorf("the branch was read %d times, want once at start and once after it was created", len(reads))
	}
}

func TestTheBranchNameIsCheckedAsItIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	model := branching.live(t, 120, 40)

	// Act: type a name git would refuse
	spaced := typing(t, model, append([]string{"b"}, letters(" x")...)...)

	// Assert: the problem is named
	requireScreen(t, spaced.View(), "✗ not a valid branch name: it contains a space")

	// Act: try to create it
	refused := typing(t, spaced, keyEnter)

	// Assert: nothing is created, and the creator stays open
	requireScreen(t, refused.View(), "┏━ New branch")

	if calls := branching.asked("create"); len(calls) != 0 {
		t.Errorf("created a branch git would refuse: %q", calls)
	}
}

func TestABranchWithoutABaseStartsFromHere(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName}
	branching.issues = nil
	model := branching.live(t, 120, 40)

	// Act: open the creator with no issue and no base
	creator := typing(t, model, "2", "b")

	// Assert: it starts from the current commit, for no issue
	requireScreen(t, creator.View(), "from the current commit (no default branch found)")
	refuseScreen(t, creator.View(), "for PROJ")

	// Act: name and create it
	created := typing(t, creator, append(letters("spike"), keyEnter)...)

	// Assert: git created it from here
	requireScreen(t, created.View(), "● created and switched to spike")

	if calls := branching.asked("create"); len(calls) != 1 || calls[0] != "create spike from " {
		t.Errorf("create calls = %q", calls)
	}
}

func TestARefusedBranchKeepsTheCreatorOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.createErr = errBranchExists
	model := branching.live(t, 120, 40)

	// Act: create a branch git refuses
	refused := typing(t, model, "b", keyEnter)

	// Assert: the creator stays open with git's reason
	requireScreen(t, refused.View(), "┏━ New branch", "✗ fatal: a branch named 'fix/x' already exists")

	// Act: close it
	closed := typing(t, refused, keyEsc).View()

	// Assert: the keyboard is back on the pane it came from
	refuseScreen(t, closed, "┏━ New branch")
	requireScreen(t, closed, focused("1 Issues"))
}

func TestADryRunBranchIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "b", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would create fix/PROJ-412-fix-token-redaction from origin/main")

	if calls := dry.asked("create"); len(calls) != 0 {
		t.Errorf("a dry run created a branch: %q", calls)
	}
}

func TestPushIsOfferedWithSomethingToPush(t *testing.T) {
	t.Parallel()

	// Arrange
	pushable := newWorld()
	pushable.branch.Ahead = 1

	// Act
	view := typing(t, pushable.live(t, 120, 40), "2").View()

	// Assert
	requireScreen(t, footerLine(view), "P push", "b new branch")
}

func TestPushIsNotOfferedWithNothingToPush(t *testing.T) {
	t.Parallel()

	// Arrange
	upToDate := newWorld()
	model := upToDate.live(t, 120, 40)

	// Act: open the Branch pane
	pane := typing(t, model, "2")

	// Assert: push is not offered
	refuseScreen(t, footerLine(pane.View()), "P push")

	// Act: press it anyway
	typing(t, pane, "P")

	// Assert: a key not offered does nothing
	if calls := upToDate.asked("push"); len(calls) != 0 {
		t.Errorf("P pushed a branch with nothing to push: %q", calls)
	}
}

func TestPushIsPreviewedBeforeItIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	pushing := newWorld()
	pushing.branch.Ahead = 1

	// Act: press P
	preview := typing(t, pushing.live(t, 120, 40), "2", "P")

	// Assert: what will be pushed is shown, nothing pushed yet
	requireScreen(t, preview.View(), "push "+featureName+" to origin", "enter push")

	if calls := pushing.asked("push"); len(calls) != 0 {
		t.Errorf("pushed before the preview was confirmed: %q", calls)
	}

	// Act: back out
	typing(t, preview, keyEsc)

	// Assert: still nothing pushed
	if calls := pushing.asked("push"); len(calls) != 0 {
		t.Errorf("esc pushed the branch: %q", calls)
	}
}

func TestPushReportsTheBranchPushed(t *testing.T) {
	t.Parallel()

	// Arrange
	pushing := newWorld()
	pushing.branch.Ahead = 1
	pushing.pushLines = []string{"To github.com:example/repo.git", "   1a2b3c4..5d6e7f8  " + featureName}

	// Act
	done := typing(t, pushing.live(t, 120, 40), "2", "P", keyEnter)

	// Assert
	requireScreen(t, done.View(), "● pushed "+featureName)

	if calls := pushing.asked("push"); len(calls) != 1 || calls[0] != "push "+featureName {
		t.Errorf("push calls = %q, want the branch pushed once", calls)
	}
}

func TestARefusedPushShowsWhatGitSaid(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.branch.Ahead = 1
	failing.pushLines = []string{"remote: Permission to example/repo.git denied."}
	failing.pushErr = errPushDenied

	// Act
	failed := typing(t, failing.live(t, 120, 40), "2", "P", keyEnter)

	// Assert
	requireScreen(t, failed.View(), "┏━ git push", "✗ exit status 128", "remote: Permission to example/repo.git denied.")
}

func TestADryRunPushIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.branch.Ahead = 1
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "P", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would push "+featureName)

	if calls := dry.asked("push"); len(calls) != 0 {
		t.Errorf("a dry run pushed: %q", calls)
	}
}

func TestBranchingForAnIssueNamesTheIssueThroughout(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	onIssues := branching.live(t, 160, 40)

	// Act & Assert: the Issues pane key names the issue
	requireScreen(t, footerLine(onIssues.View()), "b branch for PROJ-412")

	// Act: open the creator
	creator := typing(t, onIssues, "b")

	// Assert: its title names the issue
	requireScreen(t, creator.View(), "┏━ New branch for PROJ-412")

	// Act: create the branch
	created := typing(t, creator, keyEnter)

	// Assert: the notice says both things that happened
	requireScreen(t, created.View(), "● created and switched to fix/PROJ-412-fix-token-redaction")
}
