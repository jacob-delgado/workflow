// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// Workspace is where the interface runs: the repository's root, or the directory
// it was started in when there is no repository, and origin's URL.
type Workspace struct {
	Root   string
	Remote string
	// Dir is the directory the session was started in, Root or one within it,
	// with every link resolved.
	Dir string
	// Repository reports Root a repository's root rather than a directory
	// outside one.
	Repository bool
}

// Locate reads the repository dir is in. Outside one, every git action fails on
// its own and says why, so this is not an error.
func Locate(ctx context.Context, dir string) Workspace {
	// git names the root with every link resolved; the directory is read the
	// same way, so the path from one to the other never climbs out.
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	}

	repo, err := gitrepo.At(gitRunner, dir).Describe(ctx)
	if err != nil {
		return Workspace{Root: dir, Remote: "", Dir: dir, Repository: false}
	}

	return Workspace{Root: repo.Root, Remote: repo.Remote, Dir: dir, Repository: true}
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

// repositoriesDeps is what a surface asks of the directories it can work in:
// where this session works, and any other directory read the same way, with
// the configuration files that would apply there.
func repositoriesDeps(ctx context.Context, cfg config.Config, where Workspace) seams.Repositories {
	home, _ := os.UserHomeDir()

	return seams.Repositories{
		Here: placeOf(where, cfg.Layers()),
		Home: home,
		Look: func(dir string) (seams.Place, error) {
			err := workdirs.Check(dir)
			if err != nil {
				return seams.Place{}, err
			}

			// No configuration file there or above is not a failure: the
			// defaults apply, as they would on switching.
			files, _ := config.Locate(dir, home)

			return placeOf(Locate(ctx, dir), files), nil
		},
		Subdirectories: workdirs.List,
	}
}

// placeOf is a workspace as a surface reads it, with the configuration files
// that apply there.
func placeOf(where Workspace, files config.Files) seams.Place {
	root := ""
	if where.Repository {
		root = where.Root
	}

	remote, _ := parsedRemote(where)

	return seams.Place{Dir: where.Dir, Root: root, Remote: remote, Config: files}
}
