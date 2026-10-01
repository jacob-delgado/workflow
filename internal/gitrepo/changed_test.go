// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// Commands the changed-path and file reads run against a base of main.
const (
	verifyOriginMain = "git -C /work rev-parse --verify --quiet refs/remotes/origin/main"
	diffOriginMain   = "git -C /work diff --name-only -z --no-renames origin/main...HEAD"
	verifyOriginRef  = "git -C /work rev-parse --verify --quiet origin/main^{commit}"
)

// goFile and markdownFile are files at the repository root.
const (
	goFile       = "a.go"
	markdownFile = "a.md"
)

// errNoSuchRef is git refusing a ref it does not have.
var errNoSuchRef = errors.New("fatal: bad revision")

func TestChangedPathsListsWhatTheBranchChangesAgainstOriginsBase(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path holding an escape sequence is no file anyone means to own, so it
	// is left out.
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginMain: {out: []byte("0123abcd\n")},
		diffOriginMain:   {out: []byte("internal/a.go\x00docs/my file.md\x00evil\x1b[2J.go\x00")},
	}), workDir)

	// Act
	paths, err := repo.ChangedPaths(t.Context(), "main")

	// Assert
	if err != nil || !slices.Equal(paths, []string{"internal/a.go", "docs/my file.md"}) {
		t.Errorf("ChangedPaths = %q, %v, want the two showable paths", paths, err)
	}
}

func TestChangedPathsKeepsAPathThatDrawsDifferentlyButHoldsNoControl(t *testing.T) {
	t.Parallel()

	// Arrange
	// A zero-width joiner spells an emoji and a tab is a legal filename
	// character; neither is a terminal control, and the paths are only
	// matched, never shown.
	joined := "docs/\U0001F469\u200d\U0001F4BB.md"
	tabbed := "src/a\tb.go"
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginMain: {out: []byte("0123abcd\n")},
		diffOriginMain:   {out: []byte(joined + "\x00" + tabbed + "\x00")},
	}), workDir)

	// Act
	paths, err := repo.ChangedPaths(t.Context(), "main")

	// Assert
	if err != nil || !slices.Equal(paths, []string{joined, tabbed}) {
		t.Errorf("ChangedPaths = %q, %v, want both paths kept", paths, err)
	}
}

func TestChangedPathsUsesTheLocalBaseWhenOriginHasNone(t *testing.T) {
	t.Parallel()

	// Arrange
	run, ran := recordingRunner(t, map[string]reply{
		verifyOriginMain: {err: errNotIgnored},
		"git -C /work diff --name-only -z --no-renames main...HEAD": {out: []byte("a.go\x00")},
	})

	// Act
	paths, err := gitrepo.At(run, workDir).ChangedPaths(t.Context(), "main")

	// Assert
	if err != nil || !slices.Equal(paths, []string{goFile}) || len(*ran) != 2 {
		t.Errorf("ChangedPaths = %q, %v after %q, want a.go against the local main", paths, err, *ran)
	}
}

func TestChangedPathsRefusesABaseThatReadsAsAnOption(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{}), workDir)

	// Act
	_, err := repo.ChangedPaths(t.Context(), "--output=/tmp/x")

	// Assert
	if !errors.Is(err, gitrepo.ErrOptionLikeRef) {
		t.Errorf("ChangedPaths = %v, want ErrOptionLikeRef", err)
	}
}

func TestChangedPathsSaysWhyTheDiffFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginMain: {out: []byte("0123abcd\n")},
		diffOriginMain:   {err: errNoSuchRef},
		showToplevel:     {out: []byte("/work\n")},
	}), workDir)

	// Act
	_, err := repo.ChangedPaths(t.Context(), "main")

	// Assert
	if !errors.Is(err, errNoSuchRef) {
		t.Errorf("ChangedPaths = %v, want git's own error", err)
	}
}

func TestAChangedPathsThatTimedOutSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run, _ := hungRunner()

	// Act
	_, err := gitrepo.At(run, workDir).ChangedPaths(t.Context(), "main")

	// Assert
	if !timedOutNotMissing(err) {
		t.Errorf("ChangedPaths = %v, want the timeout", err)
	}
}

func TestFileAtReadsTheFirstPathPresentAtTheRef(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginRef:                      {out: []byte("0123abcd\n")},
		"git -C /work show origin/main:a.md": {err: errNotIgnored},
		"git -C /work show origin/main:b.md": {out: []byte("b\n")},
	}), workDir)

	// Act
	content, found, err := repo.FileAt(t.Context(), "origin/main", []string{markdownFile, "b.md", "c.md"})

	// Assert
	if err != nil || !found || string(content) != "b\n" {
		t.Errorf("FileAt = %q, %v, %v, want b.md's content", content, found, err)
	}
}

func TestFileAtReadsHeadWhenTheRefIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginRef:               {err: errNoSuchRef},
		"git -C /work show HEAD:a.md": {out: []byte("a\n")},
	}), workDir)

	// Act
	content, found, err := repo.FileAt(t.Context(), "origin/main", []string{markdownFile})

	// Assert
	if err != nil || !found || string(content) != "a\n" {
		t.Errorf("FileAt = %q, %v, %v, want HEAD's a.md", content, found, err)
	}
}

func TestFileAtFindsNothingWhenNoPathIsPresent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginRef:                      {out: []byte("0123abcd\n")},
		"git -C /work show origin/main:a.md": {err: errNotIgnored},
	}), workDir)

	// Act
	content, found, err := repo.FileAt(t.Context(), "origin/main", []string{markdownFile})

	// Assert
	if err != nil || found || content != nil {
		t.Errorf("FileAt = %q, %v, %v, want nothing found", content, found, err)
	}
}

func TestFileAtRefusesARefThatReadsAsAnOption(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{}), workDir)

	// Act
	_, _, err := repo.FileAt(t.Context(), "-p", []string{markdownFile})

	// Assert
	if !errors.Is(err, gitrepo.ErrOptionLikeRef) {
		t.Errorf("FileAt = %v, want ErrOptionLikeRef", err)
	}
}

func TestAFileAtThatTimedOutSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run, _ := hungRunner()

	// Act
	_, _, err := gitrepo.At(run, workDir).FileAt(t.Context(), "origin/main", []string{markdownFile})

	// Assert
	if !timedOutNotMissing(err) {
		t.Errorf("FileAt = %v, want the timeout", err)
	}
}

func TestAFileAtWhoseShowTimedOutSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	hung, _ := hungRunner()
	verified := fakeRunner(t, map[string]reply{verifyOriginRef: {out: []byte("0123abcd\n")}})
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[2] == "show" {
			return hung(ctx, name, args...)
		}

		return verified(ctx, name, args...)
	}

	// Act
	_, _, err := gitrepo.At(run, workDir).FileAt(t.Context(), "origin/main", []string{markdownFile})

	// Assert
	if !timedOutNotMissing(err) {
		t.Errorf("FileAt = %v, want the timeout", err)
	}
}
