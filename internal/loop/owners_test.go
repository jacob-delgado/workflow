// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// The base the pull request targets, a team and a person owning the API, and
// a path in it.
const (
	targetBase = "main"
	ownerTeam  = "acme/control-plane"
	ownerUser  = "ana"
	apiPath    = "api/pull.go"
)

// errForgeDown is a forge that could not be asked.
var errForgeDown = errors.New("forge down")

// ownersFile is the CODEOWNERS the base holds in these tests: the API is
// owned by ana and a team, the docs by the author and bo.
const ownersFile = "/api/ @ana @acme/control-plane\n/docs/ @Me @bo\n"

// ownerSeams answer as a branch changing the API and the docs, on a base whose
// CODEOWNERS is ownersFile, opened by "me".
func ownerSeams() loop.OwnerSeams {
	return loop.OwnerSeams{
		ChangedPaths: func(context.Context, string) ([]string, error) {
			return []string{apiPath, "docs/usage.md"}, nil
		},
		CodeOwnersAt: func(context.Context, string) (codeowners.File, bool, error) {
			return codeowners.Parse(ownersFile, codeowners.GitHub), true, nil
		},
		Author: func(context.Context) (string, error) { return "me", nil },
	}
}

func TestOwnersOfNamesTheChangedPathsOwnersButTheAuthor(t *testing.T) {
	t.Parallel()

	// Act
	owners, err := loop.OwnersOf(t.Context(), ownerSeams(), targetBase)
	// Assert
	if err != nil {
		t.Fatalf("OwnersOf returned %v", err)
	}

	if !slices.Equal(owners.Users, []string{ownerUser, "bo"}) || !slices.Equal(owners.Teams, []string{ownerTeam}) {
		t.Errorf("OwnersOf = %+v, want users [ana bo] and team acme/control-plane", owners)
	}
}

func TestOwnersOfReadsTheBaseItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	var diffed, read string

	seams := ownerSeams()
	seams.ChangedPaths = func(_ context.Context, base string) ([]string, error) {
		diffed = base

		return nil, nil
	}
	seams.CodeOwnersAt = func(_ context.Context, base string) (codeowners.File, bool, error) {
		read = base

		return codeowners.File{}, false, nil
	}

	// Act
	_, err := loop.OwnersOf(t.Context(), seams, "release")

	// Assert
	if err != nil || diffed != "release" || read != "release" {
		t.Errorf("OwnersOf diffed against %q and read CODEOWNERS at %q (err %v), want release for both",
			diffed, read, err)
	}
}

func TestOwnersOfIsNobodyWithoutAFileOrSeams(t *testing.T) {
	t.Parallel()

	cases := map[string]func(loop.OwnerSeams) loop.OwnerSeams{
		"no CODEOWNERS on the base": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.CodeOwnersAt = func(context.Context, string) (codeowners.File, bool, error) {
				return codeowners.File{}, false, nil
			}

			return seams
		},
		"no way to list the changes": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.ChangedPaths = nil

			return seams
		},
		"no way to read CODEOWNERS": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.CodeOwnersAt = nil

			return seams
		},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			owners, err := loop.OwnersOf(t.Context(), change(ownerSeams()), targetBase)

			// Assert
			if err != nil || len(owners.Users)+len(owners.Teams) != 0 {
				t.Errorf("OwnersOf = %+v, %v; want nobody and no error", owners, err)
			}
		})
	}
}

func TestOwnersOfKeepsEveryoneWithNoAuthorToLeaveOut(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := ownerSeams()
	seams.Author = nil

	// Act
	owners, err := loop.OwnersOf(t.Context(), seams, targetBase)

	// Assert
	if err != nil || !slices.Equal(owners.Users, []string{ownerUser, "Me", "bo"}) {
		t.Errorf("OwnersOf = %+v, %v; want users [ana Me bo]", owners, err)
	}
}

func TestOwnersOfReportsWhatItCouldNotRead(t *testing.T) {
	t.Parallel()

	cases := map[string]func(loop.OwnerSeams) loop.OwnerSeams{
		"the changes": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.ChangedPaths = func(context.Context, string) ([]string, error) { return nil, errSeam }

			return seams
		},
		"CODEOWNERS": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.CodeOwnersAt = func(context.Context, string) (codeowners.File, bool, error) {
				return codeowners.File{}, false, errSeam
			}

			return seams
		},
		"the author": func(seams loop.OwnerSeams) loop.OwnerSeams {
			seams.Author = func(context.Context) (string, error) { return "", errSeam }

			return seams
		},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := loop.OwnersOf(t.Context(), change(ownerSeams()), targetBase)

			// Assert
			if !errors.Is(err, errSeam) {
				t.Errorf("OwnersOf returned %v, want the seam's error", err)
			}
		})
	}
}

func TestProposedReviewersListsPeopleThenTeams(t *testing.T) {
	t.Parallel()

	// Act
	proposed := loop.ProposedReviewers(t.Context(), ownerSeams(), targetBase)

	// Assert
	if want := []string{ownerUser, "bo", ownerTeam}; !slices.Equal(proposed, want) {
		t.Errorf("ProposedReviewers = %q, want %q", proposed, want)
	}
}

func TestProposedReviewersIsNobodyWhenTheOwnersCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := ownerSeams()
	seams.ChangedPaths = func(context.Context, string) ([]string, error) { return nil, errSeam }

	// Act
	proposed := loop.ProposedReviewers(t.Context(), seams, targetBase)

	// Assert
	if proposed != nil {
		t.Errorf("ProposedReviewers = %q, want nil", proposed)
	}
}

func TestSplitReviewersTellsTeamsFromPeople(t *testing.T) {
	t.Parallel()

	// Act
	users, teams := loop.SplitReviewers([]string{ownerUser, ownerTeam, "bo", "group/sub/team"})

	// Assert
	wantTeams := []string{ownerTeam, "group/sub/team"}
	if !slices.Equal(users, []string{ownerUser, "bo"}) || !slices.Equal(teams, wantTeams) {
		t.Errorf("SplitReviewers = %q, %q; want [ana bo] and the two teams", users, teams)
	}
}

func TestComposePullProposesTheCodeOwnersAsReviewers(t *testing.T) {
	t.Parallel()

	// Arrange
	var read string

	seams := pullSeams()
	seams.Owners = ownerSeams()
	seams.Owners.ChangedPaths = func(_ context.Context, base string) ([]string, error) {
		read = base

		return []string{apiPath}, nil
	}

	// Act
	draft, _, err := loop.ComposePull(t.Context(), seams, options())
	// Assert
	if err != nil {
		t.Fatalf("ComposePull returned %v", err)
	}

	proposedTeams := draft.TeamReviewers
	if !slices.Equal(draft.Reviewers, []string{ownerUser}) || !slices.Equal(proposedTeams, []string{ownerTeam}) ||
		read != targetBase {
		t.Errorf("ComposePull proposed %q and teams %q against %q, want ana and %s against %s",
			draft.Reviewers, proposedTeams, read, ownerTeam, targetBase)
	}
}

func TestComposePullProposesTheCodeOwnersOfTheBaseChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch was read to have left main; a surface chose release-1.2.
	const chosen = "release-1.2"

	var changedSince, ownersAt string

	seams := pullSeams()
	seams.BranchOnto = func(_ context.Context, base string) (gitrepo.Branch, error) {
		branch := openable(branchName)
		branch.Base = "origin/" + base

		return branch, nil
	}
	seams.Owners = ownerSeams()
	seams.Owners.ChangedPaths = func(_ context.Context, base string) ([]string, error) {
		changedSince = base

		return []string{apiPath}, nil
	}
	seams.Owners.CodeOwnersAt = func(_ context.Context, base string) (codeowners.File, bool, error) {
		ownersAt = base

		return codeowners.Parse(ownersFile, codeowners.GitHub), true, nil
	}

	opts := options()
	opts.Base = chosen

	// Act
	draft, _, err := loop.ComposePull(t.Context(), seams, opts)

	// Assert
	if err != nil || draft.Base != chosen || changedSince != chosen || ownersAt != chosen {
		t.Errorf("ComposePull = %v, into %q, owners read since %q at %q; want all three %s",
			err, draft.Base, changedSince, ownersAt, chosen)
	}
}

func TestOwnersOfCountsANameTheForgeKnowsAsAGroupAmongTheTeams(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab spells a top-level group @bo, as it spells a user; the forge
	// knows bo as a group and ana as a person.
	seams := ownerSeams()
	seams.IsGroup = func(_ context.Context, name string) (bool, error) { return name == "bo", nil }

	// Act
	owners, err := loop.OwnersOf(t.Context(), seams, targetBase)

	// Assert
	if err != nil || !slices.Equal(owners.Users, []string{ownerUser}) ||
		!slices.Equal(owners.Teams, []string{ownerTeam, "bo"}) {
		t.Errorf("OwnersOf = %+v, %v; want ana a person, and bo a team beside acme/control-plane", owners, err)
	}
}

func TestOwnersOfKeepsANameTheForgeCannotClassifyAsAPerson(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge knows bo as a group but cannot be asked about ana: one
	// failed lookup must not cost the announcement every other owner.
	seams := ownerSeams()
	seams.IsGroup = func(_ context.Context, name string) (bool, error) {
		if name == ownerUser {
			return false, errForgeDown
		}

		return name == "bo", nil
	}

	// Act
	owners, err := loop.OwnersOf(t.Context(), seams, targetBase)

	// Assert
	if err != nil || !slices.Equal(owners.Users, []string{ownerUser}) ||
		!slices.Equal(owners.Teams, []string{ownerTeam, "bo"}) {
		t.Errorf("OwnersOf = %+v, %v; want ana kept a person, and bo a team beside acme/control-plane", owners, err)
	}
}
