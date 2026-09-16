// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubDefault is where the single GitHub template in these fixtures lives.
const githubDefault = ".github/PULL_REQUEST_TEMPLATE.md"

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

	repo := fstest.MapFS{
		githubDefault: file("## What this changes\n"),
		".github/PULL_REQUEST_TEMPLATE/feature.md":   file("## Feature\n"),
		".github/PULL_REQUEST_TEMPLATE/bugfix.md":    file("## Bug\n"),
		".github/PULL_REQUEST_TEMPLATE/notes.json":   file("{}"),
		"docs/pull_request_template.md":              file("## Docs\n"),
		"README.md":                                  file("not a template"),
		".gitlab/merge_request_templates/Default.md": file("gitlab only"),
	}

	found := forge.FindTemplates(repo, forge.KindGitHub)

	// The single template is the default and comes first; a folder of them
	// follows, by name. Case does not matter to GitHub, so not here either.
	want := []string{"PULL_REQUEST_TEMPLATE", "pull_request_template", "bugfix", "feature"}
	if got := templateNames(found); !slices.Equal(got, want) {
		t.Errorf("FindTemplates = %q, want %q", got, want)
	}

	if found[0].Body != "## What this changes\n" || found[0].Path != githubDefault {
		t.Errorf("first template = %+v", found[0])
	}
}

func TestGitHubReadsARootTemplateAndTxtFiles(t *testing.T) {
	t.Parallel()

	repo := fstest.MapFS{"pull_request_template.txt": file("plain\n")}

	got := templateNames(forge.FindTemplates(repo, forge.KindGitHub))
	if !slices.Equal(got, []string{"pull_request_template"}) {
		t.Errorf("FindTemplates = %q, want the root template", got)
	}
}

func TestGitLabTemplatesPutDefaultFirst(t *testing.T) {
	t.Parallel()

	repo := fstest.MapFS{
		".gitlab/merge_request_templates/Bug.md":     file("bug"),
		".gitlab/merge_request_templates/Default.md": file("default"),
		".gitlab/merge_request_templates/Feature.md": file("feature"),
		githubDefault: file("github only"),
	}

	// GitLab applies the template named Default on its own, so it is the one
	// to start from.
	want := []string{"Default", "Bug", "Feature"}
	if got := templateNames(forge.FindTemplates(repo, forge.KindGitLab)); !slices.Equal(got, want) {
		t.Errorf("FindTemplates = %q, want %q", got, want)
	}
}

func TestARepositoryWithoutTemplatesHasNone(t *testing.T) {
	t.Parallel()

	for _, kind := range []forge.Kind{forge.KindGitHub, forge.KindGitLab, forge.KindUnknown} {
		if got := forge.FindTemplates(fstest.MapFS{"README.md": file("hi")}, kind); len(got) != 0 {
			t.Errorf("FindTemplates(%v) = %+v, want none", kind, got)
		}
	}
}
