// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// Workspace is where the interface runs: the repository's root, or the directory
// it was started in when there is no repository, and origin's URL.
type Workspace struct {
	Root   string
	Remote string
}

// Locate reads the repository dir is in. Outside one, every git action fails on
// its own and says why, so this is not an error.
func Locate(ctx context.Context, dir string) Workspace {
	repo, err := gitrepo.At(gitRunner, dir).Describe(ctx)
	if err != nil {
		return Workspace{Root: dir, Remote: ""}
	}

	return Workspace{Root: repo.Root, Remote: repo.Remote}
}

// repoKey names the repository the store keys its state by: the origin remote's
// host and path where there is one, so the same repository shares it across
// clones, and the working tree's root otherwise. The remote is parsed to its
// host and path, never used raw, because an HTTPS remote can carry a credential
// in its userinfo and the store must never hold a secret.
func repoKey(where Workspace) string {
	repo, parsed := parsedRemote(where)
	if !parsed {
		return where.Root
	}

	return repo.Host + "/" + repo.Path
}

// parsedRemote is the origin remote parsed to its host and path, and whether
// there is one that parses.
func parsedRemote(where Workspace) (forge.Repo, bool) {
	if where.Remote == "" {
		return forge.Repo{}, false
	}

	repo, err := forge.ParseRemote(where.Remote)
	if err != nil {
		return forge.Repo{}, false
	}

	return repo, true
}
