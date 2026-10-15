// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// GetRepoGroups lists the user groups this repository's announcements may
// tag.
func (s *server) GetRepoGroups(
	ctx context.Context, _ api.GetRepoGroupsRequestObject,
) (api.GetRepoGroupsResponseObject, error) {
	groups, err := s.repoGroups(ctx)
	if err != nil {
		return problem.Answer[api.GetRepoGroupsdefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
	}

	return api.GetRepoGroups200JSONResponse(groups), nil
}

// SetRepoGroups replaces this repository's user groups with the ones named,
// each labeled as the workspace's directory has it.
func (s *server) SetRepoGroups(
	ctx context.Context, request api.SetRepoGroupsRequestObject,
) (api.SetRepoGroupsResponseObject, error) {
	err := s.setRepoGroups(ctx, request.Body.Ids)
	if err == nil {
		var groups api.RepoGroups

		groups, err = s.repoGroups(ctx)
		if err == nil {
			return api.SetRepoGroups200JSONResponse(groups), nil
		}
	}

	return problem.Answer[api.SetRepoGroupsdefaultApplicationProblemPlusJSONResponse](s.peopleFault(err)), nil
}

// repoGroups is this repository's user groups, with its name, in the Slack
// workspace read under ctx.
func (s *server) repoGroups(ctx context.Context) (api.RepoGroups, error) {
	if s.deps.Store.RepoGroups == nil {
		return api.RepoGroups{}, errNoPeopleStore
	}

	var groups []loop.SlackTarget

	err := s.inWorkspace(ctx, func(workspace string) error {
		var err error

		groups, err = s.deps.Store.RepoGroups(ctx, workspace)

		return err
	})
	if err != nil {
		return api.RepoGroups{}, err
	}

	return api.RepoGroups{Repository: s.info.Repository, Groups: slackTargetsDTO(groups)}, nil
}

// setRepoGroups keeps ids, each once, as this repository's user groups,
// asking Slack under ctx.
func (s *server) setRepoGroups(ctx context.Context, ids []string) error {
	if s.deps.Store.SetRepoGroups == nil || s.deps.Store.RepoGroups == nil {
		return errNoPeopleStore
	}

	return s.inWorkspace(ctx, func(workspace string) error {
		return s.keptWrite(func() error {
			saved, err := s.deps.Store.RepoGroups(ctx, workspace)
			if err != nil {
				return err
			}

			groups, err := s.labelGroups(ctx, ids, saved)
			if err != nil {
				return err
			}

			return s.deps.Store.SetRepoGroups(ctx, workspace, groups)
		})
	})
}

// labelGroups is ids each once, labeled as saved when one already is — so a
// group Slack no longer lists, or a token that cannot read them, keeps it —
// and from the workspace's directory when it is new. Only a new group needs
// the directory, read under ctx.
func (s *server) labelGroups(ctx context.Context, ids []string, saved []loop.SlackTarget) ([]loop.SlackTarget, error) {
	groups := make([]loop.SlackTarget, 0, len(ids))

	for _, groupID := range ids {
		if slices.ContainsFunc(groups, hasID(groupID)) {
			continue
		}

		if index := slices.IndexFunc(saved, hasID(groupID)); index >= 0 {
			groups = append(groups, saved[index])

			continue
		}

		group, err := s.slackGroup(ctx, groupID)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// hasID reports whether a Slack user or group is the one id names.
func hasID(id string) func(loop.SlackTarget) bool {
	return func(target loop.SlackTarget) bool { return target.ID == id }
}
