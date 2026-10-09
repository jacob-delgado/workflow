// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// A repository's CODEOWNERS is read in the dialect of the forge its remote
// is on, which decides among other things where the file is looked for.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestCodeOwnersAreLookedForWhereTheRemotesForgeLooks(t *testing.T) {
	// Only GitLab looks in .gitlab/, so a file there is found on GitLab
	// alone.
	cases := map[string]struct {
		remote string
		found  bool
	}{
		"on GitLab": {remote: remoteGitLab, found: true},
		"on GitHub": {remote: remoteGitHub, found: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			isolateGit(t)

			root := repository(t)

			err := os.Mkdir(filepath.Join(root, ".gitlab"), 0o750)
			if err != nil {
				t.Fatal(err)
			}

			write(t, filepath.Join(root, ".gitlab", "CODEOWNERS"), "* @acme/reviewers\n", 0o600)
			git(t, root, "add", ".gitlab/CODEOWNERS")
			git(t, root, "commit", "--quiet", "-m", "chore: name the owners")

			gitSeams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: tt.remote, Repository: true}, nil).Git

			// Act
			file, found, err := gitSeams.CodeOwnersAt("main")

			// Assert
			owners := file.OwnersOf([]string{"README.md"})
			if err != nil || found != tt.found || (len(owners.Teams) == 1) != tt.found {
				t.Errorf("CodeOwnersAt(main) = %+v, %t, %v; want found %t", owners, found, err, tt.found)
			}
		})
	}
}
