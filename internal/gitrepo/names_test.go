// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestReadBranchRefusesANameThatCannotBeShownAsItIs(t *testing.T) {
	t.Parallel()

	// git allows more in a ref's name than can be shown as it is: a direction
	// override, a character with no width. The name is handed to git and to the
	// forge as well as drawn, so there is no cleaning it; a branch that cannot
	// be shown as it is, is not worked on.
	cases := map[string]map[string]reply{
		"the branch":   {showCurrentBranch: {out: []byte("fix/PROJ-412\u202etxt.exe\n")}},
		"its upstream": {readUpstream: {out: []byte("origin/fix/PROJ\u200b-412\n")}},
		"its base":     {originsHead: {out: []byte("origin/ma\u00adin\n")}},
	}

	for name, replies := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The log is asked of whatever base is read, so it answers for any.
			answers := with(featureBranch(), replies)
			answers[logCommits+"origin/ma\u00adin..HEAD"] = reply{}

			// Act
			branch, err := gitrepo.At(fakeRunner(t, answers), workDir).ReadBranch(t.Context())

			// Assert
			if !errors.Is(err, gitrepo.ErrUnshowableName) || branch.Name != "" {
				t.Fatalf("ReadBranch = %+v, %v; want %v and no branch", branch, err, gitrepo.ErrUnshowableName)
			}

			if strings.ContainsAny(err.Error(), "\u202e\u200b\u00ad") {
				t.Errorf("the error carries the name as it was: %q", err)
			}
		})
	}
}
