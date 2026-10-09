// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// A pull request's description starts from the repository's own template,
// where its forge looks for one, read only from inside the repository.

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTemplatesAreReadFromTheRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	err := os.MkdirAll(filepath.Join(root, ".github"), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, ".github", "PULL_REQUEST_TEMPLATE.md"), "## What\n", 0o600)

	cases := map[string]struct {
		remote string
		kind   string
		want   []string
	}{
		"a github repository's":          {remote: githubRemote, kind: "", want: []string{"## What\n"}},
		"none without a remote":          {remote: "", kind: "", want: nil},
		"none on a host naming no forge": {remote: unnamedHost, kind: "bogus", want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Forge.Kind = tt.kind

			seams := wired(t, cfg, wiring.Workspace{Root: root, Remote: tt.remote}, nil).Forge

			// Act
			found := seams.Templates()

			// Assert
			bodies := make([]string, 0, len(found))
			for _, template := range found {
				bodies = append(bodies, template.Body)
			}

			if !slices.Equal(bodies, tt.want) {
				t.Errorf("Templates = %q, want %q", bodies, tt.want)
			}
		})
	}
}

func TestTemplatesAreReadOnlyFromInsideTheRepository(t *testing.T) {
	t.Parallel()

	// A cloned repository can link where its forge looks for a template to a
	// file of the user's, which would fill in a description they may post.
	elsewhere := t.TempDir()
	write(t, filepath.Join(elsewhere, "Default.md"), "a key of the user's\n", 0o600)

	cases := map[string]struct {
		remote string
		// inside is a template the repository holds, which is read.
		inside string
		// link is where the repository links to target, outside it.
		link, target string
	}{
		"a github template linked to a file outside": {
			remote: githubRemote, inside: "docs/pull_request_template.md",
			link: ".github/PULL_REQUEST_TEMPLATE.md", target: filepath.Join(elsewhere, "Default.md"),
		},
		"a github folder linked to one outside": {
			remote: githubRemote, inside: "docs/pull_request_template.md",
			link: ".github/PULL_REQUEST_TEMPLATE", target: elsewhere,
		},
		"a gitlab template linked to a file outside": {
			remote: remoteGitLab, inside: ".gitlab/merge_request_templates/Real.md",
			link: ".gitlab/merge_request_templates/Leak.md", target: filepath.Join(elsewhere, "Default.md"),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := t.TempDir()
			inside, link := filepath.Join(root, filepath.FromSlash(tt.inside)), filepath.Join(root, filepath.FromSlash(tt.link))

			for _, dir := range []string{filepath.Dir(inside), filepath.Dir(link)} {
				err := os.MkdirAll(dir, 0o750)
				if err != nil {
					t.Fatal(err)
				}
			}

			write(t, inside, "## What\n", 0o600)

			err := os.Symlink(tt.target, link)
			if err != nil {
				t.Fatal(err)
			}

			seams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: tt.remote}, nil).Forge

			// Act
			found := seams.Templates()

			// Assert
			paths := make([]string, 0, len(found))
			for _, template := range found {
				paths = append(paths, template.Path)
			}

			if want := []string{tt.inside}; !slices.Equal(paths, want) {
				t.Errorf("Templates = %+v, want %q alone, the one inside the repository", found, want)
			}
		})
	}
}

// jiraToken is the token each Jira seam test configures, so the client sends
// its requests rather than refusing for want of one.
const jiraToken = "a-token-for-tests"

// jiraAddress is a Jira base URL that no test ever reaches.
const jiraAddress = "https://jira.example.com"

func TestNoTemplatesAreReadFromARepositoryNoLongerThere(t *testing.T) {
	t.Parallel()

	// Arrange
	// The session's repository was removed while it ran, so there is no
	// directory left to read a template from.
	root := filepath.Join(t.TempDir(), "removed")
	seams := wired(t, config.Default(), wiring.Workspace{Root: root, Remote: githubRemote}, nil).Forge

	// Act
	found := seams.Templates()

	// Assert
	if len(found) != 0 {
		t.Errorf("Templates = %+v, want none from a repository that is not there", found)
	}
}
