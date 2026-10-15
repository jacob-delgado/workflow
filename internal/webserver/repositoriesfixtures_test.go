// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"slices"
	"sync"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// The directories the cases work in, under anaHome.
const (
	anaHome   = "/home/ana"
	apiRoot   = "/home/ana/src/api"
	apiCmd    = "/home/ana/src/api/cmd"
	webRoot   = "/home/ana/src/web"
	notesDir  = "/home/ana/notes"
	oldDir    = "/home/ana/old"
	reposPath = "/api/repositories"
)

// apiPlace is the api repository, worked in from its cmd directory.
func apiPlace() seams.Place {
	return seams.Place{
		Dir: apiCmd, Root: apiRoot,
		Remote: forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "acme/api"},
		Config: config.Files{Repo: apiRoot + "/.workflow.json", Home: anaHome + "/.workflow.json"},
	}
}

// placesByDir are the directories there are, as Look reads them.
func placesByDir() map[string]seams.Place {
	return map[string]seams.Place{
		apiCmd:   apiPlace(),
		webRoot:  {Dir: webRoot, Root: webRoot, Remote: forge.Repo{Host: "github.com", Path: "acme/web"}},
		notesDir: {Dir: notesDir},
	}
}

// keptFavorites is the favorites a case's store keeps, and what it was asked.
type keptFavorites struct {
	mu    sync.Mutex
	dirs  []string
	asked []string
	// hold, when set, keeps a Favor from answering until it is closed, and
	// entered is closed once one has started.
	hold, entered chan struct{}
}

// repositoryDeps work in api's cmd directory, with web, notes and a directory
// since removed as favorites.
func repositoryDeps(kept *keptFavorites) webserver.Deps {
	deps := webtest.FilledDeps()
	places := placesByDir()
	deps.Repositories = seams.Repositories{
		Here: apiPlace(), Home: anaHome,
		Look: func(_ context.Context, dir string) (seams.Place, error) {
			place, found := places[dir]
			if !found {
				return seams.Place{}, workdirs.ErrNotFound
			}

			return place, nil
		},
		Subdirectories: func(dir, _ string) (workdirs.Listing, error) {
			if dir != "/home/ana/src" {
				return workdirs.Listing{}, workdirs.ErrNotFound
			}

			return workdirs.Listing{Entries: []workdirs.Entry{{Name: "api", Repository: true}, {Name: "notes"}}}, nil
		},
	}
	deps.Store.Favorites = func(context.Context) ([]string, error) {
		kept.mu.Lock()
		defer kept.mu.Unlock()

		return slices.Clone(kept.dirs), nil
	}
	deps.Store.Favor = func(_ context.Context, dir string) error {
		if kept.hold != nil {
			close(kept.entered)
			<-kept.hold
		}

		kept.mu.Lock()
		defer kept.mu.Unlock()

		kept.asked = append(kept.asked, "favor "+dir)
		kept.dirs = append(kept.dirs, dir)

		return nil
	}
	deps.Store.Unfavor = func(_ context.Context, dir string) error {
		kept.mu.Lock()
		defer kept.mu.Unlock()

		kept.asked = append(kept.asked, "unfavor "+dir)

		return nil
	}

	return deps
}

// favoritesKept is a store keeping web, notes and a removed directory.
func favoritesKept() *keptFavorites {
	return &keptFavorites{dirs: []string{webRoot, notesDir, oldDir, apiCmd}}
}

// reachable lets deps switch to any directory Look reads, and records each
// switch asked.
func reachable(deps webserver.Deps, reached *[]string) webserver.Deps {
	deps.Reach = func(ctx context.Context, dir string) (webserver.World, error) {
		*reached = append(*reached, dir)

		place, err := deps.Repositories.Look(ctx, dir)
		if err != nil {
			return webserver.World{}, err
		}

		next := deps
		next.Repositories.Here = place

		return webserver.World{Deps: next, Config: config.Default(), Info: webserver.Info{Version: webtest.TestVersion}}, nil
	}

	return deps
}
