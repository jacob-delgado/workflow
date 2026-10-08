// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"path/filepath"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// RepositoriesView is what a surface reads where it works from: the
// directories it can work in; your favorites, nil when the store keeps none;
// and whether a favorite can be marked or forgotten there.
type RepositoriesView struct {
	Repositories  seams.Repositories
	Favorites     func() ([]string, error)
	FavoritesKept bool
}

// Report is where a surface works, its repository's worktrees and your
// favorites, each looked at on disk now: what GET /api/repositories answers
// and `workflow repositories --json` prints. describe words why the worktrees
// could not be read. Favorites that cannot be read are listed as none, and
// why is returned beside the rest.
func (v RepositoriesView) Report(describe func(error) string) (api.Repositories, error) {
	worktrees, worktreesErr := v.worktrees(describe)
	favorites, err := v.favorites()

	return api.Repositories{
		Here:           placeDTO(v.Repositories.Here, v.Repositories.Home),
		Worktrees:      worktrees,
		WorktreesError: worktreesErr,
		Favorites:      favorites,
		FavoritesKept:  v.FavoritesKept,
	}, err
}

// worktrees is the repository's worktrees, each with what it has checked
// out, or why they could not be read, in describe's words. Outside a
// repository there are none.
func (v RepositoriesView) worktrees(describe func(error) string) ([]api.Worktree, string) {
	repositories := v.Repositories

	worktrees := []api.Worktree{}
	if repositories.Worktrees == nil {
		return worktrees, ""
	}

	read, err := repositories.Worktrees()
	if err != nil {
		return worktrees, describe(err)
	}

	for _, worktree := range read {
		worktrees = append(worktrees, api.Worktree{
			Dir: worktree.Dir, Shown: workdirs.Shown(worktree.Dir, repositories.Home), Branch: worktree.Branch,
			Head: worktree.ShortHead(), State: worktreeState(worktree, repositories.Here.Root),
			Locked: worktree.Locked,
		})
	}

	return worktrees, ""
}

// worktreeState is whether a worktree is the one the server works in,
// another, or one whose directory is gone.
func worktreeState(worktree gitrepo.Worktree, here string) api.WorktreeState {
	switch {
	case worktree.Missing:
		return api.WorktreeStateMissing
	case worktree.Dir == here:
		return api.WorktreeStateHere
	default:
		return api.WorktreeStateWorktree
	}
}

// favorites is your favorites, each with what is there now, and why they
// could not be read when they could not.
func (v RepositoriesView) favorites() ([]api.Favorite, error) {
	favorites := []api.Favorite{}
	if v.Favorites == nil {
		return favorites, nil
	}

	dirs, err := v.Favorites()

	for _, dir := range dirs {
		favorites = append(favorites, v.favorite(dir))
	}

	return favorites, err
}

// favorite is the favorite dir, with what is there now.
func (v RepositoriesView) favorite(dir string) api.Favorite {
	repositories := v.Repositories
	favorite := api.Favorite{Dir: dir, Shown: workdirs.Shown(dir, repositories.Home), State: api.FavoriteStateMissing}

	place, lookErr := lookAt(repositories.Look, dir)

	switch {
	case dir == repositories.Here.Dir || workdirs.Same(dir, repositories.Here.Dir):
		favorite.State = api.FavoriteStateHere
	case lookErr != nil:
	case place.Root == "":
		favorite.State = api.FavoriteStateDirectory
	default:
		favorite.State, favorite.Origin = api.FavoriteStateRepository, originOf(place)
	}

	return favorite
}

// lookAt reads dir through look, or says it cannot be read when there is no
// look.
func lookAt(look func(dir string) (seams.Place, error), dir string) (seams.Place, error) {
	if look == nil {
		return seams.Place{}, workdirs.ErrUnreadable
	}

	return look(dir)
}

// placeDTO is a place as the API answers it, each path also written from
// home.
func placeDTO(place seams.Place, home string) api.Place {
	config := place.Config.Each()
	slices.Reverse(config)

	shownConfig := make([]string, 0, len(config))
	for _, file := range config {
		shownConfig = append(shownConfig, workdirs.Shown(file, home))
	}

	dto := api.Place{
		Dir: place.Dir, Shown: workdirs.Shown(place.Dir, home), Origin: originOf(place), Config: shownConfig,
	}

	if place.Root != "" {
		dto.Root, dto.RootShown = place.Root, workdirs.Shown(place.Root, home)

		within, err := filepath.Rel(place.Root, place.Dir)
		if err == nil && within != "." {
			dto.Within = filepath.ToSlash(within)
		}
	}

	return dto
}

// originOf is a place's origin as host and path, or empty.
func originOf(place seams.Place) string {
	if place.Remote.Host == "" {
		return ""
	}

	return place.Remote.Host + "/" + place.Remote.Path
}
