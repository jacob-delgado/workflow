// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// summaryDay is the one-day period every case reads.
func summaryDay() (time.Time, time.Time) {
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	return start, start.Add(24 * time.Hour)
}

func TestTheTasksYouTouchedAreReadThroughTheTaskSeam(t *testing.T) {
	// Arrange
	record := recording(t)
	tasks := wiredTasks(t, config.Default(), pathWithGitAnd(t, "task", fakeTaskwarrior("3.5.0")))
	start, _ := summaryDay()

	// Act
	touched, err := tasks.Touched(start)

	// Assert
	if err != nil || len(touched) != 1 {
		t.Fatalf("Touched = %+v, %v; want the one task", touched, err)
	}

	calls, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("reading what the task program was asked: %v", err)
	}

	if !askedWith(string(calls), "status.not:deleted export", "modified.after:2026-10-01T23:59:59Z") {
		t.Errorf("the task program was asked:\n%s\nwant the tasks changed since the start", calls)
	}
}

func TestOnlyJiraAnswersForTheIssuesYouTouched(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})

	// Act
	both, forgeAlone := bothTrackers(t, &fakeJira{}), forgeTracker(t)

	// Assert
	// A forge issue's own activity is read from the forge's, which the
	// Forge seam answers, so the tracker of forge issues alone has none.
	if both.Activity == nil || forgeAlone.Activity != nil {
		t.Errorf("Activity set with Jira = %v, with the forge alone = %v; want true and false",
			both.Activity != nil, forgeAlone.Activity != nil)
	}
}

func TestWhatYouDidOnTheForgeIsReadThroughItsCLI(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{search: `{"total_count":0,"items":[]}`})
	cfg, where := githubCLIWorkspace(t)
	forgeSeams := wired(t, cfg, where, nil).Forge

	// Act
	_, err := forgeSeams.Activity(summaryDay())

	// Assert
	if err != nil || !slices.ContainsFunc(ghStub.args(), func(arg string) bool {
		return strings.Contains(arg, "/search/issues")
	}) {
		t.Errorf("Activity = %v, gh called as %v; want the forge searched", err, ghStub.args())
	}
}

func TestYourCommitsAreReadFromTheRepositoryWorkflowIsIn(t *testing.T) {
	// Arrange
	// Outside a repository there is nothing to read, and the seam says so
	// rather than answering an empty period.
	gitSeams := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir()}, nil).Git

	// Act
	_, err := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if err == nil {
		t.Error("CommitsBetween outside a repository = nil, want why it could not be read")
	}
}

// committedOn is a repository whose one commit of yours was written on the
// day summaryDay reads, under email.
func committedOn(t *testing.T, email string) string {
	t.Helper()
	isolateGit(t)

	start, _ := summaryDay()
	when := start.Add(9 * time.Hour).Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", when)
	t.Setenv("GIT_COMMITTER_DATE", when)

	root := repository(t)
	git(t, root, "config", "user.email", email)
	write(t, filepath.Join(root, "notes.md"), "mine\n", 0o600)
	git(t, root, "add", "notes.md")
	git(t, root, "commit", "--quiet", "-m", "feat: mine")

	return root
}

func TestYourCommitsAreFoundUnderAnEmailWithAPlus(t *testing.T) {
	// Arrange
	// git reads --author as a basic regular expression, where an escaped + is
	// an operator rather than the + a plus address is written with.
	root := committedOn(t, "me+work@example.com")
	gitSeams := wired(t, config.Default(), wiring.Workspace{Root: root}, nil).Git

	// Act
	commits, err := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if err != nil || len(commits) != 1 || commits[0].Subject != "feat: mine" {
		t.Errorf("CommitsBetween = %+v, %v; want the one commit you wrote", commits, err)
	}
}

func TestAStashIsNotOneOfYourCommits(t *testing.T) {
	// Arrange
	root := committedOn(t, "me@example.com")
	write(t, filepath.Join(root, "notes.md"), "changed\n", 0o600)
	git(t, root, "stash", "--quiet")

	gitSeams := wired(t, config.Default(), wiring.Workspace{Root: root}, nil).Git

	// Act
	commits, err := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if err != nil || len(commits) != 1 || commits[0].Subject != "feat: mine" {
		t.Errorf("CommitsBetween = %+v, %v; want only the commit, never the stash's", commits, err)
	}
}
