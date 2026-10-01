// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The commands the branch's issue link is kept with, for a branch named
// my-thing, in git's own configuration of the repository.
const (
	readLink  = "git -C /work config --get branch.my-thing.workflow-issue"
	writeLink = "git -C /work config branch.my-thing.workflow-issue PROJ-7"
	clearLink = "git -C /work config --unset branch.my-thing.workflow-issue"
	listLinks = `git -C /work config --get-regexp ^branch\..*\.workflow-issue$`
)

// errUnset is git config's answer for a key it does not hold.
var errUnset = errors.New("exit status 1")

func TestIssueLinkReadsTheBranchesLink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{readLink: {out: []byte("PROJ-7\n")}}), workDir)

	// Act & Assert
	if got := repo.IssueLink(t.Context(), "my-thing"); got != "PROJ-7" {
		t.Errorf("IssueLink = %q, want PROJ-7", got)
	}
}

func TestIssueLinkIsEmptyForABranchWithNone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{readLink: {err: errUnset}}), workDir)

	// Act & Assert
	if got := repo.IssueLink(t.Context(), "my-thing"); got != "" {
		t.Errorf("IssueLink = %q, want none", got)
	}
}

func TestSetIssueLinkKeepsItInGitsConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{writeLink: {}}), workDir)

	// Act
	err := repo.SetIssueLink(t.Context(), "my-thing", "PROJ-7")
	// Assert
	if err != nil {
		t.Errorf("SetIssueLink = %v", err)
	}
}

func TestSetIssueLinkThatGitCannotWriteSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{writeLink: {err: errUnset}}), workDir)

	// Act
	err := repo.SetIssueLink(t.Context(), "my-thing", "PROJ-7")

	// Assert
	if !errors.Is(err, gitrepo.ErrIssueLinkNotSaved) {
		t.Errorf("SetIssueLink = %v, want ErrIssueLinkNotSaved", err)
	}
}

func TestClearIssueLinkThatGitCannotWriteSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		readLink: {out: []byte("PROJ-7\n")}, clearLink: {err: errUnset},
	}), workDir)

	// Act
	err := repo.ClearIssueLink(t.Context(), "my-thing")

	// Assert
	if !errors.Is(err, gitrepo.ErrIssueLinkNotSaved) {
		t.Errorf("ClearIssueLink = %v, want ErrIssueLinkNotSaved", err)
	}
}

func TestClearIssueLinkForgetsALinkThatIsThere(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		readLink: {out: []byte("PROJ-7\n")}, clearLink: {},
	}), workDir)

	// Act
	err := repo.ClearIssueLink(t.Context(), "my-thing")
	// Assert
	if err != nil {
		t.Errorf("ClearIssueLink = %v", err)
	}
}

func TestClearIssueLinkForgetsALinkTooOddToShow(t *testing.T) {
	t.Parallel()

	// Arrange
	run, ran := recordingRunner(t, map[string]reply{
		readLink: {out: []byte("\x1b]8;;http://x\aPROJ-7\n")}, clearLink: {},
	})
	repo := gitrepo.At(run, workDir)

	// Act
	err := repo.ClearIssueLink(t.Context(), "my-thing")

	// Assert
	if err != nil || !slices.Contains(*ran, clearLink) {
		t.Errorf("ClearIssueLink = %v, ran %q; want the stored value unset", err, *ran)
	}
}

func TestClearIssueLinkWithNoneDoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only the read is answered: an unset of a key git does not hold would fail.
	repo := gitrepo.At(fakeRunner(t, map[string]reply{readLink: {err: errUnset}}), workDir)

	// Act
	err := repo.ClearIssueLink(t.Context(), "my-thing")
	// Assert
	if err != nil {
		t.Errorf("ClearIssueLink = %v, want nothing to clear", err)
	}
}

func TestIssueLinksListsEveryBranchesLink(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{listLinks: {
		out: []byte("branch.my-thing.workflow-issue PROJ-7\nbranch.fix/v1.2-typo.workflow-issue 42\n"),
	}}), workDir)

	// Act
	links := repo.IssueLinks(t.Context())

	// Assert
	want := map[string]string{"my-thing": "PROJ-7", "fix/v1.2-typo": "42"}
	if !maps.Equal(links, want) {
		t.Errorf("IssueLinks = %v, want %v", links, want)
	}
}

func TestIssueLinksWithNoneIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{listLinks: {err: errUnset}}), workDir)

	// Act & Assert
	if links := repo.IssueLinks(t.Context()); len(links) != 0 {
		t.Errorf("IssueLinks = %v, want none", links)
	}
}
