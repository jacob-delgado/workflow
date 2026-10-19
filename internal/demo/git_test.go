// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// feature is the branch the demo's tests start work on, and mainBranch the
// one it merges into.
const (
	feature    = "fix/PROJ-412-redact-tokens"
	mainBranch = "main"
)

// The morning's changes, as the demo's work tree holds them.
const (
	reqlog     = "internal/wiring/reqlog.go"
	redact     = "internal/config/redact.go"
	reqlogTest = "internal/wiring/reqlog_test.go"
)

// onFeature is the demo's world with work started on the feature branch.
func onFeature(t *testing.T) tui.Deps {
	t.Helper()

	deps := standingStill().Deps()

	err := deps.Git.CreateBranch(t.Context(), feature, "origin/main")
	if err != nil {
		t.Fatalf("creating %s: %v", feature, err)
	}

	return deps
}

// staged stages the changes at paths.
func staged(t *testing.T, deps tui.Deps, paths ...string) {
	t.Helper()

	changes, err := deps.Git.Changes(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	for _, change := range changes {
		if slices.Contains(paths, change.Path) {
			stageErr := deps.Git.Stage(t.Context(), change)
			if stageErr != nil {
				t.Fatalf("staging %s: %v", change.Path, stageErr)
			}
		}
	}
}

// ran reads a finished run's lines and how it ended, failing the test when
// it did not start: ran(t)(deps.Git.Commit(...)).
func ran(t *testing.T) func(proc.Output, error) ([]string, error) {
	t.Helper()

	return func(output proc.Output, err error) ([]string, error) {
		t.Helper()

		if err != nil {
			t.Fatalf("the run did not start: %v", err)
		}

		var lines []string
		for line := range output.Lines {
			lines = append(lines, line)
		}

		return lines, output.Wait()
	}
}

// changeOf is the change at path, failing the test when there is none.
func changeOf(t *testing.T, deps tui.Deps, path string) gitrepo.Change {
	t.Helper()

	changes, err := deps.Git.Changes(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	at := slices.IndexFunc(changes, func(change gitrepo.Change) bool { return change.Path == path })
	if at < 0 {
		t.Fatalf("no change at %s among %+v", path, changes)
	}

	return changes[at]
}

func TestStartingWorkSwitchesToANewBranchWithNothingOnIt(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	err := deps.Git.CreateBranch(t.Context(), feature, "origin/main")
	// Assert
	if err != nil {
		t.Fatalf("creating the branch: %v", err)
	}

	branch, _ := deps.Git.Branch(t.Context())
	if branch.Name != feature || branch.Upstream != "" || len(branch.Commits) != 0 || branch.Base != "origin/main" {
		t.Errorf("the branch after starting work = %+v, want %s on origin/main, unpushed and empty", branch, feature)
	}

	names, _ := deps.Git.Branches(t.Context())
	if !slices.Contains(names, feature) || !slices.IsSorted(names) {
		t.Errorf("the branches = %v, want them sorted with %s among them", names, feature)
	}
}

func TestABranchThatExistsIsNotCreatedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	err := deps.Git.CreateBranch(t.Context(), mainBranch, "origin/main")

	// Assert
	if err == nil {
		t.Error("creating main again succeeded, want it refused")
	}

	if branch, _ := deps.Git.Branch(t.Context()); branch.Name != feature {
		t.Errorf("the refused branch switched the checkout to %s", branch.Name)
	}
}

func TestStagingMovesAChangeIntoTheIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	staged(t, deps, reqlog, reqlogTest)

	// Assert
	if change := changeOf(t, deps, reqlog); change.Staged != 'M' || change.Unstaged != ' ' {
		t.Errorf("the staged modification reads %q%q, want \"M \"", change.Staged, change.Unstaged)
	}

	if change := changeOf(t, deps, reqlogTest); change.Staged != 'A' || change.Unstaged != ' ' {
		t.Errorf("the staged new file reads %q%q, want \"A \"", change.Staged, change.Unstaged)
	}

	if change := changeOf(t, deps, redact); change.Staged != ' ' || change.Unstaged != 'M' {
		t.Errorf("the change left alone reads %q%q, want \" M\"", change.Staged, change.Unstaged)
	}
}

func TestUnstagingMovesAChangeBackToTheWorkTree(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlogTest)

	// Act
	err := deps.Git.Unstage(t.Context(), changeOf(t, deps, reqlogTest))
	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if change := changeOf(t, deps, reqlogTest); change.Staged != '?' || change.Unstaged != '?' {
		t.Errorf("the unstaged new file reads %q%q, want \"??\"", change.Staged, change.Unstaged)
	}
}

func TestAFileWithNoChangeCannotBeStaged(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	err := deps.Git.Stage(t.Context(), gitrepo.Change{Path: "README.md", Staged: ' ', Unstaged: 'M'})

	// Assert
	if err == nil {
		t.Error("staging a file the work tree has no change at succeeded")
	}

	if changes, _ := deps.Git.Changes(t.Context()); len(changes) != 3 {
		t.Errorf("the refused stage changed the work tree to %+v", changes)
	}
}

func TestACommitTakesWhatWasStagedAndLeavesTheRest(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog, reqlogTest)

	// Act
	lines, err := ran(t)(deps.Git.Commit(t.Context(), "fix(config): redact tokens in the request log\n\nWhy."))
	// Assert
	if err != nil {
		t.Fatalf("the commit failed: %v", err)
	}

	if len(lines) == 0 {
		t.Error("the commit printed nothing")
	}

	branch, _ := deps.Git.Branch(t.Context())
	if len(branch.Commits) != 1 || branch.Commits[0].Subject != "fix(config): redact tokens in the request log" ||
		branch.Head != branch.Commits[0].Hash || branch.Ahead != 0 {
		t.Errorf("the branch after the commit = %+v, want the one commit at its head, ahead of no upstream", branch)
	}

	changes, _ := deps.Git.Changes(t.Context())
	if len(changes) != 1 || changes[0].Path != redact {
		t.Errorf("the changes after the commit = %+v, want only %s, which was not staged", changes, redact)
	}
}

func TestACommitWithNothingStagedFails(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	_, err := ran(t)(deps.Git.Commit(t.Context(), "fix: nothing"))

	// Assert
	if err == nil {
		t.Error("a commit with nothing staged succeeded, want it to fail as git's does")
	}

	if branch, _ := deps.Git.Branch(t.Context()); len(branch.Commits) != 0 {
		t.Errorf("the failed commit left %+v on the branch", branch.Commits)
	}
}

func TestEachCommitHasAHashOfItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: one"))
	staged(t, deps, redact)

	// Act
	_, err := ran(t)(deps.Git.Commit(t.Context(), "fix: two"))

	// Assert
	branch, _ := deps.Git.Branch(t.Context())
	if err != nil || len(branch.Commits) != 2 || branch.Commits[0].Hash == branch.Commits[1].Hash {
		t.Errorf("two commits = %+v (%v), want two with hashes of their own", branch.Commits, err)
	}
}

func TestTheBranchCommittedOnIsListedFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))

	err := deps.Git.CreateBranch(t.Context(), "fix/PROJ-420-later", "origin/main")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	names, err := deps.Git.Branches(t.Context())

	// Assert
	if err != nil || len(names) == 0 || names[0] != feature {
		t.Errorf("the branches after a commit on %s = %v (%v), want it first, the latest committed to",
			feature, names, err)
	}
}

func TestANewBranchIsListedAmongThoseStandingAtMain(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))

	err := deps.Git.CreateBranch(t.Context(), "chore/PROJ-430-tidy-logs", "origin/main")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	names, err := deps.Git.Branches(t.Context())

	// Assert
	want := []string{
		feature, "chore/PROJ-430-tidy-logs", "feat/PROJ-418-webhook-retries", "fix/PROJ-408-flaky-timeout", mainBranch,
	}
	if err != nil || !slices.Equal(names, want) {
		t.Errorf("the branches = %v (%v), want %v: the one committed to, then those at main by name", names, err, want)
	}
}

func TestAPushSetsTheUpstreamAndLeavesNothingAhead(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))

	// Act
	lines, err := ran(t)(deps.Git.Push(t.Context(), feature))

	// Assert
	branch, _ := deps.Git.Branch(t.Context())
	if err != nil || !branch.Pushed() || branch.Upstream != "origin/"+feature {
		t.Errorf("the branch after a push = %+v (%v), want it pushed to origin/%s", branch, err, feature)
	}

	if len(lines) == 0 {
		t.Error("the push printed nothing")
	}

	remote, _ := deps.Git.RemoteBranches(t.Context())
	if !slices.Contains(remote, feature) || !slices.Contains(remote, mainBranch) {
		t.Errorf("the remote branches after a push = %v, want main and %s, named without their remote", remote, feature)
	}
}

func TestTheRemoteHoldsMainBeforeAnyPush(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	remote, err := deps.Git.RemoteBranches(t.Context())

	// Assert
	if err != nil || !slices.Equal(remote, []string{mainBranch}) {
		t.Errorf("the remote branches before a push = %v (%v), want [main], named without its remote", remote, err)
	}
}

func TestAPushAfterAPushSendsTheCommitsSince(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))
	_, _ = ran(t)(deps.Git.Push(t.Context(), feature))
	staged(t, deps, redact)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact more"))

	if branch, _ := deps.Git.Branch(t.Context()); branch.Ahead != 1 || branch.Pushed() {
		t.Fatalf("the branch before its second push = %+v, want it one ahead", branch)
	}

	// Act
	lines, err := ran(t)(deps.Git.Push(t.Context(), feature))

	// Assert
	branch, _ := deps.Git.Branch(t.Context())
	if err != nil || !branch.Pushed() || len(branch.Commits) != 2 {
		t.Errorf("the branch after a second push = %+v (%v), want both commits pushed", branch, err)
	}

	if slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, "[new branch]") }) {
		t.Errorf("a second push printed %v, as if the branch were new", lines)
	}
}

func TestABranchThatIsNotThereCannotBePushed(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)

	// Act
	_, err := ran(t)(deps.Git.Push(t.Context(), "fix/nowhere"))

	// Assert
	if err == nil {
		t.Error("pushing a branch that is not there succeeded")
	}

	if remote, _ := deps.Git.RemoteBranches(t.Context()); slices.Contains(remote, "fix/nowhere") {
		t.Errorf("the refused push put the branch on the remote: %v", remote)
	}
}

func TestTodaysSummaryHoldsTheCommitsTheDemoMade(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := onFeature(t)
	staged(t, deps, reqlog)
	_, _ = ran(t)(deps.Git.Commit(t.Context(), "fix: redact"))
	start := deps.Clock().Truncate(24 * time.Hour)

	// Act
	today := deps.Git.CommitsBetween(t.Context(), start, start.Add(24*time.Hour))
	yesterday := deps.Git.CommitsBetween(t.Context(), start.Add(-24*time.Hour), start)

	// Assert
	if len(today[0].Commits) != 1 || today[0].Commits[0].Subject != "fix: redact" {
		t.Errorf("today's commits = %+v, want the one just made", today)
	}

	if len(yesterday[0].Commits) != 1 || yesterday[0].Commits[0].Subject == "fix: redact" {
		t.Errorf("yesterday's commits = %+v, want yesterday's alone", yesterday)
	}
}

// hunkHeader is a unified diff's hunk header: where each side starts and, when
// it is not one, how many lines it spans.
var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// hunkSpan is how many lines a header's side spans: one when it gives none.
func hunkSpan(t *testing.T, count string) int {
	t.Helper()

	if count == "" {
		return 1
	}

	span, err := strconv.Atoi(count)
	if err != nil {
		t.Fatal(err)
	}

	return span
}

// hunkSides is how many lines a hunk spans on its old side and on its new.
type hunkSides struct {
	old, new int
}

// sidesOf is how many of a hunk's lines are on each side of it: context on
// both, a removal on the old one alone and an addition on the new.
func sidesOf(hunk []string) hunkSides {
	var sides hunkSides

	for _, line := range hunk {
		switch {
		case strings.HasPrefix(line, " "):
			sides.old, sides.new = sides.old+1, sides.new+1
		case strings.HasPrefix(line, "-"):
			sides.old++
		case strings.HasPrefix(line, "+"):
			sides.new++
		}
	}

	return sides
}

func TestEachDemoDiffSpansTheLinesItsHeaderCounts(t *testing.T) {
	t.Parallel()

	deps := standingStill().Deps()

	changes, err := deps.Git.Changes(t.Context())
	if err != nil || len(changes) == 0 {
		t.Fatalf("the demo's changes = %v, %v; want some", changes, err)
	}

	for _, change := range changes {
		t.Run(change.Path, func(t *testing.T) {
			t.Parallel()

			// Act
			diff, diffErr := deps.Git.Diff(t.Context(), change)

			// Assert
			if diffErr != nil || len(diff) == 0 {
				t.Fatalf("the diff of %s = %q, %v; want one", change.Path, diff, diffErr)
			}

			header := hunkHeader.FindStringSubmatch(diff[0])
			if header == nil {
				t.Fatalf("%q is no hunk header", diff[0])
			}

			counted := hunkSides{old: hunkSpan(t, header[1]), new: hunkSpan(t, header[2])}
			if spans := sidesOf(diff[1:]); spans != counted {
				t.Errorf("%q counts %+v lines; the hunk spans %+v", diff[0], counted, spans)
			}
		})
	}
}
