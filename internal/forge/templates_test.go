// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubDefault is where the single GitHub template in these fixtures lives,
// and githubFeature one of a folder of them, named featureName.
const (
	githubDefault = ".github/PULL_REQUEST_TEMPLATE.md"
	githubFeature = ".github/PULL_REQUEST_TEMPLATE/feature.md"
	featureName   = "feature"
)

// file is a fixture file with the given contents.
func file(contents string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(contents)}
}

// templateNames lists the names found, in order.
func templateNames(templates []forge.Template) []string {
	names := make([]string, 0, len(templates))
	for _, each := range templates {
		names = append(names, each.Name)
	}

	return names
}

func TestGitHubTemplatesAreFoundWhereverGitHubLooks(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := fstest.MapFS{
		githubDefault: file("## What this changes\n"),
		githubFeature: file("## Feature\n"),
		".github/PULL_REQUEST_TEMPLATE/bugfix.md":    file("## Bug\n"),
		".github/PULL_REQUEST_TEMPLATE/notes.json":   file("{}"),
		"docs/pull_request_template.md":              file("## Docs\n"),
		"README.md":                                  file("not a template"),
		".gitlab/merge_request_templates/Default.md": file("gitlab only"),
	}

	// Act
	found := forge.FindTemplates(repo, forge.KindGitHub)

	// Assert
	// The single template is the default and comes first; a folder of them
	// follows, by name. Case does not matter to GitHub, so not here either.
	want := []string{"PULL_REQUEST_TEMPLATE", "pull_request_template", "bugfix", featureName}
	if got := templateNames(found); !slices.Equal(got, want) {
		t.Fatalf("FindTemplates = %q, want %q", got, want)
	}

	if found[0].Body != "## What this changes\n" || found[0].Path != githubDefault {
		t.Errorf("first template = %+v", found[0])
	}
}

func TestTemplatesAreFoundWhereEachForgeLooks(t *testing.T) {
	t.Parallel()

	gitlabTemplates := fstest.MapFS{
		".gitlab/merge_request_templates/Bug.md":     file("bug"),
		".gitlab/merge_request_templates/Default.md": file("default"),
		".gitlab/merge_request_templates/Feature.md": file("feature"),
		githubDefault: file("github only"),
	}
	none := fstest.MapFS{"README.md": file("hi")}

	cases := map[string]struct {
		repo fstest.MapFS
		kind forge.Kind
		want []string
	}{
		"a github template at the root, as .txt": {
			repo: fstest.MapFS{"pull_request_template.txt": file("plain\n")},
			kind: forge.KindGitHub,
			want: []string{"pull_request_template"},
		},
		// GitLab applies the template named Default on its own, so it is the one
		// to start from.
		"gitlab's, with Default first": {
			repo: gitlabTemplates, kind: forge.KindGitLab, want: []string{"Default", "Bug", "Feature"},
		},
		"none for github":         {repo: none, kind: forge.KindGitHub, want: nil},
		"none for gitlab":         {repo: none, kind: forge.KindGitLab, want: nil},
		"none for an unknown one": {repo: none, kind: forge.KindUnknown, want: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := templateNames(forge.FindTemplates(tt.repo, tt.kind))

			// Assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("FindTemplates = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestATemplateThatCannotBeReadIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// A link to a file the repository does not hold is listed like a template
	// but cannot be read; the one beside it can.
	repo := fstest.MapFS{
		githubFeature:                           file("## Feature\n"),
		".github/PULL_REQUEST_TEMPLATE/gone.md": {Data: []byte("nowhere.md"), Mode: fs.ModeSymlink},
	}

	// Act
	found := forge.FindTemplates(repo, forge.KindGitHub)

	// Assert
	if got, want := templateNames(found), []string{featureName}; !slices.Equal(got, want) {
		t.Errorf("FindTemplates = %q, want %q: the unreadable one left out, the other kept", got, want)
	}
}

// breakingRepo is a repository whose file at broken opens, then breaks off
// when it is read, as one on a failing disk or a dropped network share does.
type breakingRepo struct {
	fstest.MapFS

	broken string
}

func (r breakingRepo) Open(name string) (fs.File, error) {
	opened, err := r.MapFS.Open(name)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", name, err)
	}

	if name == r.broken {
		return breakingFile{File: opened}, nil
	}

	return opened, nil
}

// breakingFile is a file whose every read fails.
type breakingFile struct {
	fs.File
}

func (breakingFile) Read([]byte) (int, error) { return 0, errBrokeOff }

func TestATemplateThatBreaksOffWhileReadIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := breakingRepo{MapFS: fstest.MapFS{
		githubFeature: file("## Feature\n"),
		".github/PULL_REQUEST_TEMPLATE/broken.md": file("## Half of it\n"),
	}, broken: ".github/PULL_REQUEST_TEMPLATE/broken.md"}

	// Act
	found := forge.FindTemplates(repo, forge.KindGitHub)

	// Assert
	if got, want := templateNames(found), []string{featureName}; !slices.Equal(got, want) {
		t.Errorf("FindTemplates = %q, want %q: the one that broke off left out, the other kept", got, want)
	}
}

func TestATemplateLargerThanAnyDescriptionIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitHub takes a description of 65,536 characters at most; a file past
	// that is no template, and is not read whole to find out.
	repo := fstest.MapFS{
		".github/PULL_REQUEST_TEMPLATE/huge.md":  file(strings.Repeat("x", 64<<10+1)),
		".github/PULL_REQUEST_TEMPLATE/large.md": file(strings.Repeat("x", 64<<10)),
	}

	// Act
	found := forge.FindTemplates(repo, forge.KindGitHub)

	// Assert
	if got, want := templateNames(found), []string{"large"}; !slices.Equal(got, want) {
		t.Errorf("FindTemplates = %q, want %q: the one past the limit left out", got, want)
	}
}
