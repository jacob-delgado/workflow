// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// The directories the Repositories cases work in, under anaHome.
const (
	anaHome = "/home/ana"
	srcDir  = "/home/ana/src"
	apiRoot = "/home/ana/src/api"
	apiCmd  = "/home/ana/src/api/cmd"
	webRoot = "/home/ana/src/web"
	oldDir  = "/home/ana/old"
)

// dirsWorld is where the session works, the directories there are, and the
// favorites the store keeps.
type dirsWorld struct {
	here      seams.Place
	places    map[string]seams.Place
	favorites []string
	// listings are the directories in each directory, by its path; those
	// named in repositories are repositories.
	listings     map[string][]string
	repositories []string
	// keepsNothing binds no favorites, as a store turned off does.
	keepsNothing bool
	// failing is why reading and changing the favorites fails, or nil.
	failing error
	// cannotLook binds no Look, as a surface given no way to read a
	// directory has.
	cannotLook bool
	// worktrees is the repository's working trees, the main one first, or
	// why they could not be read; neither binds no Worktrees, as outside a
	// repository.
	worktrees    []gitrepo.Worktree
	worktreesErr error
}

// apiPlace is the api repository, worked in from its cmd directory.
func apiPlace() seams.Place {
	return seams.Place{
		Dir: apiCmd, Root: apiRoot,
		Remote: forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "acme/api"},
		Config: config.Files{Repo: apiRoot + "/.workflow.json", Home: anaHome + "/.workflow.json"},
	}
}

// webPlace is the web repository, worked in from its root.
func webPlace() seams.Place {
	return seams.Place{Dir: webRoot, Root: webRoot, Remote: forge.Repo{Host: "github.com", Path: "acme/web"}}
}

// reposWorld is a world working in api's cmd directory, with web and a
// directory since removed as favorites.
func reposWorld() *world {
	working := newWorld()
	working.dirs = &dirsWorld{
		here: apiPlace(),
		places: map[string]seams.Place{
			apiCmd:  apiPlace(),
			webRoot: webPlace(),
		},
		favorites: []string{webRoot, oldDir},
	}

	return working
}

// listed is the directories in dir whose names start with prefix in any
// case, a hidden one only once prefix starts with its dot, the first
// workdirs.ListLimit of them kept, as workdirs.List gives them.
func (dirs *dirsWorld) listed(dir, prefix string) (workdirs.Listing, error) {
	names, found := dirs.listings[dir]
	if !found {
		return workdirs.Listing{}, workdirs.ErrNotFound
	}

	listing := workdirs.Listing{}

	for _, name := range names {
		if offered(name, prefix) {
			entry := workdirs.Entry{Name: name, Repository: slices.Contains(dirs.repositories, name)}
			listing.Entries = append(listing.Entries, entry)
		}
	}

	if len(listing.Entries) > workdirs.ListLimit {
		listing.Entries, listing.Truncated = listing.Entries[:workdirs.ListLimit], true
	}

	return listing, nil
}

// offered reports a name a listing for prefix holds, as workdirs.List
// decides: one starting with it in any case, and not hidden unless prefix
// starts with the dot.
func offered(name, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) &&
		(!strings.HasPrefix(name, ".") || strings.HasPrefix(prefix, "."))
}

// withRepositories wires the Repositories seams and the favorites into deps
// when the world has places, recording each favorite marked or forgotten.
func (w *world) withRepositories(deps tui.Deps) tui.Deps {
	if w.dirs == nil {
		return deps
	}

	dirs := w.dirs
	deps.Repositories = seams.Repositories{
		Here: dirs.here, Home: anaHome,
		Look: func(_ context.Context, dir string) (seams.Place, error) {
			place, found := dirs.places[dir]
			if !found {
				return seams.Place{}, workdirs.ErrNotFound
			}

			return place, nil
		},
		Subdirectories: func(dir, prefix string) (workdirs.Listing, error) {
			w.record("subdirectories " + dir + " " + prefix)

			return dirs.listed(dir, prefix)
		},
	}

	if dirs.cannotLook {
		deps.Repositories.Look = nil
	}

	if dirs.worktrees != nil || dirs.worktreesErr != nil {
		deps.Repositories.Worktrees = func(context.Context) ([]gitrepo.Worktree, error) {
			return dirs.worktrees, dirs.worktreesErr
		}
	}

	if dirs.keepsNothing {
		return deps
	}

	deps.Store.Favorites = func(context.Context) ([]string, error) {
		w.record("favorites")

		w.mu.Lock()
		defer w.mu.Unlock()

		return append([]string(nil), dirs.favorites...), dirs.failing
	}
	deps.Store.Favor = func(_ context.Context, dir string) error {
		w.record("favor " + dir)

		w.mu.Lock()
		defer w.mu.Unlock()

		dirs.favorites = append(dirs.favorites, dir)

		return dirs.failing
	}
	deps.Store.Unfavor = func(_ context.Context, dir string) error {
		w.record("unfavor " + dir)

		return nil
	}

	return deps
}
