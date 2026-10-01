// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/codeowners"
)

// CodeOwnersAt reads the CODEOWNERS file as it stands on base — the branch a
// pull request merges into, whose rules decide who reviews it — from the first
// place dialect looks, and parses it as dialect does. base is read as
// ChangedPaths reads it, so the owners and the paths they own agree. No file
// is not an error: it reports false, and the zero File owns nothing.
func (r Repository) CodeOwnersAt(
	ctx context.Context, base string, dialect codeowners.Dialect,
) (codeowners.File, bool, error) {
	ref, err := r.baseRef(ctx, base)
	if err != nil {
		return codeowners.File{}, false, err
	}

	content, found, err := r.FileAt(ctx, ref, codeowners.Locations(dialect))
	if err != nil || !found {
		return codeowners.File{}, false, err
	}

	return codeowners.Parse(string(content), dialect), true, nil
}
