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

// Paths and patterns several cases share.
const (
	markdownFile   = "a.md"
	readme         = "README.md"
	docsIndex      = "docs/index.md"
	docsChild      = "docs/a.md"
	docsGrandchild = "docs/a/b.md"
	docsChildren   = "docs/* @x\n"
	nestedFile     = "a/b"
)

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
		{name: "everything else", paths: []string{readme}, want: users("global-owner1", "global-owner2")},
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
			// @group is a top-level group, but CODEOWNERS spells it as it spells a
			// user, so it is read as a name the forge resolves to either.
			name: "a nested group is a team, a bare name a user or group", paths: []string{"README"},
			want: codeowners.Owners{Users: []string{"group", "dev-team"}, Teams: []string{"group/with-nested/subgroup"}},
		},
		// The [Documentation] section's "docs" names a file called docs, not a
		// directory: GitLab expands only a pattern ending in a slash, so these
		// paths take no owner from that section.
		{name: "direct children", paths: []string{docsIndex}, want: users("root-docs", "dev-team")},
		{
			name: "a globstar under a directory", paths: []string{"docs/projects/index.md"},
			want: users("root-docs", "dev-team"),
		},
		{
			name: "a nested file in an anchored directory", paths: []string{"docs/projects/a.png"},
			want: users("all-docs", "dev-team"),
		},
		{name: "a directory anywhere", paths: []string{"src/lib/a.c"}, want: users("lib-owner", "dev-team")},
		{
			name: "a combined section", paths: []string{readme},
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

// gitLabRegularEntriesExample is the example in GitLab's "Regular entries and
// sections".
const gitLabRegularEntriesExample = `# Required for all files
* @general-approvers

[Documentation] @docs-team
docs/
README.md
*.txt

[Database] @database-team
model/db/
config/db/database-setup.md @docs-team
`

func TestOwnersOfGitLabsAdvancedExamples(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{
			name: "every section's owners", content: gitLabRegularEntriesExample, paths: []string{"model/db/CHANGELOG.txt"},
			want: users("general-approvers", "docs-team", "database-team"),
		},
		{
			name: "an override in a section", content: gitLabRegularEntriesExample,
			paths: []string{"config/db/database-setup.md"}, want: users("general-approvers", "docs-team"),
		},
		{
			name: "the last matching pattern", content: "*.md @doc-team\nterms.md @legal-team\n",
			paths: []string{"terms.md"}, want: users("legal-team"),
		},
		{
			name: "an unparsable header joins the section before", content: "* @group\n\n[Section name\ndocs/ @docs_group\n",
			paths: []string{docsChild}, want: users("docs_group"),
		},
		{
			name: "a malformed owner is ignored", content: "/path/* @group user_without_at_symbol @user_with_at_symbol\n",
			paths: []string{"path/a"}, want: users("group", "user_with_at_symbol"),
		},
		{
			name: "escaped spaces", content: `path\ with\ spaces/*.md @owner`,
			paths: []string{"path with spaces/a.md"}, want: users("owner"),
		},
	}

	for _, test := range cases {
		test.dialect = codeowners.GitLab

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

// TestOwnersOfGitLabPaths pins GitLab's own reading of a pattern
// (ee/lib/gitlab/code_owners/file.rb normalize_pattern, matched by
// pattern_index.rb with File.fnmatch? and FNM_DOTMATCH | FNM_PATHNAME): only a
// trailing slash covers a directory's contents.
func TestOwnersOfGitLabPaths(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "an anchored name is not its directory", content: "/docs @x\n", paths: []string{docsIndex}},
		{name: "an anchored name is a file", content: "/docs @x\n", paths: []string{"docs"}, want: users("x")},
		{name: "an unanchored name is not its directory", content: "docs @x\n", paths: []string{docsIndex}},
		{name: "an unanchored name is a file anywhere", content: "docs @x\n", paths: []string{"a/docs"}, want: users("x")},
		{name: "a trailing slash covers contents", content: "/docs/ @x\n", paths: []string{docsIndex}, want: users("x")},
		{name: "a trailing slash covers depth", content: "/docs/ @x\n", paths: []string{docsGrandchild}, want: users("x")},
		{name: "a trailing slash is not a file", content: "docs/ @x\n", paths: []string{"docs"}},
		{name: "an unanchored directory anywhere", content: "docs/ @x\n", paths: []string{"x/docs/a"}, want: users("x")},
		{name: "a direct child", content: docsChildren, paths: []string{docsChild}, want: users("x")},
		{name: "not a grandchild", content: docsChildren, paths: []string{docsGrandchild}},
		{name: "a direct child anywhere", content: docsChildren, paths: []string{"x/docs/a.md"}, want: users("x")},
		{name: "a trailing globstar is one segment", content: "/docs/** @x\n", paths: []string{docsGrandchild}},
		{name: "a globstar matches none", content: "/docs/**/*.md @x\n", paths: []string{docsChild}, want: users("x")},
		{name: "a file name at the root", content: "README.md @x\n", paths: []string{readme}, want: users("x")},
		{name: "a file name at any depth", content: "README.md @x\n", paths: []string{"a/b/README.md"}, want: users("x")},
		{name: "a star is everything", content: "* @x\n", paths: []string{"a/b/c.go"}, want: users("x")},
		{name: "a star is a dot file", content: "* @x\n", paths: []string{"a/b/.env"}, want: users("x")},
		{name: "a glob under a dot directory", content: "*.md @x\n", paths: []string{"a/.hidden/b.md"}, want: users("x")},
		{name: "a question mark is not a slash", content: "a? @x\n", paths: []string{nestedFile}},
		{name: "a bang negates a class", content: "x[!a].md @x\n", paths: []string{"xb.md"}, want: users("x")},
		// fnmatch compares a doubled slash's empty segment with a path's, and a
		// path has none.
		{name: "a doubled slash matches nothing", content: "a//b @x\n", paths: []string{nestedFile}},
		{name: "a doubled slash after a globstar", content: "**//x @x\n", paths: []string{"a/x"}},
		{name: "a doubled trailing slash", content: "docs// @x\n", paths: []string{docsChild}},
		{name: "a trailing dash is literal", content: "x[a-] @x\n", paths: []string{"x-"}, want: users("x")},
		{name: "an empty negated class is any", content: "x[!] @x\n", paths: []string{"xq"}, want: users("x")},
		{name: "a caret negates a class", content: "x[^] @x\n", paths: []string{"xq"}, want: users("x")},
		{name: "a range from a dash", content: "a[--z] @x\n", paths: []string{"aq"}, want: users("x")},
		{name: "a reversed range holds its ends", content: "x[z-a] @x\n", paths: []string{"xz"}, want: users("x")},
		{name: "a reversed range holds nothing between", content: "x[z-a] @x\n", paths: []string{"xm"}},
		{name: "a trailing backslash ends the pattern", content: "[S] @x\na\\\n", paths: []string{"a"}, want: users("x")},
		{name: "a class spans a slash", content: "a[b/c]d @x\n", paths: []string{"acd"}, want: users("x")},
		{name: "a class spanning a slash is one character", content: "a[b/c]d @x\n", paths: []string{"a/cd"}},
		{name: "an unterminated class matches nothing", content: "a[ @x\n", paths: []string{"a["}},
		{name: "an escaped slash separates", content: "a\\/b @x\n", paths: []string{nestedFile}, want: users("x")},
		{
			name: "a repeated pattern keeps the later line", content: "!*.md\n*.md @x\n",
			paths: []string{markdownFile}, want: users("x"),
		},
		{
			name: "an exclusion matches in any order", content: "*.md @x\n!/a.md\n*.md @y\n",
			paths: []string{markdownFile},
		},
	}

	for _, test := range cases {
		test.dialect = codeowners.GitLab

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

// TestOwnersOfGitLabSectionHeaders pins which lines GitLab reads as a section
// header (ee/lib/gitlab/code_owners/section_parser.rb HEADER_REGEX): any line
// starting [ or ^[ with a ] after it, its default owners the run of @, word
// characters, '.', '-', '/' and whitespace that follows.
func TestOwnersOfGitLabSectionHeaders(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{
			// A pattern starting with a class is a header to GitLab: section Dd,
			// its default owners "ocs/ @team".
			name: "a pattern starting with a class is a header", content: "[Dd]ocs/ @team\n*.md\n",
			paths: []string{markdownFile}, want: users("team"),
		},
		{
			name: "an escaped bracket starts a pattern", content: "\\[Dd]ocs/ @x\n",
			paths: []string{"[Dd]ocs/a"}, want: users("x"),
		},
		{name: "a blank name is a section", content: "[ ] @x\n*.md\n", paths: []string{markdownFile}, want: users("x")},
		{name: "an empty name is a section", content: "[] @x\n*.md\n", paths: []string{markdownFile}, want: users("x")},
		{
			name: "approvals then defaults", content: "[Docs][2] @a\n*.md\n",
			paths: []string{markdownFile}, want: users("a"),
		},
		{name: "approvals not a number end the header", content: "[Docs][x] @a\n*.md\n", paths: []string{markdownFile}},
		{name: "approvals after a space end the header", content: "[Docs] [2] @a\n*.md\n", paths: []string{markdownFile}},
		{
			name: "a comma ends the defaults", content: "[Docs] @a, @b\n*.md\n",
			paths: []string{markdownFile}, want: users("a"),
		},
		{
			name: "a pound ends the defaults", content: "[Docs] @a #@b\n*.md\n",
			paths: []string{markdownFile}, want: users("a"),
		},
	}

	for _, test := range cases {
		test.dialect = codeowners.GitLab

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

// TestOwnersOfGitLabsDefaultSection pins GitLab's top-level section, named
// codeowners (Section::DEFAULT), and how a header finds a section already
// named (SectionParser#find_section_name): by exact name while no other
// section exists, and without case after.
func TestOwnersOfGitLabsDefaultSection(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "its name joins the top level", content: "* @a\n[codeowners]\n* @b\n", paths: []string{"x"}, want: users("b")},
		{
			name: "a first header matches it only exactly", content: "* @a\n[CODEOWNERS]\n* @b\n",
			paths: []string{"x"}, want: users("a", "b"),
		},
		{
			name: "a later header matches it without case", content: "* @a\n[D]\n* @d\n[CODEOWNERS]\n* @b\n",
			paths: []string{"x"}, want: users("b", "d"),
		},
		{
			name: "an empty name is not it", content: "* @a\n[] @x\n*.md\n",
			paths: []string{markdownFile}, want: users("a", "x"),
		},
	}

	for _, test := range cases {
		test.dialect = codeowners.GitLab

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

// TestOwnersOfGitLabOwners pins which owners GitLab reads off a line: every
// @name its ReferenceExtractor finds in the text after the pattern, wherever
// it stands, and the section's defaults only when there is no such text.
func TestOwnersOfGitLabOwners(t *testing.T) {
	t.Parallel()

	cases := []ownersCase{
		{name: "a pound starts no comment", content: "*.md @a # @b\n", paths: []string{markdownFile}, want: users("a", "b")},
		{
			name: "text that names nobody is no defaults", content: "[S] @d\ndocs/ # nobody\n",
			paths: []string{docsChild},
		},
		{name: "a comma separates", content: "*.md @a,@b\n", paths: []string{markdownFile}, want: users("a", "b")},
		{
			name: "a name may start with an underscore or a dot", content: "*.md @_e @.f @g/_h\n", paths: []string{markdownFile},
			want: codeowners.Owners{Users: []string{"_e", ".f"}, Teams: []string{"g/_h"}},
		},
		{
			name: "a name ends where GitLab's path does", content: "*.md @a/b/ @c- @d.\n", paths: []string{markdownFile},
			want: codeowners.Owners{Users: []string{"c-", "d"}, Teams: []string{"a/b"}},
		},
		{name: "an @ after a word is no owner", content: "*.md a@b @c@d\n", paths: []string{markdownFile}, want: users("c")},
	}

	for _, test := range cases {
		test.dialect = codeowners.GitLab

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
			content: docsChildren, paths: []string{docsGrandchild},
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

	// GitLab's names may also start with '_' or '.', as its namespace paths do.
	shapes := map[codeowners.Dialect]*regexp.Regexp{
		codeowners.GitHub: regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9._-]+)*$`),
		codeowners.GitLab: regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9._-]*(/[A-Za-z0-9_.][A-Za-z0-9._-]*)*$`),
	}

	f.Fuzz(func(t *testing.T, content, path string) {
		// Arrange
		paths := []string{path, "a/" + path}

		// Act
		found := map[codeowners.Dialect]codeowners.Owners{}
		for dialect := range shapes {
			found[dialect] = codeowners.Parse(content, dialect).OwnersOf(paths)
		}

		// Assert
		for dialect, owners := range found {
			for _, user := range owners.Users {
				if !shapes[dialect].MatchString(user) || strings.Contains(user, "/") {
					t.Errorf("user %q is not a username", user)
				}
			}

			for _, team := range owners.Teams {
				if !shapes[dialect].MatchString(team) || !strings.Contains(team, "/") {
					t.Errorf("team %q is not a team", team)
				}
			}
		}
	})
}

func equalOwners(got, want codeowners.Owners) bool {
	return slices.Equal(got.Users, want.Users) && slices.Equal(got.Teams, want.Teams)
}
