// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package codeowners_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/codeowners"
)

// gitHubDocsExample is the example file in GitHub's "About code owners".
const gitHubDocsExample = `# These owners will be the default owners for everything in
# the repo. Unless a later match takes precedence,
# @global-owner1 and @global-owner2 will be requested for
# review when someone opens a pull request.
*       @global-owner1 @global-owner2

# Order is important; the last matching pattern takes the most
# precedence.
*.js    @js-owner #This is an inline comment.

# You can also use email addresses if you prefer.
*.go docs@example.com

# Teams can be specified as code owners as well.
*.txt @octo-org/octocats

/build/logs/ @doctocat

docs/* docs@example.com

apps/ @octocat

/docs/ @doctocat

/scripts/ @doctocat @octocat

**/logs @octocat

/apps/ @octocat
/apps/github @doctocat
`

// gitLabDocsExample is the example file in GitLab's "Code Owners syntax".
const gitLabDocsExample = `# Specify a default Code Owner by using a wildcard:
* @default-codeowner

# Specify multiple Code Owners by using a tab or space:
* @multiple @code @owners

# Rules defined later in the file take precedence over the rules
# defined before.
*.rb @ruby-owner

# Files with a ` + "`#`" + ` can still be accessed by escaping the pound sign:
\#file_with_pound.rb @owner-file-with-pound

CODEOWNERS @multiple @code @owners

LICENSE @legal this_does_not_match janedoe@gitlab.com

README @group @group/with-nested/subgroup

/docs/ @all-docs

/docs/* @root-docs

/docs/**/*.md @root-docs

lib/ @lib-owner

/config/ @config-owner

# Code Owners section:
[Documentation]
ee/docs    @docs
docs       @docs

[Development] @dev-team
*
README.md @docs-team
data-models/ @data-science-team

# This section is combined with the previously defined [Documentation] section:
[DOCUMENTATION]
README.md  @docs
`

// gitLabExclusionsExample is the example in GitLab's "Exclude files from Code
// Owners".
const gitLabExclusionsExample = `* @username
!pom.xml

[Ruby]
*.rb @ruby-team
!/config/**/*.rb
`

// markdownFile is a Markdown file at the repository root.
const markdownFile = "a.md"

type ownersCase struct {
	name    string
	dialect codeowners.Dialect
	content string
	paths   []string
	want    codeowners.Owners
}

func users(names ...string) codeowners.Owners {
	return codeowners.Owners{Users: names}
}

func TestOwnersOfGitHubsDocumentedExample(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "everything else", paths: []string{"README.md"}, want: users("global-owner1", "global-owner2")},
		{name: "an inline comment ends the owners", paths: []string{"src/app.js"}, want: users("js-owner")},
		{name: "an email owner is dropped", paths: []string{"main.go"}, want: codeowners.Owners{}},
		{name: "a team", paths: []string{"notes.txt"}, want: codeowners.Owners{Teams: []string{"octo-org/octocats"}}},
		{name: "a logs directory anywhere", paths: []string{"deeply/nested/logs/x.log"}, want: users("octocat")},
		{name: "an anchored directory", paths: []string{"docs/build-app/troubleshooting.md"}, want: users("doctocat")},
		{name: "an apps directory anywhere", paths: []string{"web/apps/main.js"}, want: users("octocat")},
		{name: "two owners", paths: []string{"scripts/deploy.sh"}, want: users("doctocat", "octocat")},
		{name: "the last match wins", paths: []string{"apps/github/a.rb"}, want: users("doctocat")},
	}

	for _, test := range cases {
		test.dialect, test.content = codeowners.GitHub, gitHubDocsExample

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			file := codeowners.Parse(test.content, test.dialect)

			// Act
			owners := file.OwnersOf(test.paths)

			// Assert
			if !equalOwners(owners, test.want) {
				t.Errorf("OwnersOf(%q) = %+v, want %+v", test.paths, owners, test.want)
			}
		})
	}
}

func TestOwnersOfGitLabsDocumentedExample(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "every section applies", paths: []string{"app/models/user.rb"}, want: users("ruby-owner", "dev-team")},
		{name: "an escaped pound", paths: []string{"#file_with_pound.rb"}, want: users("owner-file-with-pound", "dev-team")},
		{name: "only handles own", paths: []string{"LICENSE"}, want: users("legal", "dev-team")},
		{
			name: "a nested group is a team", paths: []string{"README"},
			want: codeowners.Owners{Users: []string{"group", "dev-team"}, Teams: []string{"group/with-nested/subgroup"}},
		},
		{name: "direct children", paths: []string{"docs/index.md"}, want: users("root-docs", "docs", "dev-team")},
		{
			name: "a globstar under a directory", paths: []string{"docs/projects/index.md"},
			want: users("root-docs", "docs", "dev-team"),
		},
		{
			name: "a nested file in an anchored directory", paths: []string{"docs/projects/a.png"},
			want: users("all-docs", "docs", "dev-team"),
		},
		{name: "a directory anywhere", paths: []string{"src/lib/a.c"}, want: users("lib-owner", "dev-team")},
		{
			name: "a combined section", paths: []string{"README.md"},
			want: users("multiple", "code", "owners", "docs", "docs-team"),
		},
		{
			name: "a section's own rule", paths: []string{"data-models/x.sql"},
			want: users("multiple", "code", "owners", "data-science-team"),
		},
	}

	for _, test := range cases {
		test.dialect, test.content = codeowners.GitLab, gitLabDocsExample

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			file := codeowners.Parse(test.content, test.dialect)

			// Act
			owners := file.OwnersOf(test.paths)

			// Assert
			if !equalOwners(owners, test.want) {
				t.Errorf("OwnersOf(%q) = %+v, want %+v", test.paths, owners, test.want)
			}
		})
	}
}

func TestOwnersOfGitLabsExclusions(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "an excluded file has no owners", paths: []string{"pom.xml"}},
		{name: "an exclusion holds in its section only", paths: []string{"config/a/b.rb"}, want: users("username")},
		{name: "a file no exclusion names", paths: []string{"app/x.rb"}, want: users("username", "ruby-team")},
	}

	for _, test := range cases {
		test.dialect, test.content = codeowners.GitLab, gitLabExclusionsExample

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			file := codeowners.Parse(test.content, test.dialect)

			// Act
			owners := file.OwnersOf(test.paths)

			// Assert
			if !equalOwners(owners, test.want) {
				t.Errorf("OwnersOf(%q) = %+v, want %+v", test.paths, owners, test.want)
			}
		})
	}
}

func TestOwnersOfEachDialectsRules(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{
			name: "GitHub anchors a pattern with a slash", dialect: codeowners.GitHub,
			content: "docs/a.md @x\n", paths: []string{"web/docs/a.md"},
		},
		{
			name: "GitLab matches an unanchored pattern at any depth", dialect: codeowners.GitLab,
			content: "docs/a.md @x\n", paths: []string{"web/docs/a.md"}, want: users("x"),
		},
		{
			name: "dir/* covers only direct children", dialect: codeowners.GitHub,
			content: "docs/* @x\n", paths: []string{"docs/a/b.md"},
		},
		{
			name: "a trailing slash needs a directory", dialect: codeowners.GitHub,
			content: "apps/ @x\n", paths: []string{"apps"},
		},
		{
			name: "a line with no owners clears ownership", dialect: codeowners.GitHub,
			content: "/apps/ @x\n/apps/github\n", paths: []string{"apps/github/a"},
		},
		{
			name: "GitHub skips a negation", dialect: codeowners.GitHub,
			content: "* @a\n!b.txt @b\n", paths: []string{"b.txt"}, want: users("a"),
		},
		{
			name: "an escaped space", dialect: codeowners.GitHub,
			content: `my\ file.txt @x`, paths: []string{"my file.txt"}, want: users("x"),
		},
		{
			name: "a question mark is one character", dialect: codeowners.GitHub,
			content: "a?.go @x\n", paths: []string{"ab.go"}, want: users("x"),
		},
		{
			name: "a question mark is not two", dialect: codeowners.GitHub,
			content: "a?.go @x\n", paths: []string{"abc.go"},
		},
		{
			name: "a section takes its defaults", dialect: codeowners.GitLab,
			content: "^[Optional] @opt\n*.md\n[Two][2] @two @org/team\n*.md\n", paths: []string{markdownFile},
			want: codeowners.Owners{Users: []string{"opt", "two"}, Teams: []string{"org/team"}},
		},
		{
			name: "an email-only line in a section keeps no defaults", dialect: codeowners.GitLab,
			content: "[S] @dev\n*.md a@example.com\n", paths: []string{markdownFile},
		},
		{
			name: "owners repeat once, first seen first", dialect: codeowners.GitHub,
			content: "*.js @js\n*.go @go @JS\n", paths: []string{"a.js", "b.go", "c.js"}, want: users("js", "go"),
		},
		{
			name: "a section header with no name is skipped", dialect: codeowners.GitLab,
			content: "[ ] @x\n*.md\n", paths: []string{markdownFile},
		},
		{
			name: "a pattern of only a slash is skipped", dialect: codeowners.GitHub,
			content: "* @a\n/ @x\n", paths: []string{markdownFile}, want: users("a"),
		},
		{
			name: "a tab separates owners", dialect: codeowners.GitHub,
			content: "*.md\t@x\t@y\n", paths: []string{markdownFile}, want: users("x", "y"),
		},
		{
			name: "a leading slash on a path is ignored", dialect: codeowners.GitHub,
			content: "/a.md @x\n", paths: []string{"/a.md"}, want: users("x"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			file := codeowners.Parse(test.content, test.dialect)

			// Act
			owners := file.OwnersOf(test.paths)

			// Assert
			if !equalOwners(owners, test.want) {
				t.Errorf("OwnersOf(%q) = %+v, want %+v", test.paths, owners, test.want)
			}
		})
	}
}

func TestOwnersOfDropsOwnersOfAnotherShape(t *testing.T) {
	t.Parallel()

	// Arrange
	content := "* @ok @-bad @bad! user@example.com @@developer @org/team @a.b_c-d @org/ @ana~ @ bare\n"
	file := codeowners.Parse(content, codeowners.GitHub)

	// Act
	owners := file.OwnersOf([]string{"x"})

	// Assert
	want := codeowners.Owners{Users: []string{"ok", "a.b_c-d"}, Teams: []string{"org/team"}}
	if !equalOwners(owners, want) {
		t.Errorf("OwnersOf = %+v, want %+v", owners, want)
	}
}

func TestAnEmptyFileOwnsNothing(t *testing.T) {
	t.Parallel()

	// Act
	owners := codeowners.File{}.OwnersOf([]string{"a.go"})

	// Assert
	if len(owners.Users)+len(owners.Teams) != 0 {
		t.Errorf("OwnersOf = %+v, want none", owners)
	}
}

func TestLocations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		dialect codeowners.Dialect
		want    []string
	}{
		{name: "GitHub", dialect: codeowners.GitHub, want: []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}},
		{name: "GitLab", dialect: codeowners.GitLab, want: []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := codeowners.Locations(test.dialect); !slices.Equal(got, test.want) {
				t.Errorf("Locations = %q, want %q", got, test.want)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add(gitHubDocsExample, "apps/github/a.rb")
	f.Add(gitLabDocsExample, "docs/index.md")
	f.Add(gitLabExclusionsExample, "config/a/b.rb")
	f.Add("[a\\\n**/**/**/**/x @a\n\\ @b", "a/b/c/d/x")

	ownerShape := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9._-]+)*$`)

	f.Fuzz(func(t *testing.T, content, path string) {
		// Arrange
		paths := []string{path, "a/" + path}

		// Act
		fromGitHub := codeowners.Parse(content, codeowners.GitHub).OwnersOf(paths)
		fromGitLab := codeowners.Parse(content, codeowners.GitLab).OwnersOf(paths)

		// Assert
		for _, user := range slices.Concat(fromGitHub.Users, fromGitLab.Users) {
			if !ownerShape.MatchString(user) || strings.Contains(user, "/") {
				t.Errorf("user %q is not a username", user)
			}
		}

		for _, team := range slices.Concat(fromGitHub.Teams, fromGitLab.Teams) {
			if !ownerShape.MatchString(team) || !strings.Contains(team, "/") {
				t.Errorf("team %q is not a team", team)
			}
		}
	})
}

func equalOwners(got, want codeowners.Owners) bool {
	return slices.Equal(got.Users, want.Users) && slices.Equal(got.Teams, want.Teams)
}
