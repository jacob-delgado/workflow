// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// withoutGit answers every command the way proc does when git is not on PATH.
func withoutGit(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, fmt.Errorf("%w: git", proc.ErrNotFound)
}

func TestAMissingGitIsReportedAsSuch(t *testing.T) {
	t.Parallel()

	cases := map[string]func(context.Context, gitrepo.Repository) error{
		"describing the repository": func(ctx context.Context, repo gitrepo.Repository) error {
			_, err := repo.Describe(ctx)

			return err
		},
		"a read that asks where the work tree is": func(ctx context.Context, repo gitrepo.Repository) error {
			_, err := repo.Status(ctx)

			return err
		},
		"an ignore check": func(ctx context.Context, repo gitrepo.Repository) error {
			_, err := repo.CheckIgnored(ctx, ".workflow.json")

			return err
		},
	}

	for name, read := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := read(t.Context(), gitrepo.At(withoutGit, workDir))

			// Assert
			if !errors.Is(err, proc.ErrNotFound) || errors.Is(err, gitrepo.ErrNotARepository) {
				t.Errorf("got %v; want the missing program, not ErrNotARepository", err)
			}
		})
	}
}
