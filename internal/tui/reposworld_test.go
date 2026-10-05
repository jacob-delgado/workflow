// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// The directories the Repositories cases work in, under anaHome.
const (
	anaHome = "/home/ana"
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
	// keepsNothing binds no favorites, as a store turned off does.
	keepsNothing bool
}

// apiPlace is the api repository, worked in from its cmd directory.
func apiPlace() seams.Place {
	return seams.Place{
		Dir: apiCmd, Root: apiRoot,
		Remote: forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "acme/api"},
		Config: config.Files{Repo: apiRoot + "/.workflow.json", Home: anaHome + "/.workflow.json"},
	}
}

// reposWorld is a world working in api's cmd directory, with web and a
// directory since removed as favorites.
func reposWorld() *world {
	working := newWorld()
	working.dirs = &dirsWorld{
		here: apiPlace(),
		places: map[string]seams.Place{
			apiCmd:  apiPlace(),
			webRoot: {Dir: webRoot, Root: webRoot, Remote: forge.Repo{Host: "github.com", Path: "acme/web"}},
		},
		favorites: []string{webRoot, oldDir},
	}

	return working
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
		Look: func(dir string) (seams.Place, error) {
			place, found := dirs.places[dir]
			if !found {
				return seams.Place{}, workdirs.ErrNotFound
			}

			return place, nil
		},
		Subdirectories: nil,
	}

	if dirs.keepsNothing {
		return deps
	}

	deps.Store.Favorites = func() ([]string, error) {
		w.record("favorites")

		w.mu.Lock()
		defer w.mu.Unlock()

		return append([]string(nil), dirs.favorites...), nil
	}
	deps.Store.Favor = func(dir string) error {
		w.record("favor " + dir)

		w.mu.Lock()
		defer w.mu.Unlock()

		dirs.favorites = append(dirs.favorites, dir)

		return nil
	}
	deps.Store.Unfavor = func(dir string) error {
		w.record("unfavor " + dir)

		return nil
	}

	return deps
}
