// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/codeowners"
)

// OwnerSeams are what the owners of a pull request's changes are read through:
// the paths the branch changes since base, the CODEOWNERS file base holds, and
// who is opening the pull request. A nil ChangedPaths or CodeOwnersAt means
// there are no owners to read; a nil Author leaves nobody out.
type OwnerSeams struct {
	ChangedPaths func(base string) ([]string, error)
	CodeOwnersAt func(base string) (codeowners.File, bool, error)
	Author       func() (string, error)
}

// OwnersOf is who owns the paths the branch changes since base, as CODEOWNERS
// on base names them, without the author: nobody is asked to review their own
// pull request. With no CODEOWNERS on base, or no way to read one, it is
// nobody.
func OwnersOf(seams OwnerSeams, base string) (codeowners.Owners, error) {
	if seams.ChangedPaths == nil || seams.CodeOwnersAt == nil {
		return codeowners.Owners{}, nil
	}

	paths, err := seams.ChangedPaths(base)
	if err != nil {
		return codeowners.Owners{}, fmt.Errorf("listing the changed paths: %w", err)
	}

	file, found, err := seams.CodeOwnersAt(base)
	if err != nil {
		return codeowners.Owners{}, fmt.Errorf("reading CODEOWNERS: %w", err)
	}

	if !found {
		return codeowners.Owners{}, nil
	}

	return withoutAuthor(file.OwnersOf(paths), seams.Author)
}

// withoutAuthor leaves the author out of owners' people, compared without
// case as the forges compare handles.
func withoutAuthor(owners codeowners.Owners, author func() (string, error)) (codeowners.Owners, error) {
	if author == nil || len(owners.Users) == 0 {
		return owners, nil
	}

	name, err := author()
	if err != nil {
		return codeowners.Owners{}, fmt.Errorf("reading who the author is: %w", err)
	}

	owners.Users = slices.DeleteFunc(owners.Users, func(user string) bool { return strings.EqualFold(user, name) })

	return owners, nil
}

// ProposedReviewers is the owners of the branch's changes as the reviewers a
// pull request proposes: people, then teams as org/team. It is a proposal, so
// a failure to read the owners proposes nobody rather than holding the pull
// request back. A GitLab group stays group/subgroup here: the forge expands it
// to its members when the merge request is opened.
func ProposedReviewers(seams OwnerSeams, base string) []string {
	owners, err := OwnersOf(seams, base)
	if err != nil {
		return nil
	}

	return slices.Concat(owners.Users, owners.Teams)
}

// SplitReviewers tells the teams among reviewer names from the people: a name
// holding a slash is a team (org/team, or a GitLab group/subgroup).
func SplitReviewers(names []string) ([]string, []string) {
	var users, teams []string

	for _, name := range names {
		if strings.Contains(name, "/") {
			teams = append(teams, name)
		} else {
			users = append(users, name)
		}
	}

	return users, teams
}
