// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// pr opens what it is told to in place of what it composes: a draft, into
// another base, asking other reviewers.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// openedWith runs pr in repo with args, every request the fake gh is sent
// written to a log, and returns that log.
func openedWith(t *testing.T, repo string, args ...string) string {
	t.Helper()

	sent := filepath.Join(t.TempDir(), "sent")
	clitest.FakeGh(t, clitest.GhResponses{SentLog: sent})

	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), append([]string{"pr"}, args...)...)
	if err != nil {
		t.Fatalf("pr %v: %v (%+v)", args, err, printed)
	}

	log, err := os.ReadFile(sent)
	if err != nil {
		t.Fatalf("reading what gh was sent: %v", err)
	}

	return string(log)
}

// releaseRepo is clitest.OwnedRepo with a release branch cut from main, on
// which CODEOWNERS gives the API to bea alone.
func releaseRepo(t *testing.T) string {
	t.Helper()

	repo := clitest.OwnedRepo(t)
	clitest.Git(t, repo, "checkout", "-q", "-b", "release", "main")
	clitest.WriteRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @bea\n")
	clitest.Git(t, repo, "add", ".github/CODEOWNERS")
	clitest.Commit(t, repo, "chore: hand the API to bea")
	clitest.Git(t, repo, "checkout", "-q", "fix/PROJ-2-thing")

	return repo
}

func TestPROpensADraftWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := clitest.OwnedRepo(t)

	// Act
	sent := openedWith(t, repo, "--yes", "--draft")

	// Assert
	if !strings.Contains(sent, `"draft":true`) {
		t.Errorf("gh was sent %s, want the pull request opened as a draft", sent)
	}
}

func TestPRPreviewsADraftAsOne(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.OwnedRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--draft")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "  fix/PROJ-2-thing → main (draft)\n") {
		t.Errorf("pr --dry-run --draft = %v, printed %q; want the branches marked a draft", err, printed.Stdout)
	}
}

func TestPRAsksToOpenTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.OwnedRepo(t)

	var asked []string

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "pr", "--draft")

	// Assert
	if err != nil || len(asked) != 1 || !strings.HasPrefix(asked[0], "Open the draft pull request?") {
		t.Errorf("pr --draft = %v, asked %q; want it to ask about the draft", err, asked)
	}
}

func TestPRAsksToPushAndOpenTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.OwnedRepo(t)
	clitest.Commit(t, repo, "fix: not yet pushed")

	var asked []string

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.Answering("n", &asked), "pr", "--draft")

	// Assert
	want := "Push fix/PROJ-2-thing to origin and open the draft pull request?"
	if err != nil || len(asked) != 1 || !strings.HasPrefix(asked[0], want) {
		t.Errorf("pr --draft = %v, asked %q; want %q", err, asked, want)
	}
}

func TestPROpensIntoTheBaseItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := releaseRepo(t)

	// Act
	sent := openedWith(t, repo, "--yes", "--base", "release")

	// Assert
	if !strings.Contains(sent, `"base":"release"`) {
		t.Errorf("gh was sent %s, want the pull request opened into release", sent)
	}
}

func TestPRAsksTheCodeOwnersOfTheBaseItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := releaseRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--base", "release")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "  fix/PROJ-2-thing → release\n") ||
		!strings.Contains(printed.Stdout, "  reviewers bea (code owners)\n") {
		t.Errorf("pr --base release = %v, printed %q; want release's own code owner asked", err, printed.Stdout)
	}
}

// A branch cut from release, opened into release, is drafted from its own
// commit alone, not from release's too, which main never had.
func TestPRDraftsFromTheCommitsPastTheBaseItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := releaseRepo(t)
	clitest.Git(t, repo, "checkout", "-q", "-b", "fix/PROJ-3-release", "release")
	clitest.WriteRepoFile(t, repo, "api/release.go", "package api\n")
	clitest.Git(t, repo, "add", ".")
	clitest.Commit(t, repo, "fix: guard the release")
	clitest.PretendPushed(t, repo)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--base", "release")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "fix: guard the release") ||
		strings.Contains(printed.Stdout, "hand the API to bea") {
		t.Errorf("pr --base release = %v, printed %q; want only the commit past release drafted", err, printed.Stdout)
	}
}

func TestPRSaysWhenTheBaseItIsGivenIsNoBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.OwnedRepo(t)

	// Act
	_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--base", "nonesuch")

	// Assert
	if !strings.Contains(fmt.Sprint(err), "nonesuch") {
		t.Errorf("pr --base nonesuch = %v, want it refused, naming nonesuch", err)
	}

	wantExit(t, err, 1)
}

func TestPRRefusesABaseGitWouldNotName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a leading hyphen": "--base=-x",
		"two dots":         "--base=a..b",
	}

	for name, flag := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := clitest.OwnedRepo(t)

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", flag)

			// Assert
			if !strings.Contains(fmt.Sprint(err), `"--base"`) {
				t.Errorf("pr %s = %v, want it refused, naming --base", flag, err)
			}

			wantExit(t, err, 2)
		})
	}
}

func TestPRAsksTheReviewersItIsGivenInPlaceOfTheCodeOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	// The empty name between the commas is dropped, as a stray comma is.
	repo := clitest.OwnedRepo(t)

	// Act
	sent := openedWith(t, repo, "--yes", "--reviewer", "bea,,acme/docs")

	// Assert
	if !strings.Contains(sent, `"reviewers":["bea"]`) || !strings.Contains(sent, `"team_reviewers":["docs"]`) ||
		strings.Contains(sent, "ana") {
		t.Errorf("gh was sent %s, want bea and the docs team asked in place of the code owners", sent)
	}
}

func TestPRPreviewsTheReviewersItIsGivenAsNoCodeOwners(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{})
	repo := clitest.OwnedRepo(t)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--reviewer", "bea")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "  reviewers bea\n") {
		t.Errorf("pr --reviewer bea = %v, printed %q; want bea previewed, not as a code owner", err, printed.Stdout)
	}
}

// templatedRepo is clitest.OwnedRepo with the pull request templates named,
// each a heading of its own name, as GitHub reads them from
// .github/PULL_REQUEST_TEMPLATE.
func templatedRepo(t *testing.T, names ...string) string {
	t.Helper()

	repo := clitest.OwnedRepo(t)
	for _, name := range names {
		clitest.WriteRepoFile(t, repo, ".github/PULL_REQUEST_TEMPLATE/"+name+".md", "## The "+name+" template\n")
	}

	return repo
}

func TestPRTitlesAndDescribesItAsTold(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := clitest.OwnedRepo(t)

	// Act
	sent := openedWith(t, repo, "--yes", "--title", "fix: guard the api twice", "--body", "Guarded.")

	// Assert
	if !strings.Contains(sent, `"title":"fix: guard the api twice"`) || !strings.Contains(sent, `"body":"Guarded."`) {
		t.Errorf("gh was sent %s, want the title and body given", sent)
	}
}

func TestPRStartsFromTheTemplateItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := templatedRepo(t, "bug", "feature")

	// Act
	sent := openedWith(t, repo, "--yes", "--template", "feature")

	// Assert
	if !strings.Contains(sent, "## The feature template") || strings.Contains(sent, "## The bug template") {
		t.Errorf("gh was sent %s, want the description started from the feature template alone", sent)
	}
}

func TestPRNamesTheTemplatesItHasWhenAskedForOneItLacks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		templates []string
		want      string
	}{
		"a repository with two":  {templates: []string{"bug", "feature"}, want: "it has bug, feature"},
		"a repository with none": {want: "it has none"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.FakeGh(t, clitest.GhResponses{})
			repo := templatedRepo(t, tt.templates...)

			// Act
			_, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--template", "docs")

			// Assert
			if said := fmt.Sprint(err); !strings.Contains(said, `"docs"`) || !strings.Contains(said, tt.want) {
				t.Errorf("pr --template docs = %v, want the name refused and %q said", err, tt.want)
			}

			wantExit(t, err, 1)
		})
	}
}

func TestPRRefusesABodyBesideATemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := templatedRepo(t, "bug")

	// Act
	_, err := clitest.RunStreams(
		t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run", "--body", "Guarded.", "--template", "bug",
	)

	// Assert
	if said := fmt.Sprint(err); !strings.Contains(said, "--body") || !strings.Contains(said, "--template") {
		t.Errorf("pr --body --template = %v, want it refused, naming both flags", err)
	}

	wantExit(t, err, 2)
}
