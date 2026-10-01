// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestCodeOwnersAtReadsTheDialectsFileAtOriginsBase(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab looks at the root first, then docs/, then .gitlab/.
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginMain: {out: []byte("0123abcd\n")},
		verifyOriginRef:  {out: []byte("0123abcd\n")},
		"git -C /work show origin/main:CODEOWNERS":         {err: errNotIgnored},
		"git -C /work show origin/main:docs/CODEOWNERS":    {err: errNotIgnored},
		"git -C /work show origin/main:.gitlab/CODEOWNERS": {out: []byte("[Go] @ana\n*.go @org/team\n")},
	}), workDir)

	// Act
	file, found, err := repo.CodeOwnersAt(t.Context(), "main", codeowners.GitLab)

	// Assert
	owners := file.OwnersOf([]string{goFile})
	if err != nil || !found || !slices.Equal(owners.Teams, []string{"org/team"}) {
		t.Errorf("CodeOwnersAt = %+v, %v, %v, want org/team owning a.go", owners, found, err)
	}
}

func TestCodeOwnersAtFindsNoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := reply{err: errNotIgnored}
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		verifyOriginMain: missing,
		"git -C /work rev-parse --verify --quiet main^{commit}": {out: []byte("0123abcd\n")},
		"git -C /work show main:.github/CODEOWNERS":             missing,
		"git -C /work show main:CODEOWNERS":                     missing,
		"git -C /work show main:docs/CODEOWNERS":                missing,
	}), workDir)

	// Act
	file, found, err := repo.CodeOwnersAt(t.Context(), "main", codeowners.GitHub)

	// Assert
	if err != nil || found || len(file.OwnersOf([]string{goFile}).Users) != 0 {
		t.Errorf("CodeOwnersAt = %v, %v, want no file", found, err)
	}
}

func TestCodeOwnersAtRefusesABaseThatReadsAsAnOption(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{}), workDir)

	// Act
	_, _, err := repo.CodeOwnersAt(t.Context(), "-p", codeowners.GitHub)

	// Assert
	if !errors.Is(err, gitrepo.ErrOptionLikeRef) {
		t.Errorf("CodeOwnersAt = %v, want ErrOptionLikeRef", err)
	}
}
