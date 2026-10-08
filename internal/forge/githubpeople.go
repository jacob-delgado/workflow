// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// githubAddPeople requests reviewers and adds assignees and labels to a pull
// request already opened. GitHub names each list by the same key it reads it
// back under, and takes reviewers on the pull while assignees and labels go on
// its issue side. Reviewers it would not add are reported only once the
// assignees and labels are in, so a mistyped name costs nothing else.
func githubAddPeople(ctx context.Context, client Client, repo Repo, number int, request NewPullRequest) error {
	pull := githubPullPath(repo, number)
	issue := githubRepoPath(repo) + issuesSegment + "/" + strconv.Itoa(number)

	reviewersErr := githubRequestReviewers(ctx, client, repo, pull, request)
	if reviewersErr != nil && !errors.Is(reviewersErr, ErrSomePeopleNotAdded) {
		return reviewersErr
	}

	err := githubPostList(ctx, client, repo, issue+"/assignees", "assignees", request.Assignees)
	if err != nil {
		return err
	}

	err = githubPostList(ctx, client, repo, issue+"/labels", "labels", request.Labels)
	if err != nil {
		return err
	}

	return reviewersErr
}

// githubReviewersBody is the body that requests reviewers: users by login and
// teams by slug, each left out when empty.
type githubReviewersBody struct {
	Reviewers     []string `json:"reviewers,omitempty"`
	TeamReviewers []string `json:"team_reviewers,omitempty"`
}

// githubTeam is a team reviewer as CODEOWNERS names it, "org/team", and the
// slug GitHub asks for it by.
type githubTeam struct {
	name, slug string
}

// githubRequestReviewers requests a pull request's reviewers, users and teams in
// one call. GitHub turns the whole call down for one name it cannot request, so
// a call turned down is asked again a name at a time, and only the names still
// turned down are reported. A team of another organization is never asked for:
// GitHub would read its slug as the repository's own organization's team.
func githubRequestReviewers(ctx context.Context, client Client, repo Repo, pull string, request NewPullRequest) error {
	teams, foreign := githubTeamsOf(repo, request.TeamReviewers)

	missed := missedPeople{}
	for _, team := range foreign {
		missed.miss(team, ErrTeamOfAnotherOrg)
	}

	body := githubReviewersBody{Reviewers: request.Reviewers, TeamReviewers: slugsOf(teams)}
	if len(body.Reviewers)+len(body.TeamReviewers) == 0 {
		return missed.err()
	}

	err := githubAskReviewers(ctx, client, repo, pull, body)
	if err == nil {
		return missed.err()
	}

	if !errors.Is(err, ErrRejected) && !errors.Is(err, ErrUnexpectedStatus) {
		return err
	}

	return githubReviewersOneByOne(ctx, client, repo, pull, request.Reviewers, teams, missed)
}

// githubReviewersOneByOne requests each user, then each team, alone, and
// reports the ones turned down with those already missed.
func githubReviewersOneByOne(
	ctx context.Context, client Client, repo Repo, pull string, users []string, teams []githubTeam,
	missed missedPeople,
) error {
	ask := func(name string, body githubReviewersBody) {
		err := githubAskReviewers(ctx, client, repo, pull, body)
		if err != nil {
			missed.miss(name, err)
		}
	}

	for _, user := range users {
		ask(user, githubReviewersBody{Reviewers: []string{user}})
	}

	for _, team := range teams {
		ask(team.name, githubReviewersBody{TeamReviewers: []string{team.slug}})
	}

	return missed.err()
}

// githubAskReviewers sends one request for reviewers.
func githubAskReviewers(ctx context.Context, client Client, repo Repo, pull string, body githubReviewersBody) error {
	_, err := repoCall[json.RawMessage](ctx, client, repo, http.MethodPost, pull+"/requested_reviewers", body)

	return err
}

// githubTeamsOf splits "org/team" names into the teams of the repository's
// own organization, with their slugs, and the names of any other's. GitHub
// reads an organization's name without regard to case.
func githubTeamsOf(repo Repo, names []string) ([]githubTeam, []string) {
	org, _, _ := strings.Cut(repo.Path, "/")

	var (
		own     []githubTeam
		foreign []string
	)

	for _, name := range names {
		teamOrg, slug, found := strings.Cut(name, "/")
		if found && strings.EqualFold(teamOrg, org) {
			own = append(own, githubTeam{name: name, slug: slug})
		} else {
			foreign = append(foreign, name)
		}
	}

	return own, foreign
}

// slugsOf is each team's slug.
func slugsOf(teams []githubTeam) []string {
	slugs := make([]string, 0, len(teams))
	for _, team := range teams {
		slugs = append(slugs, team.slug)
	}

	return slugs
}

// githubPostList posts a named list to an endpoint, doing nothing when the list
// is empty so no needless request is made.
func githubPostList(ctx context.Context, client Client, repo Repo, path, key string, values []string) error {
	if len(values) == 0 {
		return nil
	}

	_, err := repoCall[json.RawMessage](ctx, client, repo, http.MethodPost, path, map[string][]string{key: values})

	return err
}
