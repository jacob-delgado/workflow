// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Template is a pull request template a repository carries.
type Template struct {
	// Name is the file's name without its extension.
	Name string
	// Path is where the file is, relative to the repository root.
	Path string
	Body string
}

// githubTemplateName is the name GitHub looks for, as a file or as a directory
// of files, in any case.
const githubTemplateName = "pull_request_template"

// FindTemplates finds a repository's pull request templates where its forge
// looks for them, the default first.
//
// GitHub reads a single pull_request_template file from .github, the root or
// docs, and a directory of them by that name in the same places; the single
// file is the default. GitLab reads .gitlab/merge_request_templates, and applies
// the one named Default by itself.
func FindTemplates(repo fs.FS, kind Kind) []Template {
	//nolint:exhaustive // KindUnknown finds no templates, which the nil lookup miss already gives.
	finders := map[Kind]func(fs.FS) []Template{KindGitHub: githubTemplates, KindGitLab: gitlabTemplates}

	find, known := finders[kind]
	if !known {
		return nil
	}

	return find(repo)
}

// githubTemplates reads GitHub's templates: single files, then directories.
func githubTemplates(repo fs.FS) []Template {
	places := []string{".github", ".", "docs"}

	var singles, grouped []Template

	for _, place := range places {
		entries, err := fs.ReadDir(repo, place)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := strings.ToLower(entry.Name())

			switch {
			case entry.IsDir() && name == githubTemplateName:
				grouped = append(grouped, readTemplates(repo, path.Join(place, entry.Name()))...)
			case !entry.IsDir() && templateFile(name) && strings.TrimSuffix(name, path.Ext(name)) == githubTemplateName:
				singles = append(singles, readTemplate(repo, path.Join(place, entry.Name()))...)
			}
		}
	}

	return append(singles, grouped...)
}

// gitlabTemplates reads GitLab's templates, Default first.
func gitlabTemplates(repo fs.FS) []Template {
	templates := readTemplates(repo, ".gitlab/merge_request_templates")

	slices.SortStableFunc(templates, func(left, right Template) int {
		return defaultRank(left) - defaultRank(right)
	})

	return templates
}

// defaultRank puts the template GitLab applies by itself ahead of the rest.
func defaultRank(template Template) int {
	if strings.EqualFold(template.Name, "default") {
		return 0
	}

	return 1
}

// readTemplates reads every template file in a directory, by name.
func readTemplates(repo fs.FS, dir string) []Template {
	entries, err := fs.ReadDir(repo, dir)
	if err != nil {
		return nil
	}

	var templates []Template

	for _, entry := range entries {
		if !entry.IsDir() && templateFile(strings.ToLower(entry.Name())) {
			templates = append(templates, readTemplate(repo, path.Join(dir, entry.Name()))...)
		}
	}

	return templates
}

// readTemplate reads one template, or nothing if it cannot be read. A template
// file is anyone's commit, so what it says is neutralized before it is shown.
func readTemplate(repo fs.FS, file string) []Template {
	body, err := fs.ReadFile(repo, file)
	if err != nil {
		return nil
	}

	base := path.Base(file)

	return []Template{{Name: strings.TrimSuffix(base, path.Ext(base)), Path: file, Body: sanitize.Text(string(body))}}
}

// templateFile reports a file name a forge reads as a template.
func templateFile(lowerName string) bool {
	return strings.HasSuffix(lowerName, ".md") || strings.HasSuffix(lowerName, ".txt")
}
