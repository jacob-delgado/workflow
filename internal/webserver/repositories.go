// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

var (
	// ErrConfigurationRefused is a directory whose configuration the
	// interface would refuse to start on, which a switch is refused for.
	ErrConfigurationRefused = errors.New("the configuration there binds keys workflow refuses")
	// ErrConfigurationUnreadable is a directory whose configuration did not
	// load, which a switch is refused for rather than serve the defaults.
	ErrConfigurationUnreadable = errors.New("the configuration there did not load")
	// errFavoritesNotKept is a favorite asked for of a store that keeps none.
	errFavoritesNotKept = errors.New("favorites are not kept: the store is turned off")
)

// GetRepositories says where the server works, and your favorites.
func (s *server) GetRepositories(
	context.Context, api.GetRepositoriesRequestObject,
) (api.GetRepositoriesResponseObject, error) {
	return api.GetRepositories200JSONResponse(s.repositoriesDTO()), nil
}

// SwitchRepository serves the directory named from now on, answering for it.
func (s *server) SwitchRepository(
	_ context.Context, request api.SwitchRepositoryRequestObject,
) (api.SwitchRepositoryResponseObject, error) {
	next, err := s.worlds.switchTo(filepath.Clean(request.Body.Dir))

	switch {
	case errors.Is(err, workdirs.ErrNotFound):
		return api.SwitchRepository404ApplicationProblemPlusJSONResponse(
			problem(api.NotFound, "there is no such directory")), nil
	case errors.Is(err, errWriteInFlight):
		return api.SwitchRepository409ApplicationProblemPlusJSONResponse(problem(api.Conflict, err.Error())), nil
	case err != nil:
		body, code := s.fault(err)

		return api.SwitchRepositorydefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.SwitchRepository200JSONResponse(next.repositoriesDTO()), nil
}

// AddFavorite keeps the directory named as a favorite.
func (s *server) AddFavorite(
	_ context.Context, request api.AddFavoriteRequestObject,
) (api.AddFavoriteResponseObject, error) {
	err := s.changeFavorite(s.deps.Favor, request.Body.Dir)
	if err != nil {
		body, code := s.fault(err)

		return api.AddFavoritedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.AddFavorite200JSONResponse(s.repositoriesDTO()), nil
}

// RemoveFavorite forgets the directory named as a favorite.
func (s *server) RemoveFavorite(
	_ context.Context, request api.RemoveFavoriteRequestObject,
) (api.RemoveFavoriteResponseObject, error) {
	err := s.changeFavorite(s.deps.Unfavor, request.Params.Dir)
	if err != nil {
		body, code := s.fault(err)

		return api.RemoveFavoritedefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.RemoveFavorite200JSONResponse(s.repositoriesDTO()), nil
}

// changeFavorite marks or forgets dir through change, refusing a path that is
// not absolute and a store that keeps nothing.
func (s *server) changeFavorite(change func(dir string) error, dir string) error {
	switch {
	case change == nil:
		return errFavoritesNotKept
	case !filepath.IsAbs(dir):
		return workdirs.ErrNotAbsolute
	default:
		return change(dir)
	}
}

// GetDirectories lists the directories in the one asked for, or in where the
// server works.
func (s *server) GetDirectories(
	_ context.Context, request api.GetDirectoriesRequestObject,
) (api.GetDirectoriesResponseObject, error) {
	repositories := s.deps.Repositories
	if repositories.Subdirectories == nil {
		return api.GetDirectories422ApplicationProblemPlusJSONResponse(
			problem(api.Unprocessable, errNoSwitching.Error())), nil
	}

	typed := ""
	if request.Params.Path != nil {
		typed = *request.Params.Path
	}

	dir := workdirs.Resolve(typed, repositories.Here.Dir, repositories.Home)

	listing, err := repositories.Subdirectories(dir, "")
	if err != nil {
		body, code := s.fault(err)

		return api.GetDirectoriesdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetDirectories200JSONResponse(listingDTO(dir, repositories.Home, listing)), nil
}

// listingDTO is the directories in dir as the API answers them.
func listingDTO(dir, home string, listing workdirs.Listing) api.DirectoryListing {
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}

	entries := make([]api.DirectoryEntry, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		entries = append(entries, api.DirectoryEntry{
			Name: entry.Name, Path: filepath.Join(dir, entry.Name), Repository: entry.Repository,
		})
	}

	return api.DirectoryListing{
		Path: dir, Shown: workdirs.Shown(dir, home), Parent: parent, Entries: entries, Truncated: listing.Truncated,
	}
}

// repositoriesDTO is where the server works and your favorites, each looked
// at on disk now.
func (s *server) repositoriesDTO() api.Repositories {
	repositories := s.deps.Repositories

	return api.Repositories{
		Here:          placeDTO(repositories.Here, repositories.Home),
		Favorites:     s.favoritesDTO(),
		FavoritesKept: s.deps.Favor != nil && !s.info.DryRun,
	}
}

// favoritesDTO is your favorites, each with what is there now.
func (s *server) favoritesDTO() []api.Favorite {
	favorites := []api.Favorite{}
	if s.deps.Favorites == nil {
		return favorites
	}

	dirs, err := s.deps.Favorites()
	if err != nil {
		s.unexpected(err)
	}

	repositories := s.deps.Repositories

	for _, dir := range dirs {
		favorite := api.Favorite{Dir: dir, Shown: workdirs.Shown(dir, repositories.Home), State: api.FavoriteMissing}

		place, lookErr := lookAt(repositories.Look, dir)

		switch {
		case dir == repositories.Here.Dir || workdirs.Same(dir, repositories.Here.Dir):
			favorite.State = api.FavoriteHere
		case lookErr != nil:
		case place.Root == "":
			favorite.State = api.FavoriteDirectory
		default:
			favorite.State, favorite.Origin = api.FavoriteRepository, originOf(place)
		}

		favorites = append(favorites, favorite)
	}

	return favorites
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
