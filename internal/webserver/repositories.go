// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/report"
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
			problem(api.ProblemCodeNotFound, "there is no such directory")), nil
	case errors.Is(err, errWriteInFlight):
		return api.SwitchRepository409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict, err.Error())), nil
	case err != nil:
		return problemAnswer[api.SwitchRepositorydefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.SwitchRepository200JSONResponse(next.repositoriesDTO()), nil
}

// AddFavorite keeps the directory named as a favorite.
func (s *server) AddFavorite(
	_ context.Context, request api.AddFavoriteRequestObject,
) (api.AddFavoriteResponseObject, error) {
	err := s.changeFavorite(s.deps.Store.Favor, request.Body.Dir)
	if err != nil {
		return problemAnswer[api.AddFavoritedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.AddFavorite200JSONResponse(s.repositoriesDTO()), nil
}

// RemoveFavorite forgets the directory named as a favorite.
func (s *server) RemoveFavorite(
	_ context.Context, request api.RemoveFavoriteRequestObject,
) (api.RemoveFavoriteResponseObject, error) {
	err := s.changeFavorite(s.deps.Store.Unfavor, request.Params.Dir)
	if err != nil {
		return problemAnswer[api.RemoveFavoritedefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
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
			problem(api.ProblemCodeUnprocessable, errNoSwitching.Error())), nil
	}

	typed := ""
	if request.Params.Path != nil {
		typed = *request.Params.Path
	}

	dir := workdirs.Resolve(typed, repositories.Here.Dir, repositories.Home)

	listing, err := repositories.Subdirectories(dir, "")
	if err != nil {
		return problemAnswer[api.GetDirectoriesdefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
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
// at on disk now, the worktrees' failure worded by fault so no path or host
// reaches the wire.
func (s *server) repositoriesDTO() api.Repositories {
	view := report.RepositoriesView{
		Repositories: s.deps.Repositories, Favorites: s.deps.Store.Favorites,
		FavoritesKept: s.deps.Store.Favor != nil && !s.info.DryRun,
	}

	answer, err := view.Report(func(err error) string {
		body := s.fault(err)

		return body.Detail
	})
	if err != nil {
		s.unexpected(err)
	}

	return answer
}
