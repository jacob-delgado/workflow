// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package codeowners reads a CODEOWNERS file the way GitHub or GitLab reads it
// and answers who owns a set of paths. It is pure: the caller reads the file.
//
// Only owners a forge can be asked to review as are kept: a @username, or a
// team or group as @org/team. Email owners, GitLab's @@role owners and any
// other shape are dropped, since nothing here can turn them into a reviewer.
package codeowners

import (
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
	// and !pattern exclusions; an unanchored pattern matches at any depth.
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
// the order first seen. A team is org/team (GitLab: group/subgroup).
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
		segments := splitPath(path)

		for _, part := range f.sections {
			found.add(part.ownersOf(segments))
		}
	}

	return found.owners
}

// ownersOf is the owners the section gives a path, split into segments.
func (s section) ownersOf(segments []string) Owners {
	var owners Owners

	for _, line := range s.rules {
		if !line.pattern.matches(segments) {
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
	reading := parser{dialect: dialect, file: File{sections: []section{{name: "", rules: nil}}}}

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
}

func (p *parser) read(line string) {
	line = strings.TrimLeft(line, " \t")
	if line == "" || line[0] == '#' {
		return
	}

	if p.dialect == GitLab && (line[0] == '[' || strings.HasPrefix(line, "^[")) {
		p.enterSection(line)

		return
	}

	p.addRule(fields(line))
}

// enterSection reads a GitLab section header — [Name], ^[Optional Name] or
// [Name][approvals] — with its default owners. A section named again is the
// same section, by name without case.
func (p *parser) enterSection(header string) {
	name, defaults, ok := sectionHeader(strings.TrimPrefix(header, "^"))
	if !ok {
		return
	}

	key := strings.ToLower(name)

	index := slices.IndexFunc(p.file.sections, func(known section) bool { return known.name == key })
	if index < 0 {
		p.file.sections = append(p.file.sections, section{name: key, rules: nil})
		index = len(p.file.sections) - 1
	}

	p.current, p.defaults = index, ownersFrom(fields(defaults))
}

// sectionHeader splits "[Name][2] @a @b" into its name and the text after the
// header, which holds the default owners.
func sectionHeader(header string) (string, string, bool) {
	name, rest, found := strings.Cut(strings.TrimPrefix(header, "["), "]")
	if !found || strings.TrimSpace(name) == "" {
		return "", "", false
	}

	if strings.HasPrefix(rest, "[") {
		_, rest, found = strings.Cut(rest, "]")
	}

	return name, rest, found
}

// addRule adds a pattern line to the current section. A GitLab line naming no
// owners takes the section's defaults; GitHub has no exclusions, so a !pattern
// line there is skipped.
func (p *parser) addRule(tokens []string) {
	if len(tokens) == 0 {
		return
	}

	raw, owners := tokens[0], tokens[1:]
	exclude := strings.HasPrefix(raw, "!")

	if exclude && p.dialect == GitHub {
		return
	}

	compiled, ok := compile(strings.TrimPrefix(raw, "!"), p.dialect)
	if !ok {
		return
	}

	line := rule{pattern: compiled, exclude: exclude, owners: ownersFrom(owners)}
	if len(owners) == 0 {
		line.owners = p.defaults
	}

	at := &p.file.sections[p.current]
	at.rules = append(at.rules, line)
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
