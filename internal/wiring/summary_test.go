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
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui"
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
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if len(read) != 1 || read[0].Failed == nil {
		t.Errorf("CommitsBetween outside a repository = %+v, want why it could not be read", read)
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
	gitSeams := wired(t, config.Default(), wiring.Locate(t.Context(), root), nil).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if commits := onlyRepository(read); len(commits) != 1 || commits[0].Subject != "feat: mine" {
		t.Errorf("CommitsBetween = %+v; want the one commit you wrote", read)
	}
}

func TestAStashIsNotOneOfYourCommits(t *testing.T) {
	// Arrange
	root := committedOn(t, "me@example.com")
	write(t, filepath.Join(root, "notes.md"), "changed\n", 0o600)
	git(t, root, "stash", "--quiet")

	gitSeams := wired(t, config.Default(), wiring.Locate(t.Context(), root), nil).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if commits := onlyRepository(read); len(commits) != 1 || commits[0].Subject != "feat: mine" {
		t.Errorf("CommitsBetween = %+v; want only the commit, never the stash's", read)
	}
}

// onlyRepository is the commits of a read that found one repository, read in
// full and unnamed, or nil when it found otherwise.
func onlyRepository(read []loop.RepositoryCommits) []gitrepo.DatedCommit {
	if len(read) != 1 || read[0].Failed != nil || read[0].Repository != "" {
		return nil
	}

	return read[0].Commits
}

// favoring wires the repository here is in, with each of favorites kept as
// a favorite.
func favoring(t *testing.T, here string, favorites ...string) tui.Deps {
	t.Helper()
	homeOfItsOwn(t)

	deps := wired(t, config.Default(), wiring.Locate(t.Context(), here), nil)
	for _, favorite := range favorites {
		err := deps.Store.Favor(favorite)
		if err != nil {
			t.Fatalf("Favor(%s): %v", favorite, err)
		}
	}

	return deps
}

func TestYourCommitsAreReadFromEveryFavoriteThatIsARepository(t *testing.T) {
	// Arrange
	here := committedOn(t, "me@example.com")
	web := committedOn(t, "me@example.com")
	git(t, web, "remote", "add", "origin", "git@github.com:acme/web.git")
	gitSeams := favoring(t, here, web, t.TempDir()).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	names := make([]string, 0, len(read))
	for _, repository := range read {
		if repository.Failed != nil || len(repository.Commits) != 1 {
			t.Errorf("%s read as %+v, want its one commit", repository.Repository, repository)
		}

		names = append(names, repository.Repository)
	}

	if want := []string{filepath.Base(here), "acme/web"}; !slices.Equal(names, want) {
		t.Errorf("repositories read = %q, want %q: here, then the favorite, by its origin", names, want)
	}
}

func TestAFavoriteWithinTheRepositoryYouAreInIsNotReadTwice(t *testing.T) {
	// Arrange
	here := committedOn(t, "me@example.com")
	within := filepath.Join(here, "docs")

	err := os.Mkdir(within, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	gitSeams := favoring(t, here, within).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if commits := onlyRepository(read); len(commits) != 1 {
		t.Errorf("CommitsBetween = %+v, want the one repository, read once and unnamed", read)
	}
}

func TestOutsideARepositoryYourFavoritesAreStillRead(t *testing.T) {
	// Arrange
	favorite := committedOn(t, "me@example.com")
	homeOfItsOwn(t)

	deps := wired(t, config.Default(), wiring.Locate(t.Context(), t.TempDir()), nil)

	err := deps.Store.Favor(favorite)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	read := deps.Git.CommitsBetween(summaryDay())

	// Assert
	if commits := onlyRepository(read); len(commits) != 1 {
		t.Errorf("CommitsBetween = %+v, want the favorite's commit, never a failure for where you are", read)
	}
}

func TestTheNameOfARepositoryReachesTheSummaryNeutralized(t *testing.T) {
	// Arrange
	// A directory's name is anyone's to write, a terminal control included.
	here := committedOn(t, "me@example.com")
	hostile := filepath.Join(t.TempDir(), "api\x1b[2J")

	err := os.Rename(committedOn(t, "me@example.com"), hostile)
	if err != nil {
		t.Fatal(err)
	}

	gitSeams := favoring(t, here, hostile).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if len(read) != 2 || strings.ContainsRune(read[1].Repository, '\x1b') {
		t.Errorf("CommitsBetween = %+v, want the favorite named without its escape", read)
	}
}

func TestAWorktreeOfTheRepositoryYouAreInIsNotReadTwice(t *testing.T) {
	// Arrange
	// Each worktree has a root of its own, but they share one history.
	here := committedOn(t, "me@example.com")
	worktree := filepath.Join(t.TempDir(), "hotfix")
	git(t, here, "worktree", "add", "--quiet", worktree)
	gitSeams := favoring(t, here, worktree).Git

	// Act
	read := gitSeams.CommitsBetween(summaryDay())

	// Assert
	if commits := onlyRepository(read); len(commits) != 1 {
		t.Errorf("CommitsBetween = %+v, want the one repository, read once and unnamed", read)
	}
}

func TestADryRunsSummaryMakesNoStore(t *testing.T) {
	// Arrange
	home := homeOfItsOwn(t)
	gitSeams := wired(t, config.Default(), wiring.Locate(t.Context(), committedOn(t, "me@example.com")), nil).Git

	// Act
	gitSeams.CommitsBetween(summaryDay())

	// Assert
	var made []string

	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, _ error) error {
		if entry != nil && strings.HasSuffix(entry.Name(), ".db") {
			made = append(made, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(made) > 0 {
		t.Errorf("reading the Summary made %v, want favorites read only from a store already there", made)
	}
}
