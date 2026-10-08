// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/store"
)

// errNoLocalData refuses the Local data area on a server wired with no store.
var errNoLocalData = errors.New("the local data is not available here")

// What a failed list or removal says, in words that name no path.
const (
	noStoreDirDetail   = "there is no directory to keep local data in: no home directory is set"
	notStoreFileDetail = "something other than the store's own file, a symlink or a directory, " +
		"sits where a database belongs; nothing was removed"
	heldOpenDetail = "a database file could not be removed, as one another program holds open can be, " +
		"and others may already be gone; read the listing again to see what is left, " +
		"then close other workflow sessions and try again"
)

// GetLocalData lists the store's directory and the database files in it.
func (s *server) GetLocalData(
	ctx context.Context, _ api.GetLocalDataRequestObject,
) (api.GetLocalDataResponseObject, error) {
	data, err := s.localData(ctx)
	if err != nil {
		return problemAnswer[api.GetLocalDatadefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.GetLocalData200JSONResponse(data), nil
}

// RemoveLocalData removes the cache, or with scope all the kept file too, and
// answers what is left.
func (s *server) RemoveLocalData(
	ctx context.Context, request api.RemoveLocalDataRequestObject,
) (api.RemoveLocalDataResponseObject, error) {
	if s.deps.RemoveLocalData == nil {
		return api.RemoveLocalData422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			errNoLocalData.Error())), nil
	}

	scope := map[api.RemoveLocalDataParamsScope]store.CleanScope{
		api.RemoveLocalDataParamsScopeCache: store.CleanCache,
		api.RemoveLocalDataParamsScopeAll:   store.CleanAll,
	}[request.Params.Scope]

	err := s.keptWrite(func() error { return s.deps.RemoveLocalData(scope) })
	if err == nil {
		var data api.LocalData

		data, err = s.localData(ctx)
		if err == nil {
			return api.RemoveLocalData200JSONResponse(data), nil
		}
	}

	return problemAnswer[api.RemoveLocalDatadefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
}

// localData reads the store's directory and files into the answer's shape.
func (s *server) localData(ctx context.Context) (api.LocalData, error) {
	if s.deps.LocalData == nil {
		return api.LocalData{}, errNoLocalData
	}

	dir, files, err := s.deps.LocalData(ctx)
	if err != nil {
		return api.LocalData{}, err
	}

	data := api.LocalData{
		Dir: dir, Files: make([]api.LocalDataFile, 0, len(files)),
		Consequences: api.LocalDataConsequences{
			Cache: store.CleanCache.Consequence(), All: store.CleanAll.Consequence(),
		},
	}

	if s.info.DryRun {
		dryRun := true
		data.DryRun = &dryRun
	}

	for _, file := range files {
		data.Files = append(data.Files, localDataFileDTO(file))
	}

	return data, nil
}

// localDataFileDTO is one database file as the answer carries it.
func localDataFileDTO(file store.DataFile) api.LocalDataFile {
	holds := make([]api.LocalDataHeld, 0, len(file.Holds))
	for _, held := range file.Holds {
		holds = append(holds, api.LocalDataHeld{What: held.What, Count: held.Count})
	}

	return api.LocalDataFile{
		Name: file.Name, Kind: api.LocalDataFileKind(file.Kind), Bytes: file.Bytes, Size: store.HumanBytes(file.Bytes),
		Holds: holds,
	}
}

// localDataFaults are the failures of a list or a removal of the local data:
// a file held open is a conflict with the state on disk; no store, no
// directory or something not the store's own in a file's place cannot be
// carried out.
func localDataFaults() []faultClass {
	return []faultClass{
		{causes: []error{errNoLocalData}, code: api.ProblemCodeUnprocessable, detail: errNoLocalData.Error()},
		{causes: []error{store.ErrNoDir}, code: api.ProblemCodeUnprocessable, detail: noStoreDirDetail},
		{causes: []error{store.ErrCleanRefused}, code: api.ProblemCodeUnprocessable, detail: notStoreFileDetail},
		{causes: []error{store.ErrNotCleaned}, code: api.ProblemCodeConflict, detail: heldOpenDetail},
	}
}
