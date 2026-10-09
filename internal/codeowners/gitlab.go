// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package codeowners

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// rubySpace is Ruby's \s, spelled out because Go's leaves out \v, and
// rubySpaces the characters it holds.
const (
	rubySpace  = `\t\n\v\f\r `
	rubySpaces = "\t\n\v\f\r "
)

// gitLabSectionHeader matches what GitLab's SectionParser::HEADER_REGEX does:
// an optional ^, a name up to the first ], approvals only straight after it and
// only digits and whitespace, then default owners as far as a run of @, word
// characters, '.', '-', '/' and whitespace reaches. Nothing need follow: any
// line it matches is a header, whatever is after.
var gitLabSectionHeader = regexp.MustCompile(
	`^(\^)?\[(.*?)\](?:\[[` + rubySpace + `\d]*\])?([` + rubySpace + `]*[@\w.\-/` + rubySpace + `]*)`)

// rubyStrip is what Ruby's String#strip takes off both ends of a line.
const rubyStrip = "\x00" + rubySpaces

// readGitLab reads a line as GitLab's CodeOwners::File does: stripped, then
// skipped when blank or a comment, a section header, skipped when it starts
// like a header GitLab cannot parse, or else an entry.
func (p *parser) readGitLab(line string) {
	line = strings.Trim(line, rubyStrip)
	if strings.TrimFunc(line, unicode.IsSpace) == "" || strings.HasPrefix(line, "#") {
		return
	}

	if match := gitLabSectionHeader.FindStringSubmatch(line); match != nil {
		p.enterSection(match[2], strings.Trim(match[3], rubyStrip))

		return
	}

	if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "^[") {
		return
	}

	pattern, owners := splitEntry(line)
	p.addRule(pattern, gitLabOwners(owners), owners != "")
}

// splitEntry is GitLab's extract_entry_info: the pattern runs to the first
// whitespace no backslash escapes, and the owners are the rest. Nothing is
// unescaped and nothing is a comment.
func splitEntry(line string) (string, string) {
	for index := 1; index < len(line); index++ {
		if strings.IndexByte(rubySpaces, line[index]) >= 0 && line[index-1] != '\\' {
			return line[:index], strings.TrimLeft(line[index:], rubySpaces)
		}
	}

	return line, ""
}

// defaultSection is GitLab's Section::DEFAULT, the name of the section a file
// starts in. GitHub's one section takes it too, and nothing reads it there.
const defaultSection = "codeowners"

// enterSection makes name the current section, with the default owners its
// header names. A section named again is the same section.
func (p *parser) enterSection(name, defaults string) {
	index := p.sectionNamed(name)
	if index < 0 {
		p.file.sections = append(p.file.sections, section{name: name, rules: nil})
		index = len(p.file.sections) - 1
	}

	p.current, p.defaults = index, gitLabOwners(defaults)
}

// sectionNamed is the section a header names, or -1 for a new one, as
// GitLab's find_section_name finds it: while no section but the default
// exists, only the exact name codeowners is it; after, the first section of
// the name without case.
func (p *parser) sectionNamed(name string) int {
	if len(p.file.sections) == 1 {
		return slices.IndexFunc(p.file.sections, func(known section) bool { return known.name == name })
	}

	return slices.IndexFunc(p.file.sections, func(known section) bool { return strings.EqualFold(known.name, name) })
}

// gitLabOwners is the owners GitLab reads from the text after a pattern or a
// header: each @name its ReferenceExtractor finds, a team when it holds a
// slash. Emails and @@roles are not names and are dropped.
func gitLabOwners(text string) Owners {
	var owners Owners

	for _, name := range gitLabNames(text) {
		if strings.Contains(name, "/") {
			owners.Teams = append(owners.Teams, name)
		} else {
			owners.Users = append(owners.Users, name)
		}
	}

	return owners
}

// gitLabNames scans text as ReferenceExtractor::NAME_REGEXP does,
// (?<![\w@])@(FULL_NAMESPACE_FORMAT_REGEX), which Go's regexp cannot spell
// for its lookbehinds: an @ not after a word character or another @, then a
// namespace path. The scan goes on after each name.
func gitLabNames(text string) []string {
	var names []string

	for index := 0; index < len(text); index++ {
		if text[index] != '@' || (index > 0 && (wordCharacter(text[index-1]) || text[index-1] == '@')) {
			continue
		}

		if end := fullNamespace(text, index+1, 0); end >= 0 {
			names = append(names, text[index+1:end])
			index = end - 1
		}
	}

	return names
}

// Limits from GitLab's Namespace: a path segment is at most URL_MAX_LENGTH
// characters, and a full path has at most NUMBER_OF_ANCESTORS_ALLOWED
// segments before its last.
const (
	namespaceMaxLength = 255
	namespaceAncestors = 20
)

// fullNamespace is where PathRegex::FULL_NAMESPACE_FORMAT_REGEX,
// (NAMESPACE/){,20}NAMESPACE, ends a match starting at start, or -1. It
// tries each segment as Ruby's backtracking does: another segment and a slash
// first, in the order the segment's ends are tried, then the segment last.
func fullNamespace(text string, start, ancestors int) int {
	ends := namespaceEnds(text, start)

	for _, end := range ends {
		if ancestors == namespaceAncestors || end == len(text) || text[end] != '/' {
			continue
		}

		if whole := fullNamespace(text, end+1, ancestors+1); whole >= 0 {
			return whole
		}
	}

	if len(ends) == 0 {
		return -1
	}

	return ends[0]
}

// namespaceEnds is where PathRegex::NAMESPACE_FORMAT_REGEX can end a segment
// starting at start, in the order Ruby tries them:
// [a-zA-Z0-9_.][a-zA-Z0-9_.-]{0,254}[a-zA-Z0-9_-], longest first, then one
// [a-zA-Z0-9_] alone; none ending in .git or .atom.
func namespaceEnds(text string, start int) []int {
	if start >= len(text) {
		return nil
	}

	var ends []int

	if namespaceCharacter(text[start], ".") {
		middle := start + 1
		for middle < len(text) && middle-start < namespaceMaxLength && namespaceCharacter(text[middle], ".-") {
			middle++
		}

		for last := middle; last > start; last-- {
			if last < len(text) && namespaceCharacter(text[last], "-") {
				ends = appendNamespaceEnd(ends, text, last+1)
			}
		}
	}

	if namespaceCharacter(text[start], "") {
		ends = appendNamespaceEnd(ends, text, start+1)
	}

	return ends
}

// appendNamespaceEnd adds end unless the segment before it ends in .git or
// .atom, which NO_SUFFIX_REGEX refuses.
func appendNamespaceEnd(ends []int, text string, end int) []int {
	if strings.HasSuffix(text[:end], ".git") || strings.HasSuffix(text[:end], ".atom") {
		return ends
	}

	return append(ends, end)
}

// namespaceCharacter reports a letter, a digit, an underscore or one of
// extra.
func namespaceCharacter(character byte, extra string) bool {
	return wordCharacter(character) || strings.IndexByte(extra, character) >= 0
}

// wordCharacter is Ruby's \w, which is ASCII.
func wordCharacter(character byte) bool {
	return character < utf8.RuneSelf && (character == '_' || alphanumeric(rune(character)))
}
