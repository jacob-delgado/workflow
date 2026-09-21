// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// showCodeOwners is the git command CodeOwners runs for the root CODEOWNERS.
const showCodeOwners = "git -C /work show HEAD:CODEOWNERS"

func TestCodeOwnersListsTheDistinctUserHandles(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file names a team and an email owner alongside people; only the people
	// can be requested as reviewers, and each only once, in first-seen order.
	content := "* @ana @org/platform\n" +
		"internal/forge/ @ben @ana\n" +
		"docs/ docs@example.com\n" +
		"# a comment line\n"
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		showCodeOwners: {out: []byte(content)},
	}), workDir)

	// Act
	owners, err := repo.CodeOwners(t.Context())

	// Assert
	if err != nil || !slices.Equal(owners, []string{"ana", "ben"}) {
		t.Errorf("CodeOwners = %v, %v, want [ana ben]", owners, err)
	}
}

func TestCodeOwnersFallsBackToTheGitHubPath(t *testing.T) {
	t.Parallel()

	// Arrange
	// The root has no CODEOWNERS, so the .github location is read instead.
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		showCodeOwners: {err: errNotIgnored},
		"git -C /work show HEAD:.github/CODEOWNERS": {out: []byte("* @ana\n")},
	}), workDir)

	// Act
	owners, err := repo.CodeOwners(t.Context())

	// Assert
	if err != nil || !slices.Equal(owners, []string{"ana"}) {
		t.Errorf("CodeOwners = %v, %v, want [ana]", owners, err)
	}
}

func TestCodeOwnersIsEmptyWithNoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := reply{err: errNotIgnored}
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		showCodeOwners: missing,
		"git -C /work show HEAD:.github/CODEOWNERS": missing,
		"git -C /work show HEAD:docs/CODEOWNERS":    missing,
	}), workDir)

	// Act
	owners, err := repo.CodeOwners(t.Context())

	// Assert
	if err != nil || len(owners) != 0 {
		t.Errorf("CodeOwners = %v, %v, want no owners", owners, err)
	}
}
