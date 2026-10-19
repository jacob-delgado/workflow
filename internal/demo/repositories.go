// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// The directories kept as favorites besides web: notes, and a site since
// removed.
const (
	notesDir   = home + "/notes"
	oldSiteDir = home + "/old-site"
)

// repositoriesDeps is where the demo works: api, from its root, with web, the
// notes and a removed directory as favorites, as the web mock has them
// (mockRepositories.ts).
func (w *World) repositoriesDeps() seams.Repositories {
	return seams.Repositories{
		Here: apiPlace(), Home: home,
		Look:      look,
		Worktrees: w.worktrees,
	}
}

// apiPlace is the api repository, where the demo works.
func apiPlace() seams.Place {
	return seams.Place{
		Dir: apiRoot, Root: apiRoot,
		Remote: forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: forgeRepository},
		Config: config.Files{Repo: apiRoot + "/.workflow.json", Home: home + "/.workflow.json"},
	}
}

// favorites are the directories kept as favorites, by path.
func favorites() []string {
	return []string{webRoot, notesDir, oldSiteDir}
}

// look reads a directory as a place: the demo's repositories and the notes,
// and any other directory, the removed site among them, not found.
func look(_ context.Context, dir string) (seams.Place, error) {
	web := forge.Repo{Kind: forge.KindGitHub, Host: githubHost, Path: "acme/web"}
	places := map[string]seams.Place{
		apiRoot:  apiPlace(),
		webRoot:  {Dir: webRoot, Root: webRoot, Remote: web},
		notesDir: {Dir: notesDir},
	}

	place, found := places[dir]
	if !found {
		return seams.Place{}, fmt.Errorf("%s: %w", dir, workdirs.ErrNotFound)
	}

	return place, nil
}

// worktrees are api's working trees: its own, on whatever branch is checked
// out, one for a feature git keeps locked, one detached for a review, and one
// whose directory is gone.
func (w *World) worktrees(context.Context) ([]gitrepo.Worktree, error) {
	here := locked(w, w.repo.checkedOut)

	return []gitrepo.Worktree{
		{Dir: apiRoot, Branch: here.Name, Head: here.Head},
		{Dir: apiRoot + "-feat-PROJ-7-rate-limits", Branch: "feat/PROJ-7-rate-limits", Head: "9c41d02", Locked: true},
		{Dir: apiRoot + "-review", Head: "5e0b7aa", Detached: true},
		{Dir: apiRoot + "-spike", Branch: "spike/cache", Head: "d1f8e30", Missing: true},
	}, nil
}
