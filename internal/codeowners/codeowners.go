// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package codeowners reads a CODEOWNERS file the way GitHub or GitLab reads it
// and answers who owns a set of paths. It is pure: the caller reads the file.
//
// Only owners a forge can be asked to review as are kept: a @username, or a
// team or group as @org/team, read on GitLab as GitLab's ReferenceExtractor
// reads them. Email owners, GitLab's @@role owners and any other shape are
// dropped, since nothing here can turn them into a reviewer.
package codeowners

import (
	"regexp"
	"slices"
	"strings"
)

// Dialect is which forge's CODEOWNERS rules a file is read by.
type Dialect int

const (
	// GitHub has one list of rules, the last match wins, and a pattern holding a
	// slash is anchored to the repository root.
	GitHub Dialect = iota
	// GitLab adds sections, each with its own last match and default owners,
	// and !pattern exclusions; a pattern repeated in a section keeps only its
	// later line. The lines before the first header are the section named
	// codeowners. A pattern matches the whole path, as GitLab's fnmatch does:
	// an unanchored one at any depth, and only one ending in a slash covers a
	// directory's contents, so docs and /docs name a file and docs/* a
	// directory's direct children.
	GitLab
)

// Locations are where dialect looks for the file, in the order it looks: the
// first one present is the file.
func Locations(dialect Dialect) []string {
	return map[Dialect][]string{
		GitHub: {".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"},
		GitLab: {"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"},
	}[dialect]
}

// Owners are the people and the teams that own some paths, each listed once in
// the order first seen. A team is org/team (GitLab: group/subgroup). On GitLab
// a top-level group is written @group, just as a user is, so a bare name is in
// Users and the forge resolves it to a user or, failing that, a group.
type Owners struct {
	Users []string
	Teams []string
}

// File is a parsed CODEOWNERS file. The zero File owns nothing.
type File struct {
	sections []section
}

// section is one GitLab section, or the whole of a GitHub file: its rules, of
// which the last that matches a path names its owners.
type section struct {
	name  string
	rules []rule
}

// rule is one line: a pattern and who owns what it matches, or an exclusion
// that leaves a path out of its section altogether.
type rule struct {
	pattern pattern
	exclude bool
	owners  Owners
}

// OwnersOf is who owns any of paths: per path, every section's last matching
// rule, unless an exclusion in that section matches it.
func (f File) OwnersOf(paths []string) Owners {
	var found collector

	for _, path := range paths {
		matching := newTarget(path)

		for _, part := range f.sections {
			found.add(part.ownersOf(matching))
		}
	}

	return found.owners
}

// ownersOf is the owners the section gives a path.
func (s section) ownersOf(path target) Owners {
	var owners Owners

	for _, line := range s.rules {
		if !line.pattern.matches(path) {
			continue
		}

		if line.exclude {
			return Owners{}
		}

		owners = line.owners
	}

	return owners
}

// collector gathers owners once each, in the order first seen. Forges compare
// handles without case, so neither does it.
type collector struct {
	owners Owners
	seen   map[string]bool
}

func (c *collector) add(owners Owners) {
	c.owners.Users = c.addNew(c.owners.Users, owners.Users)
	c.owners.Teams = c.addNew(c.owners.Teams, owners.Teams)
}

func (c *collector) addNew(into, names []string) []string {
	if c.seen == nil {
		c.seen = map[string]bool{}
	}

	for _, name := range names {
		key := strings.ToLower(name)
		if c.seen[key] {
			continue
		}

		c.seen[key] = true

		into = append(into, name)
	}

	return into
}

// Parse reads content as dialect reads it. A line the dialect cannot read is
// skipped, as the forge skips it, rather than failing the whole file.
func Parse(content string, dialect Dialect) File {
	reading := parser{dialect: dialect, file: File{sections: []section{{name: defaultSection, rules: nil}}}}
	if dialect == GitLab {
		reading.header = regexp.MustCompile(gitLabHeader)
	}

	for line := range strings.SplitSeq(content, "\n") {
		reading.read(line)
	}

	return reading.file
}

// parser is where Parse is in the file: the section it is in and that
// section's default owners.
type parser struct {
	dialect  Dialect
	file     File
	current  int
	defaults Owners
	// header is GitLab's section header; nil on GitHub, which has none.
	header *regexp.Regexp
}

func (p *parser) read(line string) {
	if p.dialect == GitLab {
		p.readGitLab(line)

		return
	}

	line = strings.TrimLeft(line, " \t")
	if line == "" || line[0] == '#' {
		return
	}

	if tokens := fields(line); len(tokens) > 0 {
		p.addRule(tokens[0], ownersFrom(tokens[1:]), len(tokens) > 1)
	}
}

// addRule adds a pattern line to the current section: the pattern, its owners,
// and whether the line has text after the pattern. A GitLab line with none
// takes the section's defaults and replaces an earlier line of the same
// pattern; GitHub has no exclusions, so a !pattern line there is skipped.
func (p *parser) addRule(raw string, owners Owners, named bool) {
	exclude := strings.HasPrefix(raw, "!")

	if exclude && p.dialect == GitHub {
		return
	}

	compiled, ok := compile(strings.TrimPrefix(raw, "!"), p.dialect)
	if !ok {
		return
	}

	line := rule{pattern: compiled, exclude: exclude, owners: owners}
	if !named {
		line.owners = p.defaults
	}

	current := &p.file.sections[p.current]
	if p.dialect == GitLab {
		current.rules = slices.DeleteFunc(current.rules, func(earlier rule) bool {
			return earlier.pattern.source == compiled.source
		})
	}

	current.rules = append(current.rules, line)
}

// fields splits a line at whitespace a backslash does not escape, keeping the
// escapes for the pattern matcher, and ends it at an inline comment.
func fields(line string) []string {
	var (
		tokens  []string
		current strings.Builder
		escaped bool
	)

	for _, character := range line {
		switch {
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case character == ' ' || character == '\t' || character == '\r':
			tokens = appendToken(tokens, &current)

			continue
		}

		current.WriteRune(character)
	}

	tokens = appendToken(tokens, &current)

	return withoutComment(tokens)
}

func appendToken(tokens []string, current *strings.Builder) []string {
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
		current.Reset()
	}

	return tokens
}

// withoutComment drops an inline comment: the owner tokens from the first
// starting with #. The first token is the pattern, never a comment, because a
// line starting with # was already skipped whole.
func withoutComment(tokens []string) []string {
	for index := 1; index < len(tokens); index++ {
		if strings.HasPrefix(tokens[index], "#") {
			return tokens[:index]
		}
	}

	return tokens
}

// ownersFrom keeps the tokens that name a user or a team, without their @.
func ownersFrom(tokens []string) Owners {
	var owners Owners

	for _, token := range tokens {
		handle, found := strings.CutPrefix(token, "@")
		if !found || !validHandle(handle) {
			continue
		}

		if strings.Contains(handle, "/") {
			owners.Teams = append(owners.Teams, handle)
		} else {
			owners.Users = append(owners.Users, handle)
		}
	}

	return owners
}

// validHandle reports a handle shaped like a forge username or team path:
// segments of letters, digits, '.', '_' and '-', the first starting with a
// letter or digit. Anything else is not something a forge can be asked for.
func validHandle(handle string) bool {
	if handle == "" || !alphanumeric(rune(handle[0])) {
		return false
	}

	for segment := range strings.SplitSeq(handle, "/") {
		if segment == "" || strings.ContainsFunc(segment, notHandleCharacter) {
			return false
		}
	}

	return true
}

func notHandleCharacter(character rune) bool {
	return !alphanumeric(character) && character != '.' && character != '_' && character != '-'
}

func alphanumeric(character rune) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}
